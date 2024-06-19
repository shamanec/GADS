package router

import (
	"GADS/common/models"
	"GADS/hub/devices"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Masterminds/semver"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Capabilities struct {
	FirstMatch []CapabilitiesFirstMatch `json:"firstMatch"`
}

type DesiredCapabilities struct {
	AutomationName    string `json:"appium:automationName"`
	BundleID          string `json:"appium:bundleId"`
	PlatformVersion   string `json:"appium:platformVersion"`
	PlatformName      string `json:"platformName"`
	DeviceUDID        string `json:"appium:udid"`
	NewCommandTimeout int64  `json:"appium:newCommandTimeout"`
	SessionTimeout    int64  `json:"appium:sessionTimeout"`
}

type CapabilitiesFirstMatch struct {
	AutomationName    string `json:"appium:automationName"`
	BundleID          string `json:"appium:bundleId"`
	PlatformVersion   string `json:"appium:platformVersion"`
	PlatformName      string `json:"platformName"`
	DeviceUDID        string `json:"appium:udid"`
	NewCommandTimeout int64  `json:"appium:newCommandTimeout"`
	SessionTimeout    int64  `json:"appium:sessionTimeout"`
}

type AppiumSession struct {
	Capabilities        Capabilities        `json:"capabilities"`
	DesiredCapabilities DesiredCapabilities `json:"desiredCapabilities"`
}

type AppiumSessionValue struct {
	SessionID string `json:"sessionId"`
}

type AppiumSessionResponse struct {
	Value AppiumSessionValue `json:"value"`
}

var sessionMapMu sync.Mutex
var devicesMapMu sync.Mutex

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
		devicesMapMu.Lock()
		for _, hubDevice := range devices.HubDevicesMap {
			if hubDevice.LastAutomationActionTS <= (time.Now().UnixMilli()-hubDevice.AppiumNewCommandTimeout) && hubDevice.IsRunningAutomation {
				hubDevice.IsRunningAutomation = false
				hubDevice.IsPreparingAutomation = false
				hubDevice.SessionID = ""
				if hubDevice.InUseBy == "automation" {
					hubDevice.InUseBy = ""
				}
			}
		}
		devicesMapMu.Unlock()
		time.Sleep(3 * time.Second)
	}
}

func AppiumGridMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasSuffix(c.Request.URL.Path, "/session") {
			// Read the request sessionRequestBody
			sessionRequestBody, err := readBody(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to read session request sessionRequestBody", "", err.Error()))
				return
			}
			defer c.Request.Body.Close()

			// Unmarshal the request sessionRequestBody []byte to <AppiumSession>
			var appiumSessionBody AppiumSession
			err = json.Unmarshal(sessionRequestBody, &appiumSessionBody)
			if err != nil {
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to unmarshal session request sessionRequestBody", "", err.Error()))
				return
			}

			//fmt.Printf(string(sessionRequestBody))

			// Check for available device
			var foundDevice *models.LocalHubDevice

			foundDevice, err = findAvailableDevice(appiumSessionBody)
			// If no device is available start checking each second for 60 seconds
			// If no device is available after 60 seconds - return error
			if foundDevice == nil {
				ticker := time.NewTicker(100 * time.Millisecond)
				timeout := time.After(60 * time.Second)
				notify := c.Writer.CloseNotify()
			FOR_LOOP:
				for {
					select {
					case <-ticker.C:
						foundDevice, err = findAvailableDevice(appiumSessionBody)
						if foundDevice != nil {
							break FOR_LOOP
						}
					case <-timeout:
						ticker.Stop()
						responseError := SeleniumSessionErrorResponse{
							Value: SeleniumSessionErrorResponseValue{
								Message: "No available device found",
							},
						}
						c.JSON(http.StatusInternalServerError, responseError)
						return
					case <-notify:
						ticker.Stop()
						return
					}
				}
			}

			if foundDevice == nil {
				return
			}

			devicesMapMu.Lock()
			// Update the session timeout values if none were provided
			if appiumSessionBody.Capabilities.FirstMatch[0].NewCommandTimeout != 0 {
				foundDevice.AppiumNewCommandTimeout = appiumSessionBody.Capabilities.FirstMatch[0].NewCommandTimeout * 1000
			} else if appiumSessionBody.DesiredCapabilities.NewCommandTimeout != 0 {
				foundDevice.AppiumNewCommandTimeout = appiumSessionBody.DesiredCapabilities.NewCommandTimeout * 1000
			} else {
				foundDevice.AppiumNewCommandTimeout = 60000
			}
			devicesMapMu.Unlock()

			// Create a new request to the device target URL on its provider instance
			proxyReq, err := http.NewRequest(c.Request.Method, fmt.Sprintf("http://%s/device/%s/appium%s", foundDevice.Device.Host, foundDevice.Device.UDID, strings.Replace(c.Request.URL.Path, "/grid", "", -1)), bytes.NewBuffer(sessionRequestBody))
			if err != nil {
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to create http request to proxy the call to the device respective provider Appium session endpoint", "", err.Error()))
				devicesMapMu.Lock()
				foundDevice.IsPreparingAutomation = false
				devicesMapMu.Unlock()
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
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to failed to execute the proxy request to the device respective provider Appium session endpoint", "", err.Error()))
				devicesMapMu.Lock()
				foundDevice.IsPreparingAutomation = false
				devicesMapMu.Unlock()
				return
			}
			defer resp.Body.Close()

			// Read the response sessionRequestBody from the proxied request
			proxiedSessionResponseBody, err := readBody(resp.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to read the response sessionRequestBody of the proxied Appium session request", "", err.Error()))
				devicesMapMu.Lock()
				foundDevice.IsPreparingAutomation = false
				devicesMapMu.Unlock()
				return
			}

			// Unmarshal the response sessionRequestBody to AppiumSessionResponse
			var proxySessionResponse AppiumSessionResponse
			err = json.Unmarshal(proxiedSessionResponseBody, &proxySessionResponse)
			if err != nil {
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to unmarshal the response sessionRequestBody of the proxied Appium session request", "", err.Error()))
				devicesMapMu.Lock()
				foundDevice.IsPreparingAutomation = false
				devicesMapMu.Unlock()
				return
			}

			sessionMapMu.Lock()
			foundDevice.SessionID = proxySessionResponse.Value.SessionID
			sessionMapMu.Unlock()

			// Copy the response back to the original client
			for k, v := range resp.Header {
				c.Writer.Header()[k] = v
			}
			c.Writer.WriteHeader(resp.StatusCode)
			c.Writer.Write(proxiedSessionResponseBody)
			foundDevice.IsRunningAutomation = true
			foundDevice.IsPreparingAutomation = false
			foundDevice.LastAutomationActionTS = time.Now().UnixMilli()
			foundDevice.InUseBy = "automation"
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
			sessionMapMu.Lock()
			foundDevice, err := getDeviceBySessionID(sessionID)
			sessionMapMu.Unlock()
			if err != nil {
				c.JSON(http.StatusNotFound, createErrorResponse(fmt.Sprintf("No session ID `%s` is available to GADS, it timed out or something unexpected occurred", sessionID), "", ""))
				return
			}

			// Set the device last automation action timestamp when call returns
			defer func() {
				foundDevice.LastAutomationActionTS = time.Now().UnixMilli()
			}()

			// Create a new request to the device target URL on its provider instance
			proxyReq, err := http.NewRequest(c.Request.Method, fmt.Sprintf("http://%s/device/%s/appium%s", foundDevice.Device.Host, foundDevice.Device.UDID, strings.Replace(c.Request.URL.Path, "/grid", "", -1)), bytes.NewBuffer(origRequestBody))
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
				sessionMapMu.Lock()
				foundDevice.SessionID = ""
				foundDevice.IsPreparingAutomation = false
				foundDevice.IsRunningAutomation = false
				foundDevice.InUseBy = ""
				sessionMapMu.Unlock()
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
			foundDevice.LastAutomationActionTS = time.Now().UnixMilli()
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
	for _, localDevice := range devices.HubDevicesMap {
		if localDevice.SessionID == sessionID {
			return localDevice, nil
		}
	}
	return nil, fmt.Errorf("No device with session ID `%s` was found in the local devices map", sessionID)
}

func getDeviceByUDID(udid string) (*models.LocalHubDevice, error) {
	devicesMapMu.Lock()
	defer devicesMapMu.Unlock()
	for _, localDevice := range devices.HubDevicesMap {
		if strings.EqualFold(localDevice.Device.UDID, udid) {
			return localDevice, nil
		}
	}
	return nil, fmt.Errorf("No device with udid `%s` was found in the local devices map", udid)
}

func findAvailableDevice(appiumSessionBody AppiumSession) (*models.LocalHubDevice, error) {
	devicesMapMu.Lock()
	defer devicesMapMu.Unlock()

	var foundDevice *models.LocalHubDevice

	var deviceUDID = ""
	if appiumSessionBody.Capabilities.FirstMatch[0].DeviceUDID != "" {
		deviceUDID = appiumSessionBody.Capabilities.FirstMatch[0].DeviceUDID
	}
	if appiumSessionBody.DesiredCapabilities.DeviceUDID != "" {
		deviceUDID = appiumSessionBody.DesiredCapabilities.DeviceUDID
	}

	if deviceUDID != "" {
		foundDevice, _ := getDeviceByUDID(deviceUDID)
		foundDevice.IsPreparingAutomation = true
		return foundDevice, nil
	} else {
		var availableDevices []*models.LocalHubDevice

		if strings.EqualFold(appiumSessionBody.Capabilities.FirstMatch[0].PlatformName, "iOS") ||
			strings.EqualFold(appiumSessionBody.DesiredCapabilities.PlatformName, "iOS") ||
			strings.EqualFold(appiumSessionBody.Capabilities.FirstMatch[0].AutomationName, "XCUITest") ||
			strings.EqualFold(appiumSessionBody.DesiredCapabilities.AutomationName, "XCUITest") {

			// Loop through all latest devices looking for an iOS device that is not currently `being prepared` for automation and the last time it was updated from provider was less than 3 seconds ago
			for _, localDevice := range devices.HubDevicesMap {
				if strings.EqualFold(localDevice.Device.OS, "ios") && !localDevice.IsPreparingAutomation && !localDevice.IsRunningAutomation && localDevice.Device.LastUpdatedTimestamp >= (time.Now().UnixMilli()-3000) {
					availableDevices = append(availableDevices, localDevice)
				}
			}
		} else if strings.EqualFold(appiumSessionBody.Capabilities.FirstMatch[0].PlatformName, "Android") ||
			strings.EqualFold(appiumSessionBody.DesiredCapabilities.PlatformName, "Android") ||
			strings.EqualFold(appiumSessionBody.Capabilities.FirstMatch[0].AutomationName, "UiAutomator2") ||
			strings.EqualFold(appiumSessionBody.DesiredCapabilities.AutomationName, "UiAutomator2") {

			// Loop through all latest devices looking for an Android device that is not currently `being prepared` for automation and the last time it was updated from provider was less than 3 seconds ago
			for _, localDevice := range devices.HubDevicesMap {
				if strings.EqualFold(localDevice.Device.OS, "android") && !localDevice.IsPreparingAutomation && !localDevice.IsRunningAutomation && localDevice.Device.LastUpdatedTimestamp >= (time.Now().UnixMilli()-3000) {
					availableDevices = append(availableDevices, localDevice)
				}
			}
		}

		// If we have `appium:platformVersion` capability provided, then we want to filter out the devices even more
		// Loop through the accumulated available devices slice and get a device that matches the platform version
		if appiumSessionBody.Capabilities.FirstMatch[0].PlatformVersion != "" {
			// First check if device completely matches the required version
			if len(availableDevices) != 0 {
				for _, device := range availableDevices {
					if device.Device.OSVersion == appiumSessionBody.Capabilities.FirstMatch[0].PlatformVersion {
						foundDevice = device
						foundDevice.IsPreparingAutomation = true
						break
					}
				}
			}
			// If no device completely matches the required version try a major version
			if foundDevice == nil {
				v, _ := semver.NewVersion(appiumSessionBody.Capabilities.FirstMatch[0].PlatformVersion)
				requestedMajorVersion := fmt.Sprintf("%d", v.Major())
				// Create a constraint for the requested version
				constraint, _ := semver.NewConstraint(fmt.Sprintf("^%s.0.0", requestedMajorVersion))

				if len(availableDevices) != 0 {
					for _, device := range availableDevices {
						deviceV, _ := semver.NewVersion(device.Device.OSVersion)
						if constraint.Check(deviceV) {
							foundDevice = device
							foundDevice.IsPreparingAutomation = true
							break
						}
					}
				}
			}
		} else {
			// If no platform version capability is provided, get the first device from the available list
			if len(availableDevices) != 0 {
				foundDevice = availableDevices[0]
				foundDevice.IsPreparingAutomation = true
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
