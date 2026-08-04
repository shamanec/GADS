/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

import (
	"GADS/common/api"
	"GADS/common/models"
	"GADS/hub/devices"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/gin-gonic/gin"
)

// Timeout for proxied session commands - generous because calls like screen
// recordings and source dumps can be slow
const gridCommandTimeout = 90 * time.Second

// Shared HTTP clients for proxying grid traffic to providers, reusing the same
// connection pool as the device proxy (proxyTransport). Session creation gets a
// generous timeout because driver/WDA startup can take minutes
var (
	gridSessionClient = &http.Client{Transport: proxyTransport, Timeout: 240 * time.Second}
	gridCommandClient = &http.Client{Transport: proxyTransport, Timeout: gridCommandTimeout}
)

type AppiumSessionValue struct {
	SessionID string `json:"sessionId"`
}

type AppiumSessionResponse struct {
	Value AppiumSessionValue `json:"value"`
}

type SeleniumSessionErrorResponse struct {
	Value SeleniumSessionErrorResponseValue `json:"value"`
}

type SeleniumSessionErrorResponseValue struct {
	Error      string `json:"error"`
	Message    string `json:"message"`
	StackTrace string `json:"stacktrace"`
}

// registerGridRoutes mounts the Appium grid endpoints on the `/grid` group. The URL
// surface is identical to the old catch-all middleware. Shared between the hub
// router and handler-level tests
func registerGridRoutes(grid *gin.RouterGroup) {
	grid.GET("/status", GridStatus)
	grid.POST("/session", GridCreateSession)
	grid.DELETE("/session/:sessionId", GridDeleteSession)
	// A bare `/session/{id}` with any other method (e.g. `GET /session/{id}`) is an
	// ordinary WebDriver command - only the exact DELETE above ends the session
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodHead, http.MethodOptions} {
		grid.Handle(method, "/session/:sessionId", GridSessionCommand)
	}
	grid.Any("/session/:sessionId/*path", GridSessionCommand)
}

// GridStatus is the W3C status endpoint (`GET /grid/status`) - reports whether the
// grid can serve sessions plus a per-device readiness list. Like the rest of /grid
// it is unauthenticated, so it deliberately exposes no workspace, tenant or user
// information
func GridStatus(c *gin.Context) {
	type gridStatusDevice struct {
		UDID              string `json:"udid"`
		Name              string `json:"name"`
		OS                string `json:"os"`
		OSVersion         string `json:"os_version"`
		Provider          string `json:"provider"`
		Available         bool   `json:"available"`
		InUseByAutomation bool   `json:"in_use_by_automation"`
	}

	ready := false
	statusDevices := []gridStatusDevice{}
	for _, hubDevice := range devices.HubDeviceStore.AllSorted() {
		hubDevice.Mu.RLock()
		connectedAndLive := hubDevice.Connected && hubDevice.ProviderState == "live"
		usageAllowsAutomation := hubDevice.Device.Usage != "control" && hubDevice.Device.Usage != "disabled"
		statusDevices = append(statusDevices, gridStatusDevice{
			UDID:              hubDevice.Device.UDID,
			Name:              hubDevice.Device.Name,
			OS:                hubDevice.Device.OS,
			OSVersion:         hubDevice.Device.OSVersion,
			Provider:          hubDevice.Device.Provider,
			Available:         connectedAndLive && usageAllowsAutomation && hubDevice.IsAvailableForAutomation,
			InUseByAutomation: hubDevice.IsRunningAutomation,
		})
		if connectedAndLive {
			ready = true
		}
		hubDevice.Mu.RUnlock()
	}

	message := "GADS grid ready"
	if !ready {
		message = "no devices available"
	}
	c.JSON(http.StatusOK, gin.H{"value": gin.H{"ready": ready, "message": message, "devices": statusDevices}})
}

type automationSession struct {
	SessionID     string `json:"session_id"`
	DeviceUDID    string `json:"device_udid"`
	DeviceName    string `json:"device_name"`
	InUseBy       string `json:"in_use_by"`
	InUseByTenant string `json:"in_use_by_tenant"`
	StartedTS     int64  `json:"started_ts"`
	LastCommandTS int64  `json:"last_command_ts"`
}

// GetAutomationSessions godoc
// @Summary      List active automation sessions
// @Description  Retrieve the currently active Appium grid sessions and the devices serving them
// @Tags         Hub - Devices
// @Produce      json
// @Success      200  {object}  models.APIResponse
// @Failure      401  {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /automation-sessions [get]
func GetAutomationSessions(c *gin.Context) {
	sessions := []automationSession{}
	for sessionID, sessionDevice := range devices.AllSessions() {
		sessionDevice.Mu.RLock()
		sessions = append(sessions, automationSession{
			SessionID:     sessionID,
			DeviceUDID:    sessionDevice.Device.UDID,
			DeviceName:    sessionDevice.Device.Name,
			InUseBy:       sessionDevice.InUseBy,
			InUseByTenant: sessionDevice.InUseByTenant,
			StartedTS:     sessionDevice.AutomationSessionStartTS,
			LastCommandTS: sessionDevice.LastAutomationActionTS,
		})
		sessionDevice.Mu.RUnlock()
	}
	// Map iteration order is random - keep the output stable for consumers
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].StartedTS != sessions[j].StartedTS {
			return sessions[i].StartedTS < sessions[j].StartedTS
		}
		return sessions[i].SessionID < sessions[j].SessionID
	})
	api.OK(c, "Active automation sessions", sessions)
}

// Every second sweep the devices and clean up automation sessions
// that expired or whose device is gone
func UpdateExpiredGridSessions() {
	for {
		sweepExpiredGridSessions()
		time.Sleep(1 * time.Second)
	}
}

// sweepExpiredGridSessions does one janitor pass over all hub devices:
// releases expired API leases and resets devices that disconnected, went
// non-live (re-provisioned for example) or idled past their Appium new-command timeout
func sweepExpiredGridSessions() {
	now := time.Now().UnixMilli()
	for _, hubDevice := range devices.HubDeviceStore.All() {
		hubDevice.Mu.Lock()
		// Release expired API leases
		if hubDevice.LockSource == devices.LockSourceAPI && hubDevice.LeaseExpiresAt > 0 && hubDevice.LeaseExpiresAt < now {
			hubDevice.ReleaseLock()
		}
		// A new-command timeout of 0 means the client explicitly disabled the idle
		// timer (`appium:newCommandTimeout: 0`) - such sessions live until they are
		// deleted or the device disconnects
		idleExpired := hubDevice.IsRunningAutomation &&
			hubDevice.AppiumNewCommandTimeout > 0 &&
			hubDevice.LastAutomationActionTS <= (now-hubDevice.AppiumNewCommandTimeout)
		if !hubDevice.Connected || hubDevice.ProviderState != "live" || idleExpired {
			// Ask the provider to actually close the expired Appium session (best
			// effort, in the background) - otherwise the app under test keeps running
			// on the device until the next session overrides it
			if hubDevice.SessionID != "" && hubDevice.Connected && hubDevice.ProviderState == "live" {
				go killProviderAppiumSession(hubDevice.Host, hubDevice.Device.UDID, hubDevice.SessionID)
			}
			hubDevice.ReleaseFromAutomation()
		}
		hubDevice.Mu.Unlock()
	}
}

// killProviderAppiumSession sends a session DELETE to the device's provider,
// used when the hub expires a session on its side. Best effort - failures are
// only logged, the janitor keeps running either way
func killProviderAppiumSession(deviceHost string, deviceUDID string, sessionID string) {
	deleteURL := fmt.Sprintf("http://%s/device/%s/appium/session/%s", deviceHost, deviceUDID, sessionID)
	deleteReq, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to create cleanup request for expired Appium session `%s` on device `%s` - %s", sessionID, deviceUDID, err))
		return
	}
	resp, err := gridCommandClient.Do(deleteReq)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to delete expired Appium session `%s` on device `%s` - %s", sessionID, deviceUDID, err))
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}

// gridUser is the automation client identity resolved from its `gads:clientSecret`,
// with the workspaces its sessions are allowed to claim devices from
type gridUser struct {
	UserID              string
	Tenant              string
	AllowedWorkspaceIDs []string
}

// resolveGridUser validates the client secret extracted from the session capabilities
// and resolves the workspaces the client may use devices from
func resolveGridUser(clientSecret string) (gridUser, *W3CError) {
	if clientSecret == "" {
		return gridUser{}, &W3CError{
			HTTPStatus: http.StatusUnauthorized,
			Code:       "session not created",
			Message:    fmt.Sprintf("Client credentials are required. Provide %s:clientSecret in the capabilities.", capabilityPrefix),
		}
	}

	credential, err := gridDB.GetClientCredentialBySecret(clientSecret)
	if err != nil || !credential.IsActive {
		return gridUser{}, &W3CError{HTTPStatus: http.StatusUnauthorized, Code: "session not created", Message: "Invalid client credentials"}
	}

	user := gridUser{UserID: credential.UserID, Tenant: credential.Tenant}

	if credential.Tenant != "" {
		defaultTenant, _ := gridDB.GetOrCreateDefaultTenant()
		useAllTenantWorkspaces := true

		// Check if we need to filter by user workspaces
		if credential.Tenant == defaultTenant && credential.UserID != "" {
			dbUser, err := gridDB.GetUser(credential.UserID)
			if err != nil {
				return gridUser{}, &W3CError{HTTPStatus: http.StatusUnauthorized, Code: "session not created", Message: "User not found"}
			}

			if dbUser.Role != "admin" {
				// Regular user: only assigned workspaces
				useAllTenantWorkspaces = false
				userWorkspaces := gridDB.GetUserWorkspaces(credential.UserID)
				for _, ws := range userWorkspaces {
					user.AllowedWorkspaceIDs = append(user.AllowedWorkspaceIDs, ws.ID)
				}
			}
		}

		// Admin users or non-default tenant: all workspaces of the tenant
		if useAllTenantWorkspaces {
			allWorkspaces, _ := gridDB.GetWorkspaces()
			for _, ws := range allWorkspaces {
				if ws.Tenant == credential.Tenant {
					user.AllowedWorkspaceIDs = append(user.AllowedWorkspaceIDs, ws.ID)
				}
			}
		}
	}

	return user, nil
}

// abortAutomationClaim undoes a pre-session device claim after a failed session
// create. The lock fields are left untouched on purpose - no session was
// established, so there is nothing to release beyond the availability flags
func abortAutomationClaim(foundDevice *devices.LocalHubDevice) {
	foundDevice.Mu.Lock()
	foundDevice.IsAvailableForAutomation = true
	foundDevice.IsRunningAutomation = false
	foundDevice.Mu.Unlock()
}

// GridCreateSession handles `POST /grid/session` - authenticates the client via its
// `gads:clientSecret` capability, finds and claims a matching device, forwards the
// session request to the device's provider Appium endpoint and records the created
// session in the session registry
func GridCreateSession(c *gin.Context) {
	sessionRequestBody, err := readBody(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to read session request sessionRequestBody", "session not created", err.Error()))
		return
	}
	defer c.Request.Body.Close()

	// Parse the request body per the W3C capabilities processing rules
	parsedReq, w3cErr := parseSessionRequest(sessionRequestBody, capabilityPrefix)
	if w3cErr != nil {
		writeW3CError(c, w3cErr)
		return
	}

	candidate, matched := selectGridCandidate(parsedReq.Candidates)
	if !matched {
		writeW3CError(c, w3cSessionNotCreated("GADS could not determine a target device platform from any capabilities candidate in the session request"))
		return
	}

	// Extract client secret from capabilities and resolve the allowed workspaces
	clientSecret := models.ExtractClientSecretFromSession(parsedReq.Raw, capabilityPrefix)
	user, w3cErr := resolveGridUser(clientSecret)
	if w3cErr != nil {
		writeW3CError(c, w3cErr)
		return
	}

	// Check for available device
	foundDevice, deviceErr := findAvailableDevice(candidate, user.AllowedWorkspaceIDs, user.UserID, user.Tenant)

	if deviceErr != nil && strings.Contains(deviceErr.Error(), "No device with udid") {
		c.JSON(http.StatusNotFound, createErrorResponse("No available device found", "session not created", ""))
		return
	}

	// Usage misconfiguration cannot resolve by waiting - fail immediately
	// with the reason instead of entering the retry loop
	if deviceErr != nil && strings.Contains(deviceErr.Error(), "is not enabled for automation") {
		writeW3CError(c, w3cSessionNotCreated(deviceErr.Error()))
		return
	}

	// If no device is available start checking each second for 10 seconds
	// If no device is available after 10 seconds - return error
	if foundDevice == nil {
		ticker := time.NewTicker(100 * time.Millisecond)
		timeout := time.After(10 * time.Second)
		notify := c.Writer.CloseNotify()
	FOR_LOOP:
		for {
			select {
			case <-ticker.C:
				foundDevice, deviceErr = findAvailableDevice(candidate, user.AllowedWorkspaceIDs, user.UserID, user.Tenant)
				if foundDevice != nil {
					break FOR_LOOP
				}
			case <-timeout:
				ticker.Stop()
				if deviceErr != nil {
					c.JSON(http.StatusInternalServerError, createErrorResponse(deviceErr.Error(), "session not created", ""))
				} else {
					c.JSON(http.StatusInternalServerError, createErrorResponse("No available device found", "session not created", ""))
				}
				return
			case <-notify:
				ticker.Stop()
				return
			}
		}
	}

	if foundDevice == nil {
		if deviceErr != nil {
			c.JSON(http.StatusInternalServerError, createErrorResponse(deviceErr.Error(), "session not created", ""))
		} else {
			c.JSON(http.StatusInternalServerError, createErrorResponse("No available device found", "session not created", ""))
		}
		return
	}

	// `appium:newCommandTimeout` semantics: absent -> 60s default; explicit 0 ->
	// idle timeout disabled on the hub too (0 makes the janitor skip the idle
	// check, matching Appium disabling its own timer); anything else -> as given
	newCommandTimeoutMS := int64(60000)
	if candidate.NewCommandTimeout != nil {
		newCommandTimeoutMS = *candidate.NewCommandTimeout * 1000
	}
	foundDevice.Mu.Lock()
	foundDevice.ClaimForAutomation(newCommandTimeoutMS)
	foundDevice.Mu.Unlock()

	// Remove grid-internal `gads:*` capabilities so the client secret never reaches Appium (and its logs)
	stripGadsCaps(parsedReq.Raw, capabilityPrefix)
	updatedSessionBody, _ := json.Marshal(parsedReq.Raw)

	foundDevice.Mu.RLock()
	deviceHost := foundDevice.Host
	deviceUDID := foundDevice.Device.UDID
	foundDevice.Mu.RUnlock()

	// Create a new request to the device target URL
	proxyReq, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/device/%s/appium/session", deviceHost, deviceUDID), bytes.NewBuffer(updatedSessionBody))
	if err != nil {
		abortAutomationClaim(foundDevice)
		c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to create http request to proxy the call to the device respective provider Appium session endpoint", "session not created", err.Error()))
		return
	}

	// Copy headers from the original request to the new request
	for k, v := range c.Request.Header {
		proxyReq.Header[k] = v
	}

	// Send the request
	resp, err := gridSessionClient.Do(proxyReq)
	if err != nil {
		abortAutomationClaim(foundDevice)
		c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to execute the proxy request to the device respective provider Appium session endpoint", "session not created", err.Error()))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Release the claim for any error status. On a 500 the lock fields are kept
		// for a grace period (the delayed goroutine below) so a quick retry by the
		// same client is not raced; any other error releases the lock right away
		if resp.StatusCode == http.StatusInternalServerError {
			abortAutomationClaim(foundDevice)
			go func() {
				time.Sleep(10 * time.Second)
				foundDevice.Mu.Lock()
				if foundDevice.LastAutomationActionTS <= (time.Now().UnixMilli() - 5000) {
					foundDevice.ReleaseFromAutomation()
				}
				foundDevice.Mu.Unlock()
			}()
		} else {
			foundDevice.Mu.Lock()
			foundDevice.ReleaseFromAutomation()
			foundDevice.Mu.Unlock()
		}

		// Read and pass the error response
		proxiedResponseBody, _ := readBody(resp.Body)
		for k, v := range resp.Header {
			c.Writer.Header()[k] = v
		}
		c.Writer.WriteHeader(resp.StatusCode)
		c.Writer.Write(proxiedResponseBody)
		return
	}

	// Read the response sessionRequestBody from the proxied request
	proxiedSessionResponseBody, err := readBody(resp.Body)
	if err != nil {
		abortAutomationClaim(foundDevice)
		c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to read the response sessionRequestBody of the proxied Appium session request", "session not created", err.Error()))
		return
	}

	// Unmarshal the response sessionRequestBody to AppiumSessionResponse
	var proxySessionResponse AppiumSessionResponse
	err = json.Unmarshal(proxiedSessionResponseBody, &proxySessionResponse)
	if err != nil {
		abortAutomationClaim(foundDevice)
		c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to unmarshal the response sessionRequestBody of the proxied Appium session request", "session not created", err.Error()))
		return
	}

	foundDevice.Mu.Lock()
	// A leftover session ID from a just-ended session must leave the registry
	// before the new one is indexed
	if foundDevice.SessionID != "" && foundDevice.SessionID != proxySessionResponse.Value.SessionID {
		devices.UnregisterSession(foundDevice.SessionID)
	}
	foundDevice.SessionID = proxySessionResponse.Value.SessionID
	foundDevice.AutomationSessionStartTS = time.Now().UnixMilli()
	devices.RegisterSession(foundDevice.SessionID, foundDevice)
	foundDevice.Mu.Unlock()

	// Enrich the returned capabilities with grid-level `gads:*` entries (spec-legal
	// for intermediary nodes) so the client knows which device it actually got
	responseBody := enrichSessionResponse(proxiedSessionResponseBody, foundDevice, hubBaseURL(c))

	// Copy the response back to the original client
	for k, v := range resp.Header {
		c.Writer.Header()[k] = v
	}
	// The body may have grown past the provider's Content-Length
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))
	c.Writer.WriteHeader(resp.StatusCode)
	c.Writer.Write(responseBody)

	foundDevice.Mu.Lock()
	foundDevice.LastAutomationActionTS = time.Now().UnixMilli()
	// Set InUseBy with user ID and tenant for tracking
	automationUser := user.UserID
	if automationUser == "" {
		automationUser = "unknown"
	}
	// Only update InUseBy if no UI or API session is active
	if !foundDevice.HasUISession() && !foundDevice.HasActiveLease() {
		foundDevice.InUseBy = automationUser
		foundDevice.InUseByTenant = user.Tenant
		foundDevice.InUseTS = time.Now().UnixMilli()
	}
	foundDevice.Mu.Unlock()
}

// hubBaseURL derives the hub's base URL from the incoming request - the address the
// client used to reach the hub is by definition reachable for whoever ran the test.
// The hub itself only serves plain HTTP; a TLS-terminating reverse proxy announces
// itself via X-Forwarded-Proto
func hubBaseURL(c *gin.Context) string {
	if c.Request.Host == "" {
		return ""
	}
	scheme := "http"
	if forwardedProto := c.Request.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
		scheme = forwardedProto
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

// enrichSessionResponse injects grid-level `<prefix>:*` capabilities into a successful
// session-create response: the served device's UDID, name and provider, plus a link to
// the hub's device control UI for the device. The response is parsed as a generic map
// so every capability Appium returned survives untouched; on any unexpected shape the
// original body is returned as-is
func enrichSessionResponse(responseBody []byte, foundDevice *devices.LocalHubDevice, hubURL string) []byte {
	var response map[string]interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return responseBody
	}
	value, ok := response["value"].(map[string]interface{})
	if !ok {
		return responseBody
	}
	caps, ok := value["capabilities"].(map[string]interface{})
	if !ok {
		return responseBody
	}

	foundDevice.Mu.RLock()
	deviceUDID := foundDevice.Device.UDID
	deviceName := foundDevice.Device.Name
	deviceProvider := foundDevice.Device.Provider
	foundDevice.Mu.RUnlock()

	caps[capabilityPrefix+":deviceUdid"] = deviceUDID
	caps[capabilityPrefix+":deviceName"] = deviceName
	caps[capabilityPrefix+":provider"] = deviceProvider
	if hubURL != "" {
		caps[capabilityPrefix+":controlUrl"] = fmt.Sprintf("%s/devices/control/%s", hubURL, deviceUDID)
	}

	enriched, err := json.Marshal(response)
	if err != nil {
		return responseBody
	}
	return enriched
}

// GridSessionCommand proxies an ordinary WebDriver command (anything under
// `/grid/session/{id}` that is not the exact session DELETE) to the device's
// provider Appium endpoint via a streaming reverse proxy
func GridSessionCommand(c *gin.Context) {
	sessionID := c.Param("sessionId")
	// Empty on the bare `/session/{id}` route (e.g. `GET /session/{id}`)
	commandPath := c.Param("path")

	foundDevice, ok := devices.DeviceBySession(sessionID)
	if !ok {
		writeW3CError(c, w3cInvalidSessionID(fmt.Sprintf("No session ID `%s` is available to GADS, it timed out or something unexpected occurred", sessionID)))
		return
	}

	proxyGridSessionRequest(c, foundDevice, sessionID, commandPath, func(statusCode int) {
		if statusCode == http.StatusInternalServerError {
			releaseAfterProviderError(foundDevice)
		}
	})
}

// GridDeleteSession handles the exact `DELETE /grid/session/{id}` - the only request
// that ends a session and releases its device
func GridDeleteSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	foundDevice, ok := devices.DeviceBySession(sessionID)
	if !ok {
		writeW3CError(c, w3cInvalidSessionID(fmt.Sprintf("No session ID `%s` is available to GADS, it timed out or something unexpected occurred", sessionID)))
		return
	}

	proxyGridSessionRequest(c, foundDevice, sessionID, "", func(statusCode int) {
		// The session was deleted - mark the device available right away and fully
		// release it after a second if no new session claims it in the meantime
		foundDevice.Mu.Lock()
		foundDevice.IsAvailableForAutomation = true
		foundDevice.Mu.Unlock()
		go func() {
			time.Sleep(1 * time.Second)
			foundDevice.Mu.Lock()
			if foundDevice.LastAutomationActionTS <= (time.Now().UnixMilli() - 1000) {
				foundDevice.ReleaseFromAutomation()
			}
			foundDevice.Mu.Unlock()
		}()
		if statusCode == http.StatusInternalServerError {
			releaseAfterProviderError(foundDevice)
		}
	})
}

// releaseAfterProviderError starts the delayed device release used when the provider
// answers a session request with a 500 - if no further automation activity happens
// within 10 seconds the session is considered dead and the device is freed
func releaseAfterProviderError(foundDevice *devices.LocalHubDevice) {
	go func() {
		time.Sleep(10 * time.Second)
		foundDevice.Mu.Lock()
		if foundDevice.LastAutomationActionTS <= (time.Now().UnixMilli() - 10000) {
			foundDevice.ReleaseFromAutomation()
		}
		foundDevice.Mu.Unlock()
	}()
}

// proxyGridSessionRequest forwards a session-scoped request to the Appium endpoint of
// the device's provider through a streaming reverse proxy, bounded by the same command
// timeout the old shared client enforced. onResponse runs on the provider's response
// status before the response is written back to the client
func proxyGridSessionRequest(c *gin.Context, foundDevice *devices.LocalHubDevice, sessionID string, commandPath string, onResponse func(statusCode int)) {
	foundDevice.Mu.RLock()
	deviceHost := foundDevice.Host
	deviceUDID := foundDevice.Device.UDID
	foundDevice.Mu.RUnlock()

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = deviceHost
			req.URL.Path = fmt.Sprintf("/device/%s/appium/session/%s%s", deviceUDID, sessionID, commandPath)
		},
		Transport: proxyTransport,
		ModifyResponse: func(resp *http.Response) error {
			if onResponse != nil {
				onResponse(resp.StatusCode)
			}
			resp.Header.Del("Access-Control-Allow-Origin")
			// Long-standing behavior: a provider 500 is masked with a GADS error body
			if resp.StatusCode == http.StatusInternalServerError {
				replaceResponseBody(resp, createErrorResponse("GADS got an internal server error from the proxy request to the device respective provider Appium endpoint", "", ""))
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, proxyErr error) {
			c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to execute the proxy request to the device respective provider Appium endpoint", "", proxyErr.Error()))
		},
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), gridCommandTimeout)
	defer cancel()
	proxy.ServeHTTP(c.Writer, c.Request.WithContext(ctx))

	// Record the automation activity so the janitor's idle expiry starts counting
	// from the end of this command
	foundDevice.Mu.Lock()
	foundDevice.LastAutomationActionTS = time.Now().UnixMilli()
	foundDevice.Mu.Unlock()
}

// replaceResponseBody swaps a proxied response's body for a GADS error body,
// draining the original so the upstream connection can be reused
func replaceResponseBody(resp *http.Response, errorResponse SeleniumSessionErrorResponse) {
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	newBody, _ := json.Marshal(errorResponse)
	resp.Body = io.NopCloser(bytes.NewReader(newBody))
	resp.ContentLength = int64(len(newBody))
	resp.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
	resp.Header.Set("Content-Type", "application/json; charset=utf-8")
}

func readBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return []byte{}, err
	}

	return body, nil
}

func getDeviceByUDID(udid string) (*devices.LocalHubDevice, error) {
	// Try direct lookup first (O(1))
	if d, ok := devices.HubDeviceStore.Get(udid); ok {
		return d, nil
	}
	// Fall back to case-insensitive search
	for _, localDevice := range devices.HubDeviceStore.All() {
		if strings.EqualFold(localDevice.Device.UDID, udid) {
			return localDevice, nil
		}
	}
	return nil, fmt.Errorf("No device with udid `%s` was found", udid)
}

func findAvailableDevice(candidate gridCandidate, allowedWorkspaceIDs []string, userID string, userTenant string) (*devices.LocalHubDevice, error) {
	var foundDevice *devices.LocalHubDevice

	deviceUDID := candidate.DeviceUDID

	if len(allowedWorkspaceIDs) == 0 {
		return nil, fmt.Errorf("No device with udid `%s` was found in allowed workspaces", deviceUDID)
	}

	if deviceUDID != "" {
		d, err := getDeviceByUDID(deviceUDID)
		if err != nil {
			return nil, err
		}

		d.Mu.RLock()
		wsID := d.Device.WorkspaceID
		connected := d.Connected
		state := d.ProviderState
		lastUpdated := d.LastUpdatedTimestamp
		usage := d.Device.Usage
		isLockedByOther := d.IsLockedByOther(userID, userTenant)
		d.Mu.RUnlock()

		// Check if device is in allowed workspaces
		deviceAllowed := false
		for _, allowedWsID := range allowedWorkspaceIDs {
			if wsID == allowedWsID {
				deviceAllowed = true
				break
			}
		}
		if !deviceAllowed {
			return nil, fmt.Errorf("No device with udid `%s` was found", deviceUDID)
		}

		// Usage is static configuration - a device set to remote-control-only or
		// disabled can never serve automation, so tell the client the reason and
		// let the middleware fail fast instead of retrying
		if usage == "control" || usage == "disabled" {
			return nil, fmt.Errorf("Device with udid `%s` is not enabled for automation, its usage is set to `%s`", deviceUDID, usage)
		}

		// A UDID-pinned device must pass the same runtime eligibility checks as the
		// generic search below - without them a client could start automation on a
		// device another user is actively controlling. The exact reason is
		// deliberately not exposed to the client
		if !connected ||
			state != "live" ||
			lastUpdated < (time.Now().UnixMilli()-3000) ||
			isLockedByOther {
			return nil, fmt.Errorf("Device is currently not available for automation")
		}

		d.Mu.Lock()
		if d.IsAvailableForAutomation {
			d.IsAvailableForAutomation = false
			d.Mu.Unlock()
			return d, nil
		}
		d.Mu.Unlock()
		return nil, fmt.Errorf("Device is currently not available for automation")
	}

	var availableDevices []*devices.LocalHubDevice

	targetOS := candidateTargetOS(candidate)
	if targetOS != "" {
		for _, localDevice := range devices.HubDeviceStore.All() {
			localDevice.Mu.RLock()
			os := localDevice.Device.OS
			connected := localDevice.Connected
			state := localDevice.ProviderState
			lastUpdated := localDevice.LastUpdatedTimestamp
			available := localDevice.IsAvailableForAutomation
			usage := localDevice.Device.Usage
			wsID := localDevice.Device.WorkspaceID
			isLockedByOther := localDevice.IsLockedByOther(userID, userTenant)
			localDevice.Mu.RUnlock()

			if !strings.EqualFold(os, targetOS) ||
				!connected ||
				state != "live" ||
				lastUpdated < (time.Now().UnixMilli()-3000) ||
				!available ||
				usage == "control" ||
				usage == "disabled" {
				continue
			}

			deviceAllowed := false
			for _, wsID2 := range allowedWorkspaceIDs {
				if wsID == wsID2 {
					deviceAllowed = true
					break
				}
			}
			if !deviceAllowed {
				continue
			}

			if isLockedByOther {
				continue
			}

			availableDevices = append(availableDevices, localDevice)
		}
	}

	if candidate.PlatformVersion != "" {
		// First try exact version match
		for _, device := range availableDevices {
			device.Mu.RLock()
			osVersion := device.Device.OSVersion
			device.Mu.RUnlock()

			if osVersion == candidate.PlatformVersion {
				device.Mu.Lock()
				if device.IsAvailableForAutomation {
					device.IsAvailableForAutomation = false
					device.Mu.Unlock()
					foundDevice = device
					break
				}
				device.Mu.Unlock()
			}
		}

		// Fall back to major version match
		if foundDevice == nil {
			v, _ := semver.NewVersion(candidate.PlatformVersion)
			requestedMajorVersion := fmt.Sprintf("%d", v.Major())
			constraint, _ := semver.NewConstraint(fmt.Sprintf("^%s.0.0", requestedMajorVersion))

			for _, device := range availableDevices {
				device.Mu.RLock()
				osVersion := device.Device.OSVersion
				device.Mu.RUnlock()

				deviceV, _ := semver.NewVersion(osVersion)
				if constraint.Check(deviceV) {
					device.Mu.Lock()
					if device.IsAvailableForAutomation {
						device.IsAvailableForAutomation = false
						device.Mu.Unlock()
						foundDevice = device
						break
					}
					device.Mu.Unlock()
				}
			}
		}
	} else {
		// No platform version requested — take the first available
		for _, device := range availableDevices {
			device.Mu.Lock()
			if device.IsAvailableForAutomation {
				device.IsAvailableForAutomation = false
				device.Mu.Unlock()
				foundDevice = device
				break
			}
			device.Mu.Unlock()
		}
	}

	if foundDevice != nil {
		return foundDevice, nil
	}

	return nil, fmt.Errorf("No available device found")
}

func createErrorResponse(msg string, err string, stacktrace string) SeleniumSessionErrorResponse {
	return SeleniumSessionErrorResponse{
		Value: SeleniumSessionErrorResponseValue{
			Message:    msg,
			Error:      err,
			StackTrace: stacktrace,
		},
	}
}
