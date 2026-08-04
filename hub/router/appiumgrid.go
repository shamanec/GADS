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
	"GADS/common/models"
	"GADS/hub/devices"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/gin-gonic/gin"
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

// Every 3 seconds check the devices
// And clean the automation session if no action was taken in the timeout limit
func UpdateExpiredGridSessions() {
	for {
		now := time.Now().UnixMilli()
		for _, hubDevice := range devices.HubDeviceStore.All() {
			hubDevice.Mu.Lock()
			// Release expired API leases
			if hubDevice.LockSource == devices.LockSourceAPI && hubDevice.LeaseExpiresAt > 0 && hubDevice.LeaseExpiresAt < now {
				hubDevice.ReleaseLock()
			}
			// Reset device if its not connected
			// Or it hasn't received any Appium requests in the command timeout and is running automation
			// Or if its provider state is not "live" - device was re-provisioned for example
			if !hubDevice.Connected ||
				(hubDevice.LastAutomationActionTS <= (time.Now().UnixMilli()-hubDevice.AppiumNewCommandTimeout) && hubDevice.IsRunningAutomation) ||
				hubDevice.ProviderState != "live" {
				hubDevice.IsRunningAutomation = false
				hubDevice.IsAvailableForAutomation = true
				hubDevice.SessionID = ""
				hubDevice.ReleaseLockIfNotHeld()
			}
			hubDevice.Mu.Unlock()
		}
		time.Sleep(1 * time.Second)
	}
}

func AppiumGridMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasSuffix(c.Request.URL.Path, "/session") {
			// Read the request sessionRequestBody
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

			// Extract client secret from capabilities and get allowed workspaces
			var allowedWorkspaceIDs []string
			clientSecret := models.ExtractClientSecretFromSession(parsedReq.Raw, capabilityPrefix)

			if clientSecret == "" {
				c.JSON(http.StatusUnauthorized, createErrorResponse(
					fmt.Sprintf("Client credentials are required. Provide %s:clientSecret in the capabilities.", capabilityPrefix),
					"session not created",
					""))
				return
			}

			credential, err := gridDB.GetClientCredentialBySecret(clientSecret)
			if err != nil || !credential.IsActive {
				c.JSON(http.StatusUnauthorized, createErrorResponse("Invalid client credentials", "session not created", ""))
				return
			}

			if credential.Tenant != "" {
				defaultTenant, _ := gridDB.GetOrCreateDefaultTenant()
				useAllTenantWorkspaces := true

				// Check if we need to filter by user workspaces
				if credential.Tenant == defaultTenant && credential.UserID != "" {
					user, err := gridDB.GetUser(credential.UserID)
					if err != nil {
						c.JSON(http.StatusUnauthorized, createErrorResponse("User not found", "session not created", ""))
						return
					}

					if user.Role != "admin" {
						// Regular user: only assigned workspaces
						useAllTenantWorkspaces = false
						userWorkspaces := gridDB.GetUserWorkspaces(credential.UserID)
						for _, ws := range userWorkspaces {
							allowedWorkspaceIDs = append(allowedWorkspaceIDs, ws.ID)
						}
					}
				}

				// Admin users or non-default tenant: all workspaces of the tenant
				if useAllTenantWorkspaces {
					allWorkspaces, _ := gridDB.GetWorkspaces()
					for _, ws := range allWorkspaces {
						if ws.Tenant == credential.Tenant {
							allowedWorkspaceIDs = append(allowedWorkspaceIDs, ws.ID)
						}
					}
				}
			}

			// Check for available device
			var foundDevice *devices.LocalHubDevice
			var deviceErr error

			foundDevice, deviceErr = findAvailableDevice(candidate, allowedWorkspaceIDs, credential.UserID, credential.Tenant)

			if deviceErr != nil && strings.Contains(deviceErr.Error(), "No device with udid") {
				c.JSON(http.StatusNotFound, createErrorResponse("No available device found", "session not created", ""))
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
						foundDevice, deviceErr = findAvailableDevice(candidate, allowedWorkspaceIDs, credential.UserID, credential.Tenant)
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

			foundDevice.Mu.Lock()
			// Set device found as running automation and is not available for automation
			// Before even starting the Appium session creation request
			// Also set an automation action timestamp so that the goroutine does not reset it while session is being created
			foundDevice.IsRunningAutomation = true
			foundDevice.IsAvailableForAutomation = false
			foundDevice.LastAutomationActionTS = time.Now().UnixMilli()
			// Update the session timeout values if none were provided
			// NOTE: explicit `appium:newCommandTimeout: 0` currently falls back to the default like "absent" does - Phase 2 gives it disable semantics
			if candidate.NewCommandTimeout != nil && *candidate.NewCommandTimeout != 0 {
				foundDevice.AppiumNewCommandTimeout = *candidate.NewCommandTimeout * 1000
			} else {
				foundDevice.AppiumNewCommandTimeout = 60000
			}
			foundDevice.Mu.Unlock()

			// Remove grid-internal `gads:*` capabilities so the client secret never reaches Appium (and its logs)
			stripGadsCaps(parsedReq.Raw, capabilityPrefix)
			updatedSessionBody, _ := json.Marshal(parsedReq.Raw)
			// Create a new request to the device target URL
			foundDevice.Mu.RLock()
			deviceHost := foundDevice.Host
			deviceUDID := foundDevice.Device.UDID
			foundDevice.Mu.RUnlock()

			proxyReq, err := http.NewRequest(c.Request.Method, fmt.Sprintf("http://%s/device/%s/appium%s", deviceHost, deviceUDID, strings.Replace(c.Request.URL.Path, "/grid", "", -1)), bytes.NewBuffer(updatedSessionBody))
			if err != nil {
				foundDevice.Mu.Lock()
				foundDevice.IsAvailableForAutomation = true
				foundDevice.IsRunningAutomation = false
				foundDevice.Mu.Unlock()
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to create http request to proxy the call to the device respective provider Appium session endpoint", "session not created", err.Error()))
				return
			}

			// Copy headers from the original request to the new request
			for k, v := range c.Request.Header {
				proxyReq.Header[k] = v
			}

			// Send the request
			client := &http.Client{}
			resp, err := client.Do(proxyReq)
			if err != nil {
				foundDevice.Mu.Lock()
				foundDevice.IsAvailableForAutomation = true
				foundDevice.IsRunningAutomation = false
				foundDevice.Mu.Unlock()
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to execute the proxy request to the device respective provider Appium session endpoint", "session not created", err.Error()))
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				// Release device for any error status
				foundDevice.Mu.Lock()
				foundDevice.IsAvailableForAutomation = true
				foundDevice.IsRunningAutomation = false
				if resp.StatusCode != http.StatusInternalServerError {
					foundDevice.ReleaseLockIfNotHeld()
				}
				foundDevice.Mu.Unlock()

				// For 500 errors, keep the existing behavior with goroutine
				if resp.StatusCode == http.StatusInternalServerError {
					go func() {
						time.Sleep(10 * time.Second)
						foundDevice.Mu.Lock()
						if foundDevice.LastAutomationActionTS <= (time.Now().UnixMilli() - 5000) {
							foundDevice.IsAvailableForAutomation = true
							foundDevice.SessionID = ""
							foundDevice.IsRunningAutomation = false
							foundDevice.ReleaseLockIfNotHeld()
						}
						foundDevice.Mu.Unlock()
					}()
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
				foundDevice.Mu.Lock()
				foundDevice.IsAvailableForAutomation = true
				foundDevice.IsRunningAutomation = false
				foundDevice.Mu.Unlock()
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to read the response sessionRequestBody of the proxied Appium session request", "session not created", err.Error()))
				return
			}

			// Unmarshal the response sessionRequestBody to AppiumSessionResponse
			var proxySessionResponse AppiumSessionResponse
			err = json.Unmarshal(proxiedSessionResponseBody, &proxySessionResponse)
			if err != nil {
				foundDevice.Mu.Lock()
				foundDevice.IsAvailableForAutomation = true
				foundDevice.IsRunningAutomation = false
				foundDevice.Mu.Unlock()
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to unmarshal the response sessionRequestBody of the proxied Appium session request", "session not created", err.Error()))
				return
			}

			foundDevice.Mu.Lock()
			foundDevice.SessionID = proxySessionResponse.Value.SessionID
			foundDevice.Mu.Unlock()

			// Copy the response back to the original client
			for k, v := range resp.Header {
				c.Writer.Header()[k] = v
			}
			c.Writer.WriteHeader(resp.StatusCode)
			c.Writer.Write(proxiedSessionResponseBody)

			foundDevice.Mu.Lock()
			foundDevice.LastAutomationActionTS = time.Now().UnixMilli()
			// Set InUseBy with user ID and tenant for tracking
			automationUser := credential.UserID
			if automationUser == "" {
				automationUser = "unknown"
			}
			// Only update InUseBy if no UI or API session is active
			if !foundDevice.HasUISession() && !foundDevice.HasActiveLease() {
				foundDevice.InUseBy = automationUser
				foundDevice.InUseByTenant = credential.Tenant
				foundDevice.InUseTS = time.Now().UnixMilli()
			}
			foundDevice.Mu.Unlock()
		} else {
			// If this is not a request for a new session
			var sessionID = ""

			// Check if the call uses session ID
			if strings.Contains(c.Request.URL.Path, "/session/") {
				var startIndex int
				var endIndex int

				// Extract the session ID from the call URL path
				if c.Request.Method == http.MethodDelete {
					// Find the start and end of the session ID
					startIndex = strings.Index(c.Request.URL.Path, "/session/") + len("/session/")
					endIndex = len(c.Request.URL.Path)
				} else {
					// Find the start and end of the session ID
					startIndex = strings.Index(c.Request.URL.Path, "/session/") + len("/session/")
					endIndex = strings.Index(c.Request.URL.Path[startIndex:], "/") + startIndex
				}

				if startIndex == -1 || endIndex == -1 {
					writeW3CError(c, w3cInvalidSessionID(fmt.Sprintf("No session ID could be extracted from the request - %s", c.Request.URL.Path)))
					return
				}

				sessionID = c.Request.URL.Path[startIndex:endIndex]
			}

			// If no session ID could be parsed from the request
			if sessionID == "" {
				writeW3CError(c, w3cInvalidSessionID("No session ID could be extracted from the request"))
				return
			}

			// Read the request origRequestBody
			origRequestBody, err := readBody(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to read the proxied Appium request origRequestBody", "", err.Error()))
				return
			}
			defer c.Request.Body.Close()

			// Check if there is a device in the local session map for that session ID
			foundDevice, err := getDeviceBySessionID(sessionID)
			if err != nil {
				writeW3CError(c, w3cInvalidSessionID(fmt.Sprintf("No session ID `%s` is available to GADS, it timed out or something unexpected occurred", sessionID)))
				return
			}

			// Set the device last automation action timestamp when call returns
			defer func() {
				foundDevice.Mu.Lock()
				foundDevice.LastAutomationActionTS = time.Now().UnixMilli()
				foundDevice.Mu.Unlock()
			}()

			foundDevice.Mu.RLock()
			deviceHost := foundDevice.Host
			deviceUDID := foundDevice.Device.UDID
			foundDevice.Mu.RUnlock()

			// Create a new request to the device target URL on its provider instance
			proxyReq, err := http.NewRequest(
				c.Request.Method,
				fmt.Sprintf("http://%s/device/%s/appium%s",
					deviceHost,
					deviceUDID,
					strings.Replace(c.Request.URL.Path, "/grid", "", -1)),
				bytes.NewBuffer(origRequestBody),
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to create proxy request for this call", "", err.Error()))
				return
			}

			// Copy headers
			for k, v := range c.Request.Header {
				proxyReq.Header[k] = v
			}

			// Send the request
			client := &http.Client{}
			resp, err := client.Do(proxyReq)
			if err != nil {
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to failed to execute the proxy request to the device respective provider Appium endpoint", "", err.Error()))
				return
			}
			defer resp.Body.Close()

			// If the request succeeded and was a delete request, remove the session ID from the map
			if c.Request.Method == http.MethodDelete {
				foundDevice.Mu.Lock()
				foundDevice.IsAvailableForAutomation = true
				foundDevice.Mu.Unlock()
				// Start a goroutine that will release the device after 1 second if no other actions were taken
				go func() {
					time.Sleep(1 * time.Second)
					foundDevice.Mu.Lock()
					if foundDevice.LastAutomationActionTS <= (time.Now().UnixMilli() - 1000) {
						foundDevice.SessionID = ""
						foundDevice.IsRunningAutomation = false
						foundDevice.ReleaseLockIfNotHeld()
					}
					foundDevice.Mu.Unlock()
				}()
			}

			if resp.StatusCode == http.StatusInternalServerError {
				// Start a goroutine that will release the device after 10 seconds if no other actions were taken
				go func() {
					time.Sleep(10 * time.Second)
					foundDevice.Mu.Lock()
					if foundDevice.LastAutomationActionTS <= (time.Now().UnixMilli() - 10000) {
						foundDevice.SessionID = ""
						foundDevice.IsAvailableForAutomation = true
						foundDevice.IsRunningAutomation = false
						foundDevice.ReleaseLockIfNotHeld()
					}
					foundDevice.Mu.Unlock()
				}()
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS got an internal server error from the proxy request to the device respective provider Appium endpoint", "", ""))
				return
			}

			// Read the response origRequestBody of the proxied request
			proxiedRequestBody, err := readBody(resp.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to read the response origRequestBody of the proxied Appium request", "", err.Error()))
				return
			}

			// Copy the response back to the original client
			for k, v := range resp.Header {
				c.Writer.Header()[k] = v
			}
			c.Writer.WriteHeader(resp.StatusCode)
			c.Writer.Write(proxiedRequestBody)

			foundDevice.Mu.Lock()
			foundDevice.LastAutomationActionTS = time.Now().UnixMilli()
			foundDevice.Mu.Unlock()
		}
	}
}

func readBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return []byte{}, err
	}

	return body, nil
}

func getDeviceBySessionID(sessionID string) (*devices.LocalHubDevice, error) {
	for _, localDevice := range devices.HubDeviceStore.All() {
		localDevice.Mu.RLock()
		sid := localDevice.SessionID
		localDevice.Mu.RUnlock()
		if sid == sessionID {
			return localDevice, nil
		}
	}
	return nil, fmt.Errorf("No device with session ID `%s` was found", sessionID)
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

		// Check if device is in allowed workspaces
		d.Mu.RLock()
		wsID := d.Device.WorkspaceID
		d.Mu.RUnlock()

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
