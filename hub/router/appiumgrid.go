package router

import (
	"GADS/common/models"
	"GADS/hub/devices"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Capabilities struct {
	FirstMatch  []interface{} `json:"firstMatch"`
	AlwaysMatch AlwaysMatch   `json:"alwaysMatch"`
}

type DesiredCapabilities struct {
	AutomationName  string `json:"appium:automationName"`
	BundleID        string `json:"appium:bundleId"`
	PlatformVersion string `json:"appium:platformVersion"`
	PlatformName    string `json:"platformName"`
	DeviceUDID      string `json:"appium:udid"`
}

type AlwaysMatch struct {
	AutomationName  string `json:"appium:automationName"`
	BundleID        string `json:"appium:bundleId"`
	PlatformVersion string `json:"appium:platformVersion"`
	PlatformName    string `json:"platformName"`
	DeviceUDID      string `json:"appium:udid"`
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
var devicesMap sync.Mutex
var localDevicesMap = make(map[string]*LocalAutoDevice)

type LocalAutoDevice struct {
	Device                *models.Device
	IsPreparingAutomation bool
	SessionID             string
}

type SeleniumSessionErrorResponse struct {
	Value SeleniumSessionErrorResponseValue `json:"value"`
}

type SeleniumSessionErrorResponseValue struct {
	Error      string `json:"error"`
	Message    string `json:"message"`
	StackTrace string `json:"stacktrace"`
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

			// Check for available device
			var foundDevice *LocalAutoDevice
			foundDevice, err = findAvailableDevice(appiumSessionBody)
			// If no device is available start checking each second for 60 seconds
			// If no device is available after 60 seconds - return error
			if foundDevice == nil {
				fmt.Println("Inside check")
				ticker := time.NewTicker(1 * time.Second)
				timeout := time.After(5 * time.Second)
				for {
					select {
					case <-ticker.C:
						foundDevice, err = findAvailableDevice(appiumSessionBody)
						fmt.Println("Inside ticker")
						fmt.Println(foundDevice)
						if foundDevice != nil {
							fmt.Println("Are we breaking")
							break
						}
					case <-timeout:
						fmt.Println("TIMEOUT")
						responseError := SeleniumSessionErrorResponse{
							Value: SeleniumSessionErrorResponseValue{
								Message: "No available device found",
							},
						}
						c.JSON(http.StatusInternalServerError, responseError)
						return
					}
				}
			}

			// When a device is finally chosen, set the flag that is being prepared for automation
			devicesMap.Lock()
			foundDevice.IsPreparingAutomation = true
			devicesMap.Unlock()

			// Create a new request to the device target URL on its provider instance
			proxyReq, err := http.NewRequest(c.Request.Method, fmt.Sprintf("http://%s/device/%s/appium%s", foundDevice.Device.Host, foundDevice.Device.UDID, strings.Replace(c.Request.URL.Path, "/grid", "", -1)), bytes.NewBuffer(sessionRequestBody))
			if err != nil {
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to create http request to proxy the call to the device respective provider Appium session endpoint", "", err.Error()))
				foundDevice.IsPreparingAutomation = false
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
				foundDevice.IsPreparingAutomation = false
				return
			}
			defer resp.Body.Close()

			// Read the response sessionRequestBody from the proxied request
			proxiedSessionResponseBody, err := readBody(resp.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to read the response sessionRequestBody of the proxied Appium session request", "", err.Error()))
				foundDevice.IsPreparingAutomation = false
				return
			}

			// Unmarshal the response sessionRequestBody to AppiumSessionResponse
			var proxySessionResponse AppiumSessionResponse
			err = json.Unmarshal(proxiedSessionResponseBody, &proxySessionResponse)
			if err != nil {
				c.JSON(http.StatusInternalServerError, createErrorResponse("GADS failed to unmarshal the response sessionRequestBody of the proxied Appium session request", "", err.Error()))
				foundDevice.IsPreparingAutomation = false
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
				c.JSON(http.StatusNotFound, createErrorResponse(fmt.Sprintf("No device with session ID `%s` is available to GADS", sessionID), "", ""))
				return
			}

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

func copyLatestDevicesToLocalMap() {
	for _, device := range devices.LatestDevices {
		mapDevice, ok := localDevicesMap[device.UDID]
		if !ok {
			localDevicesMap[device.UDID] = &LocalAutoDevice{
				Device:                device,
				IsPreparingAutomation: false,
			}
		} else {
			mapDevice.Device = device
		}
	}
}

func getDeviceBySessionID(sessionID string) (*LocalAutoDevice, error) {
	for _, localDevice := range localDevicesMap {
		if localDevice.SessionID == sessionID {
			return localDevice, nil
		}
	}
	return nil, fmt.Errorf("No device with session ID `%s` was found in the local devices map", sessionID)
}

func getDeviceByUDID(udid string) (*LocalAutoDevice, error) {
	for _, localDevice := range localDevicesMap {
		if localDevice.Device.UDID == udid {
			return localDevice, nil
		}
	}
	return nil, fmt.Errorf("No device with udid `%s` was found in the local devices map", udid)
}

func findAvailableDevice(appiumSessionBody AppiumSession) (*LocalAutoDevice, error) {
	var foundDevice *LocalAutoDevice

	if appiumSessionBody.Capabilities.AlwaysMatch.DeviceUDID != "" {
		return getDeviceByUDID(appiumSessionBody.Capabilities.AlwaysMatch.DeviceUDID)
	} else if strings.EqualFold(appiumSessionBody.Capabilities.AlwaysMatch.PlatformName, "iOS") || strings.EqualFold(appiumSessionBody.Capabilities.AlwaysMatch.AutomationName, "XCUITest") {
		var iosDevices []*LocalAutoDevice

		// Loop through all latest devices looking for an iOS device that is not currently `being prepared` for automation and the last time it was updated from provider was less than 3 seconds ago
		devicesMap.Lock()
		copyLatestDevicesToLocalMap()
		for _, localDevice := range localDevicesMap {
			if localDevice.Device.OS == "ios" && !localDevice.IsPreparingAutomation && localDevice.Device.LastUpdatedTimestamp >= (time.Now().UnixMilli()-3000) {
				iosDevices = append(iosDevices, localDevice)
			}
		}
		devicesMap.Unlock()

		// If we have `appium:platformVersion` capability provided, then we want to filter out the devices even more
		// Loop through the accumulated available devices slice and get a device that matches the platform version
		if appiumSessionBody.Capabilities.AlwaysMatch.PlatformVersion != "" {
			devicesMap.Lock()
			for _, device := range iosDevices {
				if device.Device.OSVersion == appiumSessionBody.Capabilities.AlwaysMatch.PlatformVersion {
					foundDevice = device
				}
			}
			devicesMap.Unlock()
		} else {
			// If no platform version capability is provided, get the first device from the available list
			devicesMap.Lock()
			foundDevice = iosDevices[0]
			devicesMap.Unlock()
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
