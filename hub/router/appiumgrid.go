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
	"GADS/common/db"
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
		for _, hubDevice := range devices.HubDeviceStore.All() {
			hubDevice.Mu.Lock()
			// Reset device if its not connected
			// Or it hasn't received any Appium requests in the command timeout and is running automation
			// Or if its provider state is not "live" - device was re-provisioned for example
			if !hubDevice.Device.Connected ||
				(hubDevice.LastAutomationActionTS <= (time.Now().UnixMilli()-hubDevice.AppiumNewCommandTimeout) && hubDevice.IsRunningAutomation) ||
				hubDevice.Device.ProviderState != "live" {
				hubDevice.IsRunningAutomation = false
				hubDevice.IsAvailableForAutomation = true
				hubDevice.SessionID = ""
				if hubDevice.InUseBy != "" {
					hubDevice.InUseBy = ""
					hubDevice.InUseByTenant = ""
					hubDevice.InUseTS = 0
				}
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

			// Unmarshal the request sessionRequestBody []byte to <AppiumSession>
			var appiumSessionBody models.AppiumSession
			err = json.Unmarshal(sessionRequestBody, &appiumSessionBody)
			if err != nil {
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to unmarshal session request sessionRequestBody", "session not created", err.Error()))
				return
			}

			var capsToUse models.CommonCapabilities

			if appiumSessionBody.DesiredCapabilities.PlatformName != "" && appiumSessionBody.DesiredCapabilities.AutomationName != "" {
				capsToUse = appiumSessionBody.DesiredCapabilities
			} else if appiumSessionBody.Capabilities.FirstMatch[0].PlatformName != "" && appiumSessionBody.Capabilities.FirstMatch[0].AutomationName != "" {
				capsToUse = appiumSessionBody.Capabilities.FirstMatch[0]
			} else if appiumSessionBody.Capabilities.AlwaysMatch.PlatformName != "" && appiumSessionBody.Capabilities.AlwaysMatch.AutomationName != "" {
				capsToUse = appiumSessionBody.Capabilities.AlwaysMatch
			} else {
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS did not find any suitable capabilities object in the session request, check your setup or open an issues on the project Github page", "session not created", ""))
				return
			}

			// Extract client secret from capabilities and get allowed workspaces
			var allowedWorkspaceIDs []string
			var sessionReq map[string]interface{}
			json.Unmarshal(sessionRequestBody, &sessionReq)
			capabilityPrefix := getEnvOrDefault("GADS_CAPABILITY_PREFIX", "gads")
			clientSecret := models.ExtractClientSecretFromSession(sessionReq, capabilityPrefix)

			if clientSecret == "" {
				c.JSON(http.StatusUnauthorized, createErrorResponse(
					fmt.Sprintf("Client credentials are required. Provide %s:clientSecret in the capabilities.", capabilityPrefix),
					"session not created",
					""))
				return
			}

			credential, err := db.GlobalMongoStore.GetClientCredentialBySecret(clientSecret)
			if err != nil || !credential.IsActive {
				c.JSON(http.StatusUnauthorized, createErrorResponse("Invalid client credentials", "session not created", ""))
				return
			}

			if credential.Tenant != "" {
				defaultTenant, _ := db.GlobalMongoStore.GetOrCreateDefaultTenant()
				useAllTenantWorkspaces := true

				// Check if we need to filter by user workspaces
				if credential.Tenant == defaultTenant && credential.UserID != "" {
					user, err := db.GlobalMongoStore.GetUser(credential.UserID)
					if err != nil {
						c.JSON(http.StatusUnauthorized, createErrorResponse("User not found", "session not created", ""))
						return
					}

					if user.Role != "admin" {
						// Regular user: only assigned workspaces
						useAllTenantWorkspaces = false
						userWorkspaces := db.GlobalMongoStore.GetUserWorkspaces(credential.UserID)
						for _, ws := range userWorkspaces {
							allowedWorkspaceIDs = append(allowedWorkspaceIDs, ws.ID)
						}
					}
				}

				// Admin users or non-default tenant: all workspaces of the tenant
				if useAllTenantWorkspaces {
					allWorkspaces, _ := db.GlobalMongoStore.GetWorkspaces()
					for _, ws := range allWorkspaces {
						if ws.Tenant == credential.Tenant {
							allowedWorkspaceIDs = append(allowedWorkspaceIDs, ws.ID)
						}
					}
				}
			}

			// Check for available device
			var foundDevice *models.LocalHubDevice
			var deviceErr error

			foundDevice, deviceErr = findAvailableDevice(capsToUse, allowedWorkspaceIDs, credential.UserID, credential.Tenant)

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
						foundDevice, deviceErr = findAvailableDevice(capsToUse, allowedWorkspaceIDs, credential.UserID, credential.Tenant)
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
			if capsToUse.NewCommandTimeout != 0 {
				foundDevice.AppiumNewCommandTimeout = capsToUse.NewCommandTimeout * 1000
			} else {
				foundDevice.AppiumNewCommandTimeout = 60000
			}
			foundDevice.Mu.Unlock()

			updatedSessionBody, _ := json.Marshal(sessionReq)
			// Create a new request to the device target URL
			foundDevice.Mu.RLock()
			deviceHost := foundDevice.Device.Host
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
					// Only clear user info if no manual session is active
					if foundDevice.InUseWSConnection == nil {
						foundDevice.InUseBy = ""
						foundDevice.InUseByTenant = ""
						foundDevice.InUseTS = 0
					}
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
							// Only clear user info if no manual session is active
							if foundDevice.InUseWSConnection == nil {
								foundDevice.InUseBy = ""
								foundDevice.InUseByTenant = ""
								foundDevice.InUseTS = 0
							}
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
			// Only update InUseBy if no manual session is active
			if foundDevice.InUseWSConnection == nil {
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
					c.JSON(http.StatusInternalServerError, createErrorResponse(fmt.Sprintf("No session ID could be extracted from the request - %s", c.Request.URL.Path), "", ""))
					return
				}

				sessionID = c.Request.URL.Path[startIndex:endIndex]
			}

			// If no session ID could be parsed from the request
			if sessionID == "" {
				c.JSON(http.StatusInternalServerError, createErrorResponse("No session ID could be extracted from the request", "", ""))
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
				c.JSON(http.StatusNotFound, createErrorResponse(fmt.Sprintf("No session ID `%s` is available to GADS, it timed out or something unexpected occurred", sessionID), "", ""))
				return
			}

			// Set the device last automation action timestamp when call returns
			defer func() {
				foundDevice.Mu.Lock()
				foundDevice.LastAutomationActionTS = time.Now().UnixMilli()
				foundDevice.Mu.Unlock()
			}()

			foundDevice.Mu.RLock()
			deviceHost := foundDevice.Device.Host
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
						// Only clear user info if no manual session is active
						if foundDevice.InUseWSConnection == nil {
							foundDevice.InUseBy = ""
							foundDevice.InUseByTenant = ""
							foundDevice.InUseTS = 0
						}
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
						// Only clear user info if no manual session is active
						if foundDevice.InUseWSConnection == nil {
							foundDevice.InUseBy = ""
							foundDevice.InUseByTenant = ""
							foundDevice.InUseTS = 0
						}
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

func getDeviceBySessionID(sessionID string) (*models.LocalHubDevice, error) {
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

func getDeviceByUDID(udid string) (*models.LocalHubDevice, error) {
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

func getTargetOSFromCaps(caps models.CommonCapabilities) string {
	if strings.EqualFold(caps.PlatformName, "iOS") ||
		strings.EqualFold(caps.AutomationName, "XCUITest") {
		return "ios"
	}

	if strings.EqualFold(caps.PlatformName, "Android") ||
		strings.EqualFold(caps.AutomationName, "UiAutomator2") {
		return "android"
	}

	if strings.EqualFold(caps.PlatformName, "TizenTV") ||
		strings.EqualFold(caps.AutomationName, "TizenTV") {
		return "tizen"
	}

	if strings.EqualFold(caps.PlatformName, "lgtv") ||
		strings.EqualFold(caps.AutomationName, "webos") {
		return "webos"
	}

	return ""
}

func findAvailableDevice(caps models.CommonCapabilities, allowedWorkspaceIDs []string, userID string, userTenant string) (*models.LocalHubDevice, error) {
	var foundDevice *models.LocalHubDevice

	var deviceUDID = ""
	if caps.DeviceUDID != "" {
		deviceUDID = caps.DeviceUDID
	}

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

	var availableDevices []*models.LocalHubDevice

	targetOS := getTargetOSFromCaps(caps)
	if targetOS != "" {
		for _, localDevice := range devices.HubDeviceStore.All() {
			localDevice.Mu.RLock()
			os := localDevice.Device.OS
			connected := localDevice.Device.Connected
			state := localDevice.Device.ProviderState
			lastUpdated := localDevice.Device.LastUpdatedTimestamp
			available := localDevice.IsAvailableForAutomation
			usage := localDevice.Device.Usage
			wsID := localDevice.Device.WorkspaceID
			inUseBy := localDevice.InUseBy
			inUseByTenant := localDevice.InUseByTenant
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

			if inUseBy != "" && inUseByTenant != "" {
				currentUser := userID
				if currentUser == "" {
					currentUser = "unknown"
				}
				if inUseBy != currentUser || inUseByTenant != userTenant {
					continue
				}
			}

			availableDevices = append(availableDevices, localDevice)
		}
	}

	if caps.PlatformVersion != "" {
		// First try exact version match
		for _, device := range availableDevices {
			device.Mu.RLock()
			osVersion := device.Device.OSVersion
			device.Mu.RUnlock()

			if osVersion == caps.PlatformVersion {
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
			v, _ := semver.NewVersion(caps.PlatformVersion)
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
