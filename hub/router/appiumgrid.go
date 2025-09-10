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

// Service layer interfaces

// AuthService handles authentication and workspace access control
type AuthService interface {
	ValidateCredentials(clientSecret string) (*models.ClientCredentials, *AppiumError)
	GetAllowedWorkspaces(credential *models.ClientCredentials) ([]string, *AppiumError)
}

// DeviceService handles device allocation and state management
type DeviceService interface {
	FindAndReserveDevice(caps models.CommonCapabilities, workspaceIDs []string, userID, tenant string) (*models.LocalHubDevice, *AppiumError)
	ReleaseDevice(device *models.LocalHubDevice)
	ReleaseDeviceWithCleanup(device *models.LocalHubDevice, clearUserInfo bool)
	SetDeviceInUse(device *models.LocalHubDevice, userID, tenant string)
}

// SessionService handles session lifecycle operations  
type SessionService interface {
	CreateProxyRequest(device *models.LocalHubDevice, originalReq *http.Request, body []byte) (*http.Request, *AppiumError)
	ExecuteProxyRequest(req *http.Request) (*http.Response, *AppiumError)
	ExtractSessionID(responseBody []byte) (string, *AppiumError)
	FindDeviceBySessionID(sessionID string) (*models.LocalHubDevice, *AppiumError)
}

// Service implementations

// authService implements AuthService interface
type authService struct{}

func (s *authService) ValidateCredentials(clientSecret string) (*models.ClientCredentials, *AppiumError) {
	credential, err := db.GlobalMongoStore.GetClientCredentialBySecret(clientSecret)
	if err != nil || !credential.IsActive {
		return nil, ErrInvalidClientCredentials.WithCause(err)
	}
	return &credential, nil
}

func (s *authService) GetAllowedWorkspaces(credential *models.ClientCredentials) ([]string, *AppiumError) {
	var allowedWorkspaceIDs []string
	
	if credential.Tenant != "" {
		defaultTenant, _ := db.GlobalMongoStore.GetOrCreateDefaultTenant()
		useAllTenantWorkspaces := true

		// Check if we need to filter by user workspaces
		if credential.Tenant == defaultTenant && credential.UserID != "" {
			user, err := db.GlobalMongoStore.GetUser(credential.UserID)
			if err != nil {
				return nil, ErrUserNotFound.WithCause(err)
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
	
	return allowedWorkspaceIDs, nil
}

// deviceService implements DeviceService interface
type deviceService struct{}

func (s *deviceService) FindAndReserveDevice(caps models.CommonCapabilities, workspaceIDs []string, userID, tenant string) (*models.LocalHubDevice, *AppiumError) {
	foundDevice, err := findAvailableDevice(caps, workspaceIDs, userID, tenant)
	if err != nil {
		if strings.Contains(err.Error(), "No device with udid") {
			return nil, ErrNoAvailableDevice.WithCause(err)
		}
		return nil, ErrNoAvailableDevice.WithCause(err)
	}

	if foundDevice == nil {
		// Wait up to 10 seconds for a device to become available
		ticker := time.NewTicker(100 * time.Millisecond)
		timeout := time.After(10 * time.Second)
		
		for {
			select {
			case <-ticker.C:
				foundDevice, err = findAvailableDevice(caps, workspaceIDs, userID, tenant)
				if foundDevice != nil {
					ticker.Stop()
					goto deviceFound
				}
			case <-timeout:
				ticker.Stop()
				if err != nil {
					return nil, ErrNoAvailableDevice.WithCause(err)
				}
				return nil, ErrNoAvailableDevice
			}
		}
	}

deviceFound:
	if foundDevice == nil {
		return nil, ErrNoAvailableDevice
	}

	// Reserve the device
	devices.HubDevicesData.Mu.Lock()
	foundDevice.IsRunningAutomation = true
	foundDevice.IsAvailableForAutomation = false
	foundDevice.LastAutomationActionTS = time.Now().UnixMilli()
	if caps.NewCommandTimeout != 0 {
		foundDevice.AppiumNewCommandTimeout = caps.NewCommandTimeout * 1000
	} else {
		foundDevice.AppiumNewCommandTimeout = 60000
	}
	devices.HubDevicesData.Mu.Unlock()

	return foundDevice, nil
}

func (s *deviceService) ReleaseDevice(device *models.LocalHubDevice) {
	releaseDevice(device)
}

func (s *deviceService) ReleaseDeviceWithCleanup(device *models.LocalHubDevice, clearUserInfo bool) {
	releaseDeviceWithUserCleanup(device, clearUserInfo)
}

func (s *deviceService) SetDeviceInUse(device *models.LocalHubDevice, userID, tenant string) {
	devices.HubDevicesData.Mu.Lock()
	defer devices.HubDevicesData.Mu.Unlock()
	
	device.LastAutomationActionTS = time.Now().UnixMilli()
	automationUser := userID
	if automationUser == "" {
		automationUser = "unknown"
	}
	// Only update InUseBy if no manual session is active
	if device.InUseWSConnection == nil {
		device.InUseBy = automationUser
		device.InUseByTenant = tenant
		device.InUseTS = time.Now().UnixMilli()
	}
}

// sessionService implements SessionService interface
type sessionService struct{}

func (s *sessionService) CreateProxyRequest(device *models.LocalHubDevice, originalReq *http.Request, body []byte) (*http.Request, *AppiumError) {
	proxyURL := createProxyURL(device.Device.Host, device.Device.UDID, originalReq.URL.Path)
	proxyReq, err := http.NewRequest(originalReq.Method, proxyURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, ErrCreateProxyRequest.WithCause(err)
	}

	// Copy headers from the original request to the new request
	for k, v := range originalReq.Header {
		proxyReq.Header[k] = v
	}

	return proxyReq, nil
}

func (s *sessionService) ExecuteProxyRequest(req *http.Request) (*http.Response, *AppiumError) {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, ErrExecuteProxyRequest.WithCause(err)
	}
	return resp, nil
}

func (s *sessionService) ExtractSessionID(responseBody []byte) (string, *AppiumError) {
	return parseAppiumSessionResponse(responseBody)
}

func (s *sessionService) FindDeviceBySessionID(sessionID string) (*models.LocalHubDevice, *AppiumError) {
	devices.HubDevicesData.Mu.Lock()
	foundDevice, err := getDeviceBySessionID(sessionID)
	devices.HubDevicesData.Mu.Unlock()
	if err != nil {
		customErr := ErrSessionIDNotFound.WithMessage(fmt.Sprintf("No session ID `%s` is available to GADS, it timed out or something unexpected occurred", sessionID))
		return nil, customErr
	}
	return foundDevice, nil
}

// Service instances
var (
	AuthSvc    AuthService    = &authService{}
	DeviceSvc  DeviceService  = &deviceService{}
	SessionSvc SessionService = &sessionService{}
)

// AppiumError represents a structured error for Appium Grid operations
type AppiumError struct {
	Code       string
	Message    string
	StatusCode int
	Cause      error
	ErrorType  string
}

func (e *AppiumError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Predefined error types
var (
	ErrSessionNotCreated   = "session not created"
	ErrSessionNotFound     = "session not found"
	ErrInvalidRequest      = "invalid request"
	ErrDeviceNotFound      = "device not found"
	ErrUnauthorized        = "unauthorized"
	ErrInternalServerError = "internal server error"
)

// Common Appium Grid errors
var (
	ErrReadRequestBody = &AppiumError{
		Code: "REQUEST_READ_FAILED", Message: "Failed to read request body",
		StatusCode: http.StatusInternalServerError, ErrorType: ErrSessionNotCreated,
	}
	ErrUnmarshalRequest = &AppiumError{
		Code: "REQUEST_UNMARSHAL_FAILED", Message: "Failed to unmarshal request body",
		StatusCode: http.StatusInternalServerError, ErrorType: ErrSessionNotCreated,
	}
	ErrNoSuitableCapabilities = &AppiumError{
		Code: "INVALID_CAPABILITIES", Message: "No suitable capabilities found in session request",
		StatusCode: http.StatusInternalServerError, ErrorType: ErrSessionNotCreated,
	}
	ErrMissingClientCredentials = &AppiumError{
		Code: "MISSING_CREDENTIALS", Message: "Client credentials required",
		StatusCode: http.StatusUnauthorized, ErrorType: ErrSessionNotCreated,
	}
	ErrInvalidClientCredentials = &AppiumError{
		Code: "INVALID_CREDENTIALS", Message: "Invalid client credentials",
		StatusCode: http.StatusUnauthorized, ErrorType: ErrSessionNotCreated,
	}
	ErrUserNotFound = &AppiumError{
		Code: "USER_NOT_FOUND", Message: "User not found",
		StatusCode: http.StatusUnauthorized, ErrorType: ErrSessionNotCreated,
	}
	ErrNoAvailableDevice = &AppiumError{
		Code: "NO_DEVICE_AVAILABLE", Message: "No available device found",
		StatusCode: http.StatusNotFound, ErrorType: ErrSessionNotCreated,
	}
	ErrCreateProxyRequest = &AppiumError{
		Code: "PROXY_REQUEST_CREATE_FAILED", Message: "Failed to create proxy request",
		StatusCode: http.StatusInternalServerError, ErrorType: ErrSessionNotCreated,
	}
	ErrExecuteProxyRequest = &AppiumError{
		Code: "PROXY_REQUEST_EXECUTE_FAILED", Message: "Failed to execute proxy request",
		StatusCode: http.StatusInternalServerError, ErrorType: ErrSessionNotCreated,
	}
	ErrReadProxyResponse = &AppiumError{
		Code: "PROXY_RESPONSE_READ_FAILED", Message: "Failed to read proxy response",
		StatusCode: http.StatusInternalServerError, ErrorType: ErrSessionNotCreated,
	}
	ErrUnmarshalProxyResponse = &AppiumError{
		Code: "PROXY_RESPONSE_UNMARSHAL_FAILED", Message: "Failed to unmarshal proxy response",
		StatusCode: http.StatusInternalServerError, ErrorType: ErrSessionNotCreated,
	}
	ErrSessionIDExtraction = &AppiumError{
		Code: "SESSION_ID_EXTRACTION_FAILED", Message: "Failed to extract session ID from request",
		StatusCode: http.StatusInternalServerError, ErrorType: ErrInvalidRequest,
	}
	ErrSessionIDNotFound = &AppiumError{
		Code: "SESSION_ID_NOT_FOUND", Message: "Session ID not found or expired",
		StatusCode: http.StatusNotFound, ErrorType: ErrSessionNotFound,
	}
)

// Helper functions for error handling

// WithCause creates a new AppiumError with a cause
func (e *AppiumError) WithCause(cause error) *AppiumError {
	return &AppiumError{
		Code:       e.Code,
		Message:    e.Message,
		StatusCode: e.StatusCode,
		ErrorType:  e.ErrorType,
		Cause:      cause,
	}
}

// WithMessage creates a new AppiumError with a custom message
func (e *AppiumError) WithMessage(message string) *AppiumError {
	return &AppiumError{
		Code:       e.Code,
		Message:    message,
		StatusCode: e.StatusCode,
		ErrorType:  e.ErrorType,
		Cause:      e.Cause,
	}
}

// respondWithAppiumError sends an AppiumError as JSON response
func respondWithAppiumError(c *gin.Context, err *AppiumError) {
	stackTrace := ""
	if err.Cause != nil {
		stackTrace = err.Cause.Error()
	}

	response := createErrorResponse(err.Message, err.ErrorType, stackTrace)
	c.JSON(err.StatusCode, response)
}

// createAppiumError creates a new AppiumError with custom values
func createAppiumError(code, message, errorType string, statusCode int, cause error) *AppiumError {
	return &AppiumError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		ErrorType:  errorType,
		Cause:      cause,
	}
}

// Device cleanup utility functions

// releaseDevice immediately releases a device from automation use
func releaseDevice(device *models.LocalHubDevice) {
	devices.HubDevicesData.Mu.Lock()
	defer devices.HubDevicesData.Mu.Unlock()

	device.IsAvailableForAutomation = true
	device.IsRunningAutomation = false
}

// releaseDeviceWithUserCleanup releases a device and optionally clears user info
func releaseDeviceWithUserCleanup(device *models.LocalHubDevice, clearUserInfo bool) {
	devices.HubDevicesData.Mu.Lock()
	defer devices.HubDevicesData.Mu.Unlock()

	device.IsAvailableForAutomation = true
	device.IsRunningAutomation = false

	// Only clear user info if requested and no manual session is active
	if clearUserInfo && device.InUseWSConnection == nil {
		device.InUseBy = ""
		device.InUseByTenant = ""
		device.InUseTS = 0
	}
}

// releaseDeviceCompletely fully releases a device including session ID and user info
func releaseDeviceCompletely(device *models.LocalHubDevice) {
	devices.HubDevicesData.Mu.Lock()
	defer devices.HubDevicesData.Mu.Unlock()

	device.IsAvailableForAutomation = true
	device.IsRunningAutomation = false
	device.SessionID = ""

	// Only clear user info if no manual session is active
	if device.InUseWSConnection == nil {
		device.InUseBy = ""
		device.InUseByTenant = ""
		device.InUseTS = 0
	}
}

// conditionalDeviceRelease releases device only if the last action timestamp meets the condition
func conditionalDeviceRelease(device *models.LocalHubDevice, timeThresholdMs int64, includeSessionID bool) {
	devices.HubDevicesData.Mu.Lock()
	defer devices.HubDevicesData.Mu.Unlock()

	if device.LastAutomationActionTS <= (time.Now().UnixMilli() - timeThresholdMs) {
		device.IsAvailableForAutomation = true
		device.IsRunningAutomation = false

		if includeSessionID {
			device.SessionID = ""
		}

		// Only clear user info if no manual session is active
		if device.InUseWSConnection == nil {
			device.InUseBy = ""
			device.InUseByTenant = ""
			device.InUseTS = 0
		}
	}
}

// setDeviceAvailable sets device as available for automation
func setDeviceAvailable(device *models.LocalHubDevice) {
	devices.HubDevicesData.Mu.Lock()
	defer devices.HubDevicesData.Mu.Unlock()
	device.IsAvailableForAutomation = true
}

// Request parsing utility functions

// SessionRequest contains the parsed session request data
type SessionRequest struct {
	Body            []byte
	AppiumSession   models.AppiumSession
	Capabilities    models.CommonCapabilities
	ClientSecret    string
	RawCapabilities map[string]interface{}
}

// parseSessionRequest parses and validates an Appium session creation request
func parseSessionRequest(c *gin.Context) (*SessionRequest, *AppiumError) {
	// Read request body
	sessionRequestBody, err := readBody(c.Request.Body)
	if err != nil {
		return nil, ErrReadRequestBody.WithCause(err)
	}
	defer c.Request.Body.Close()

	// Parse Appium session structure
	var appiumSessionBody models.AppiumSession
	err = json.Unmarshal(sessionRequestBody, &appiumSessionBody)
	if err != nil {
		return nil, ErrUnmarshalRequest.WithCause(err)
	}

	// Extract capabilities to use
	var capsToUse models.CommonCapabilities
	if appiumSessionBody.DesiredCapabilities.PlatformName != "" && appiumSessionBody.DesiredCapabilities.AutomationName != "" {
		capsToUse = appiumSessionBody.DesiredCapabilities
	} else if len(appiumSessionBody.Capabilities.FirstMatch) > 0 && appiumSessionBody.Capabilities.FirstMatch[0].PlatformName != "" && appiumSessionBody.Capabilities.FirstMatch[0].AutomationName != "" {
		capsToUse = appiumSessionBody.Capabilities.FirstMatch[0]
	} else if appiumSessionBody.Capabilities.AlwaysMatch.PlatformName != "" && appiumSessionBody.Capabilities.AlwaysMatch.AutomationName != "" {
		capsToUse = appiumSessionBody.Capabilities.AlwaysMatch
	} else {
		return nil, ErrNoSuitableCapabilities
	}

	// Parse raw capabilities for client secret extraction
	var sessionReq map[string]interface{}
	json.Unmarshal(sessionRequestBody, &sessionReq)
	capabilityPrefix := getEnvOrDefault("GADS_CAPABILITY_PREFIX", "gads")
	clientSecret := models.ExtractClientSecretFromSession(sessionReq, capabilityPrefix)

	if clientSecret == "" {
		customErr := ErrMissingClientCredentials.WithMessage(
			fmt.Sprintf("Client credentials are required. Provide %s:clientSecret in the capabilities.", capabilityPrefix))
		return nil, customErr
	}

	return &SessionRequest{
		Body:            sessionRequestBody,
		AppiumSession:   appiumSessionBody,
		Capabilities:    capsToUse,
		ClientSecret:    clientSecret,
		RawCapabilities: sessionReq,
	}, nil
}

// extractSessionID extracts session ID from URL path
func extractSessionID(urlPath string, isDelete bool) (string, *AppiumError) {
	if !strings.Contains(urlPath, "/session/") {
		return "", ErrSessionIDExtraction
	}

	var startIndex, endIndex int

	if isDelete {
		// Find the start and end of the session ID
		startIndex = strings.Index(urlPath, "/session/") + len("/session/")
		endIndex = len(urlPath)
	} else {
		// Find the start and end of the session ID
		startIndex = strings.Index(urlPath, "/session/") + len("/session/")
		endIndex = strings.Index(urlPath[startIndex:], "/") + startIndex
	}

	if startIndex == -1 || endIndex == -1 {
		customErr := ErrSessionIDExtraction.WithMessage(fmt.Sprintf("No session ID could be extracted from the request - %s", urlPath))
		return "", customErr
	}

	sessionID := urlPath[startIndex:endIndex]
	if sessionID == "" {
		return "", ErrSessionIDExtraction
	}

	return sessionID, nil
}

// createProxyURL creates the target URL for proxying requests
func createProxyURL(deviceHost, deviceUDID, originalPath string) string {
	cleanPath := strings.Replace(originalPath, "/grid", "", -1)
	return fmt.Sprintf("http://%s/device/%s/appium%s", deviceHost, deviceUDID, cleanPath)
}

// parseAppiumSessionResponse parses the Appium session response to extract session ID
func parseAppiumSessionResponse(responseBody []byte) (string, *AppiumError) {
	var proxySessionResponse AppiumSessionResponse
	err := json.Unmarshal(responseBody, &proxySessionResponse)
	if err != nil {
		return "", ErrUnmarshalProxyResponse.WithCause(err)
	}
	return proxySessionResponse.Value.SessionID, nil
}

// Every 3 seconds check the devices
// And clean the automation session if no action was taken in the timeout limit
func UpdateExpiredGridSessions() {
	for {
		devices.HubDevicesData.Mu.Lock()
		for _, hubDevice := range devices.HubDevicesData.Devices {
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
		}
		devices.HubDevicesData.Mu.Unlock()
		time.Sleep(1 * time.Second)
	}
}

// AppiumGridMiddleware coordinates between session creation and session requests
func AppiumGridMiddleware(config *models.HubConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasSuffix(c.Request.URL.Path, "/session") {
			handleSessionCreation(c)
		} else {
			handleSessionRequest(c)
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
	for _, localDevice := range devices.HubDevicesData.Devices {
		if localDevice.SessionID == sessionID {
			return localDevice, nil
		}
	}
	return nil, fmt.Errorf("No device with session ID `%s` was found", sessionID)
}

func getDeviceByUDID(udid string) (*models.LocalHubDevice, error) {
	for _, localDevice := range devices.HubDevicesData.Devices {
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
	devices.HubDevicesData.Mu.Lock()
	defer devices.HubDevicesData.Mu.Unlock()

	var foundDevice *models.LocalHubDevice

	var deviceUDID = ""
	if caps.DeviceUDID != "" {
		deviceUDID = caps.DeviceUDID
	}

	if len(allowedWorkspaceIDs) == 0 {
		return nil, fmt.Errorf("No device with udid `%s` was found in allowed workspaces", deviceUDID)
	}

	if deviceUDID != "" {
		foundDevice, err := getDeviceByUDID(deviceUDID)
		if err != nil {
			return nil, err
		}

		// Check if device is in allowed workspaces
		deviceAllowed := false
		for _, wsID := range allowedWorkspaceIDs {
			if foundDevice.Device.WorkspaceID == wsID {
				deviceAllowed = true
				break
			}
		}
		if !deviceAllowed {
			return nil, fmt.Errorf("No device with udid `%s` was found", deviceUDID)
		}

		if foundDevice.IsAvailableForAutomation {
			foundDevice.IsAvailableForAutomation = false
			return foundDevice, nil
		} else {
			return nil, fmt.Errorf("Device is currently not available for automation")
		}

	} else {
		var availableDevices []*models.LocalHubDevice

		targetOS := getTargetOSFromCaps(caps)
		if targetOS != "" {
			// Loop through all latest devices looking for a device that is not currently `being prepared` for automation and the last time it was updated from provider was less than 3 seconds ago
			// Also device should not be disabled or for remote control only
			for _, localDevice := range devices.HubDevicesData.Devices {
				if strings.EqualFold(localDevice.Device.OS, targetOS) &&
					localDevice.Device.Connected &&
					localDevice.Device.ProviderState == "live" &&
					localDevice.Device.LastUpdatedTimestamp >= (time.Now().UnixMilli()-3000) &&
					localDevice.IsAvailableForAutomation &&
					localDevice.Device.Usage != "control" &&
					localDevice.Device.Usage != "disabled" {

					// Check if device is in allowed workspaces
					deviceAllowed := false
					for _, wsID := range allowedWorkspaceIDs {
						if localDevice.Device.WorkspaceID == wsID {
							deviceAllowed = true
							break
						}
					}
					if !deviceAllowed {
						continue
					}

					// Check if device is in use by another user
					if localDevice.InUseBy != "" && localDevice.InUseByTenant != "" {
						currentUser := userID
						if currentUser == "" {
							currentUser = "unknown"
						}
						if localDevice.InUseBy != currentUser || localDevice.InUseByTenant != userTenant {
							continue
						}
					}

					availableDevices = append(availableDevices, localDevice)
				}
			}
		}

		// If we have `appium:platformVersion` capability provided, then we want to filter out the devices even more
		// Loop through the accumulated available devices slice and get a device that matches the platform version
		if caps.PlatformVersion != "" {
			// First check if device completely matches the required version
			if len(availableDevices) != 0 {
				for _, device := range availableDevices {
					if device.Device.OSVersion == caps.PlatformVersion {
						foundDevice = device
						foundDevice.IsAvailableForAutomation = false
						break
					}
				}
			}
			// If no device completely matches the required version try a major version
			if foundDevice == nil {
				v, _ := semver.NewVersion(caps.PlatformVersion)
				requestedMajorVersion := fmt.Sprintf("%d", v.Major())
				// Create a constraint for the requested version
				constraint, _ := semver.NewConstraint(fmt.Sprintf("^%s.0.0", requestedMajorVersion))

				if len(availableDevices) != 0 {
					for _, device := range availableDevices {
						deviceV, _ := semver.NewVersion(device.Device.OSVersion)
						if constraint.Check(deviceV) {
							foundDevice = device
							foundDevice.IsAvailableForAutomation = false
							break
						}
					}
				}
			}
		} else {
			// If no platform version capability is provided, get the first device from the available list
			if len(availableDevices) != 0 {
				foundDevice = availableDevices[0]
				foundDevice.IsAvailableForAutomation = false
			}
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

// handleSessionCreation processes Appium session creation requests
func handleSessionCreation(c *gin.Context) {
	// Parse session creation request
	sessionReq, appiumErr := parseSessionRequest(c)
	if appiumErr != nil {
		respondWithAppiumError(c, appiumErr)
		return
	}

	// Validate credentials using AuthService
	credential, authErr := AuthSvc.ValidateCredentials(sessionReq.ClientSecret)
	if authErr != nil {
		respondWithAppiumError(c, authErr)
		return
	}

	// Get allowed workspaces using AuthService
	allowedWorkspaceIDs, workspaceErr := AuthSvc.GetAllowedWorkspaces(credential)
	if workspaceErr != nil {
		respondWithAppiumError(c, workspaceErr)
		return
	}

	// Find and reserve device using DeviceService
	foundDevice, deviceErr := DeviceSvc.FindAndReserveDevice(sessionReq.Capabilities, allowedWorkspaceIDs, credential.UserID, credential.Tenant)
	if deviceErr != nil {
		respondWithAppiumError(c, deviceErr)
		return
	}

	// Create proxy request using SessionService
	proxyReq, proxyErr := SessionSvc.CreateProxyRequest(foundDevice, c.Request, sessionReq.Body)
	if proxyErr != nil {
		DeviceSvc.ReleaseDevice(foundDevice)
		respondWithAppiumError(c, proxyErr)
		return
	}

	// Execute proxy request using SessionService
	resp, execErr := SessionSvc.ExecuteProxyRequest(proxyReq)
	if execErr != nil {
		DeviceSvc.ReleaseDevice(foundDevice)
		respondWithAppiumError(c, execErr)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Release device for any error status, clear user info only for non-500 errors
		clearUserInfo := resp.StatusCode != http.StatusInternalServerError
		DeviceSvc.ReleaseDeviceWithCleanup(foundDevice, clearUserInfo)

		// For 500 errors, keep the existing behavior with goroutine
		if resp.StatusCode == http.StatusInternalServerError {
			go func() {
				time.Sleep(10 * time.Second)
				conditionalDeviceRelease(foundDevice, 5000, true)
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

	// Read and parse the response from the proxied request
	proxiedSessionResponseBody, err := readBody(resp.Body)
	if err != nil {
		DeviceSvc.ReleaseDevice(foundDevice)
		respondWithAppiumError(c, ErrReadProxyResponse.WithCause(err))
		return
	}

	// Extract session ID using SessionService
	sessionID, parseErr := SessionSvc.ExtractSessionID(proxiedSessionResponseBody)
	if parseErr != nil {
		DeviceSvc.ReleaseDevice(foundDevice)
		respondWithAppiumError(c, parseErr)
		return
	}

	// Update device with session ID
	devices.HubDevicesData.Mu.Lock()
	foundDevice.SessionID = sessionID
	devices.HubDevicesData.Mu.Unlock()

	// Copy the response back to the original client
	for k, v := range resp.Header {
		c.Writer.Header()[k] = v
	}
	c.Writer.WriteHeader(resp.StatusCode)
	c.Writer.Write(proxiedSessionResponseBody)

	// Set device in use using DeviceService
	DeviceSvc.SetDeviceInUse(foundDevice, credential.UserID, credential.Tenant)
}

// handleSessionRequest processes Appium session-related requests (non-creation)
func handleSessionRequest(c *gin.Context) {
	// Extract session ID from request
	isDelete := c.Request.Method == http.MethodDelete
	sessionID, sessionErr := extractSessionID(c.Request.URL.Path, isDelete)
	if sessionErr != nil {
		respondWithAppiumError(c, sessionErr)
		return
	}

	// Read the request body
	origRequestBody, err := readBody(c.Request.Body)
	if err != nil {
		respondWithAppiumError(c, ErrReadRequestBody.WithCause(err))
		return
	}
	defer c.Request.Body.Close()

	// Find device by session ID using SessionService
	foundDevice, deviceErr := SessionSvc.FindDeviceBySessionID(sessionID)
	if deviceErr != nil {
		respondWithAppiumError(c, deviceErr)
		return
	}

	// Set the device last automation action timestamp when call returns
	defer func() {
		foundDevice.LastAutomationActionTS = time.Now().UnixMilli()
	}()

	// Create proxy request using SessionService
	proxyReq, proxyErr := SessionSvc.CreateProxyRequest(foundDevice, c.Request, origRequestBody)
	if proxyErr != nil {
		respondWithAppiumError(c, proxyErr)
		return
	}

	// Execute proxy request using SessionService
	resp, execErr := SessionSvc.ExecuteProxyRequest(proxyReq)
	if execErr != nil {
		respondWithAppiumError(c, execErr)
		return
	}
	defer resp.Body.Close()

	// Handle session deletion
	if c.Request.Method == http.MethodDelete {
		setDeviceAvailable(foundDevice)
		// Start a goroutine that will release the device after 1 second if no other actions were taken
		go func() {
			time.Sleep(1 * time.Second)
			conditionalDeviceRelease(foundDevice, 1000, true)
		}()
	}

	// Handle server errors
	if resp.StatusCode == http.StatusInternalServerError {
		// Start a goroutine that will release the device after 10 seconds if no other actions were taken
		go func() {
			time.Sleep(10 * time.Second)
			conditionalDeviceRelease(foundDevice, 10000, true)
		}()
		customErr := createAppiumError("PROXY_SERVER_ERROR", "Internal server error from device provider", ErrInternalServerError, http.StatusInternalServerError, nil)
		respondWithAppiumError(c, customErr)
		return
	}

	// Read the response body of the proxied request
	proxiedRequestBody, err := readBody(resp.Body)
	if err != nil {
		respondWithAppiumError(c, ErrReadProxyResponse.WithCause(err))
		return
	}

	// Copy the response back to the original client
	for k, v := range resp.Header {
		c.Writer.Header()[k] = v
	}
	c.Writer.WriteHeader(resp.StatusCode)
	c.Writer.Write(proxiedRequestBody)
	foundDevice.LastAutomationActionTS = time.Now().UnixMilli()
}
