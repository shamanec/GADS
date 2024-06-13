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
var localDeviceSessionMap = make(map[string]*models.Device)

func AppiumGridMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasSuffix(c.Request.URL.Path, "/session") {
			fmt.Println("Creating session")
			time.Sleep(1 * time.Second)
			var foundDevice *models.Device

			body, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(500, "Failed to read json body")
			}
			defer c.Request.Body.Close()

			var sessionBody AppiumSession
			err = json.Unmarshal(body, &sessionBody)
			if err != nil {
				c.String(http.StatusInternalServerError, "failed to unmarshal")
				return
			}

			if sessionBody.Capabilities.AlwaysMatch.DeviceUDID != "" {
				foundDevice = devices.GetDeviceByUDID(sessionBody.Capabilities.AlwaysMatch.DeviceUDID)
			} else if strings.EqualFold(sessionBody.Capabilities.AlwaysMatch.PlatformName, "iOS") || strings.EqualFold(sessionBody.Capabilities.AlwaysMatch.AutomationName, "XCUITest") {
				var iosDevices []*models.Device
				devicesMap.Lock()
				for _, device := range devices.LatestDevices {
					if device.OS == "ios" {
						fmt.Println("Device udid - " + device.UDID)
						fmt.Println(device.IsPreparingAutomation)
						fmt.Println(device.Available)
					}

					if device.OS == "ios" && !device.IsPreparingAutomation && device.LastUpdatedTimestamp >= (time.Now().UnixMilli()-3000) {
						fmt.Printf("Appending  %s", device)
						iosDevices = append(iosDevices, device)
					}
				}
				devicesMap.Unlock()
				if sessionBody.Capabilities.AlwaysMatch.PlatformVersion != "" {
					devicesMap.Lock()
					for _, device := range iosDevices {
						if device.OSVersion == sessionBody.Capabilities.AlwaysMatch.PlatformVersion {
							foundDevice = device
						}
					}
					devicesMap.Unlock()
				} else {
					devicesMap.Lock()
					for _, device := range iosDevices {
						if device.AppiumSessionID == "" {
							foundDevice = device
						}
					}
					devicesMap.Unlock()
				}
			}

			devicesMap.Lock()
			foundDevice.IsPreparingAutomation = true
			devicesMap.Unlock()

			// Create a new request to the target URL
			proxyReq, err := http.NewRequest(c.Request.Method, fmt.Sprintf("http://%s/device/%s/appium%s", foundDevice.Host, foundDevice.UDID, strings.Replace(c.Request.URL.Path, "/grid", "", -1)), bytes.NewBuffer(body))
			if err != nil {
				c.String(http.StatusInternalServerError, "proxy request create fail")
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
				c.String(http.StatusInternalServerError, "client do fail")
				return
			}
			defer resp.Body.Close()

			// Read the response body
			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				c.String(http.StatusInternalServerError, "read body fail")
				return
			}

			var sessionResponse AppiumSessionResponse
			err = json.Unmarshal(respBody, &sessionResponse)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed unmarshaling session response")
				return
			}
			sessionMapMu.Lock()
			localDeviceSessionMap[sessionResponse.Value.SessionID] = foundDevice
			sessionMapMu.Unlock()

			// Copy the response back to the original client
			for k, v := range resp.Header {
				c.Writer.Header()[k] = v
			}
			c.Writer.WriteHeader(resp.StatusCode)
			c.Writer.Write(respBody)
		} else {
			var sessionID = ""
			if strings.Contains(c.Request.URL.Path, "/session/") {
				var startIndex int
				var endIndex int

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
					fmt.Println("Do we have error here")
					c.JSON(http.StatusNotFound, gin.H{"error": "No session ID"})
					return
				}

				sessionID = c.Request.URL.Path[startIndex:endIndex]
			}

			if sessionID == "" {
				fmt.Println("In no session ID error - " + sessionID)
				c.JSON(http.StatusNotFound, gin.H{"error": "No session ID"})
				return
			}

			body, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(500, "Failed to read json body")
			}
			defer c.Request.Body.Close()

			sessionMapMu.Lock()
			foundDevice, ok := localDeviceSessionMap[sessionID]
			sessionMapMu.Unlock()
			if !ok {
				fmt.Println("Device not found in map error")
				fmt.Println(sessionID)
				fmt.Println(localDeviceSessionMap)
				c.JSON(http.StatusNotFound, gin.H{"error": "No session ID"})
				return
			}

			// Create a new request to the target URL
			proxyReq, err := http.NewRequest(c.Request.Method, fmt.Sprintf("http://%s/device/%s/appium%s", foundDevice.Host, foundDevice.UDID, strings.Replace(c.Request.URL.Path, "/grid", "", -1)), bytes.NewBuffer(body))
			if err != nil {
				c.String(http.StatusInternalServerError, "proxy request create fail")
				return
			}
			fmt.Printf("Calling on - %s\n", proxyReq.URL)

			// Copy headers
			for k, v := range c.Request.Header {
				proxyReq.Header[k] = v
			}

			// Send the request
			client := &http.Client{}
			resp, err := client.Do(proxyReq)
			if err != nil {
				c.String(http.StatusInternalServerError, "Client do")
				return
			}
			defer resp.Body.Close()

			defer func() {
				if c.Request.Method == http.MethodDelete {
					fmt.Println("Deleting session id from map - " + sessionID)
					sessionMapMu.Lock()
					fmt.Println(localDeviceSessionMap)
					delete(localDeviceSessionMap, sessionID)
					sessionMapMu.Unlock()
				}
			}()

			// Read the response body
			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				c.String(http.StatusInternalServerError, "response read all")
				return
			}

			var sessionResponse AppiumSessionResponse
			err = json.Unmarshal(respBody, &sessionResponse)

			devicesMap.Lock()
			foundDevice.AppiumSessionID = sessionResponse.Value.SessionID
			devicesMap.Unlock()

			// Copy the response back to the original client
			for k, v := range resp.Header {
				c.Writer.Header()[k] = v
			}
			c.Writer.WriteHeader(resp.StatusCode)
			c.Writer.Write(respBody)
		}
	}
}
