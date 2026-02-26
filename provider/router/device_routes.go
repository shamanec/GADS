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
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"GADS/common/api"
	"GADS/common/models"
	"GADS/provider/devices"

	"github.com/gin-gonic/gin"
)

var netClient = &http.Client{
	Timeout: time.Second * 120,
}

// Copy the headers from the original endpoint to the proxied endpoint
func copyHeaders(destination, source http.Header) {
	for name, values := range source {
		for _, v := range values {
			destination.Add(name, v)
		}
	}
}

// Check the device health by checking Appium and WDA(for iOS)
func DeviceHealth(c *gin.Context) {
	udid := c.Param("udid")
	dev := devices.DBDeviceMap[udid]
	bool, err := devices.GetDeviceHealth(dev)
	if err != nil {
		dev.Logger.LogInfo("device", fmt.Sprintf("Could not check device health - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	if bool {
		dev.Logger.LogInfo("device", "Device is healthy")
		api.GenericResponse(c, http.StatusOK, "Device is healthy", nil)
		return
	}

	dev.Logger.LogError("device", "Device is not healthy")
	api.GenericResponse(c, http.StatusInternalServerError, "Device is not healthy", nil)
}

// Call the respective Appium/WDA endpoint to go to Homescreen
func DeviceHome(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]
	device.Logger.LogInfo("appium_interact", "Navigating to Home/Springboard")

	// Send the request
	homeResponse, err := deviceHome(device)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to navigate to Home/Springboard - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, "Failed to navigate to Home/Springboard", nil)
		return
	}
	defer homeResponse.Body.Close()

	// Read the response body
	homeResponseBody, err := io.ReadAll(homeResponse.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to navigate to Home/Springboard - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, "Failed to navigate to Home/Springboard", nil)
		return
	}

	api.GenericResponse(c, homeResponse.StatusCode, string(homeResponseBody), nil)
}

func DeviceGetClipboard(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]
	device.Logger.LogInfo("appium_interact", "Getting device clipboard value")

	// Send the request
	clipboardResponse, err := deviceGetClipboard(device)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to get device clipboard value - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to get device clipboard value - %s", err), nil)
		return
	}
	defer clipboardResponse.Body.Close()

	// Read the response body
	clipboardResponseBody, err := io.ReadAll(clipboardResponse.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to read clipboard response body while getting clipboard value - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to read clipboard response body while getting clipboard value - %s", err), nil)
		return
	}

	// Unmarshal the response body to get the actual value returned
	valueResp := struct {
		Value string `json:"value"`
	}{}
	err = json.Unmarshal(clipboardResponseBody, &valueResp)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to unmarshal clipboard response body - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to unmarshal clipboard response body - %s", err), nil)
		return
	}

	// Decode the value because Appium returns it as base64 encoded string
	decoded, _ := base64.StdEncoding.DecodeString(valueResp.Value)
	api.GenericResponse(c, http.StatusOK, string(decoded), nil)
}

// Call respective Appium/WDA endpoint to lock the device
func DeviceLock(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]
	device.Logger.LogInfo("appium_interact", "Locking device")

	lockResponse, err := deviceLock(device, "lock")
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to lock device - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	defer lockResponse.Body.Close()

	// Read the response body
	lockResponseBody, err := io.ReadAll(lockResponse.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to lock device - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	api.GenericResponse(c, lockResponse.StatusCode, string(lockResponseBody), nil)
}

// Call the respective Appium/WDA endpoint to unlock the device
func DeviceUnlock(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]
	device.Logger.LogInfo("appium_interact", "Unlocking device")

	lockResponse, err := deviceLock(device, "unlock")
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to unlock device - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	defer lockResponse.Body.Close()

	// Read the response body
	lockResponseBody, err := io.ReadAll(lockResponse.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to unlock device - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	api.GenericResponse(c, lockResponse.StatusCode, string(lockResponseBody), nil)
}

// Call the respective Appium/WDA endpoint to take a screenshot of the device screen
func DeviceScreenshot(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]
	device.Logger.LogInfo("appium_interact", "Getting screenshot from device")

	screenshotResp, err := deviceScreenshot(device)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to get screenshot from device - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	api.GenericResponse(c, http.StatusOK, screenshotResp, nil)
}

//======================================
// Appium source

func DeviceAppiumSource(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]
	device.Logger.LogInfo("appium_interact", "Getting Appium source from device")

	sourceResp, err := appiumSource(device)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to get Appium source from device - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	// Read the response body
	body, err := io.ReadAll(sourceResp.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to get Appium source from device - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	defer sourceResp.Body.Close()

	api.GenericResponse(c, sourceResp.StatusCode, string(body), nil)
}

//=======================================
// ACTIONS

func DeviceTypeText(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]

	var requestBody models.ActionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to type text to active element - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	device.Logger.LogInfo("appium_interact", fmt.Sprintf("Typing `%s` to active element", requestBody.TextToType))
	typeTextPayload := models.AppiumTypeText{
		Text: requestBody.TextToType,
	}
	typeJSON, err := json.MarshalIndent(typeTextPayload, "", "  ")
	if err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	var typeResp *http.Response

	if device.OS == "ios" {
		typeResp, err = wdaRequest(device, http.MethodPost, "wda/type", bytes.NewBuffer(typeJSON))
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
			return
		}
	} else {
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%v/type", device.AndroidIMEPort), bytes.NewBuffer(typeJSON))
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		typeResp, err = netClient.Do(req)
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
			return
		}
	}

	var body []byte
	body, err = io.ReadAll(typeResp.Body)

	api.GenericResponse(c, typeResp.StatusCode, string(body), nil)
}

func DeviceTap(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]

	var requestBody models.ActionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	device.Logger.LogInfo("appium_interact", fmt.Sprintf("Tapping at coordinates X:%v Y:%v", fmt.Sprintf("%.2f", requestBody.X), fmt.Sprintf("%.2f", requestBody.Y)))

	tapResp, err := deviceTap(device, requestBody.X, requestBody.Y)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to tap at coordinates X:%v Y:%v - %s", fmt.Sprintf("%.2f", requestBody.X), fmt.Sprintf("%.2f", requestBody.Y), err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	defer tapResp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(tapResp.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to tap at coordinates X:%v Y:%v` - %s", fmt.Sprintf("%.2f", requestBody.X), fmt.Sprintf("%.2f", requestBody.Y), err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	api.GenericResponse(c, tapResp.StatusCode, string(body), nil)
}

func DeviceTouchAndHold(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]

	var requestBody models.ActionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	device.Logger.LogInfo("appium_interact", fmt.Sprintf("Touch and hold at coordinates X:%v Y:%v", fmt.Sprintf("%.2f", requestBody.X), fmt.Sprintf("%.2f", requestBody.Y)))

	touchAndHoldResp, err := deviceTouchAndHold(device, requestBody.X, requestBody.Y, requestBody.Duration)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to touch and hold at coordinates X:%v Y:%v - %s", fmt.Sprintf("%.2f", requestBody.X), fmt.Sprintf("%.2f", requestBody.Y), err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	defer touchAndHoldResp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(touchAndHoldResp.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to touch and hold at coordinates X:%v Y:%v` - %s", fmt.Sprintf("%.2f", requestBody.X), fmt.Sprintf("%.2f", requestBody.Y), err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	api.GenericResponse(c, touchAndHoldResp.StatusCode, string(body), nil)
}

func DeviceSwipe(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]

	var requestBody models.ActionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to decode request body when performing swipe - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	device.Logger.LogInfo("appium_interact", fmt.Sprintf("Swiping from X:%v Y:%v to X:%v Y:%v", fmt.Sprintf("%.3f", requestBody.X), fmt.Sprintf("%.3f", requestBody.Y), fmt.Sprintf("%.3f", requestBody.EndX), fmt.Sprintf("%.3f", requestBody.EndY)))

	swipeResp, err := deviceSwipe(device, requestBody.X, requestBody.Y, requestBody.EndX, requestBody.EndY)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to swipe from X:%v Y:%v to X:%v Y:%v - %s", fmt.Sprintf("%.3f", requestBody.X), fmt.Sprintf("%.3f", requestBody.Y), fmt.Sprintf("%.3f", requestBody.EndX), fmt.Sprintf("%.3f", requestBody.EndY), err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	defer swipeResp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(swipeResp.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to swipe from X:%v Y:%v to X:%v Y:%v - %s", fmt.Sprintf("%.3f", requestBody.X), fmt.Sprintf("%.3f", requestBody.Y), fmt.Sprintf("%.3f", requestBody.EndX), fmt.Sprintf("%.3f", requestBody.EndY), err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	api.GenericResponse(c, swipeResp.StatusCode, string(body), nil)
}

func DeviceExecuteCustomAction(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]

	var requestBody models.ExecuteCustomActionRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		device.Logger.LogError("device_control", fmt.Sprintf("Failed to decode request body when executing custom action - %s", err))
		api.GenericResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	if requestBody.ActionType == "" {
		device.Logger.LogError("device_control", "Missing action_type in request")
		api.GenericResponse(c, http.StatusBadRequest, "action_type is required", nil)
		return
	}

	device.Logger.LogInfo("device_control", fmt.Sprintf("Executing custom action '%s' with parameters: %+v", requestBody.ActionType, requestBody.Parameters))

	actionResp, err := executeCustomAction(device, requestBody.ActionType, requestBody.Parameters)
	if err != nil {
		device.Logger.LogError("device_control", fmt.Sprintf("Failed to execute custom action '%s' - %s", requestBody.ActionType, err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	defer actionResp.Body.Close()

	body, err := io.ReadAll(actionResp.Body)
	if err != nil {
		device.Logger.LogError("device_control", fmt.Sprintf("Failed to read response for custom action '%s' - %s", requestBody.ActionType, err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	api.GenericResponse(c, actionResp.StatusCode, string(body), nil)
}

// DeviceEnableAdbTcpIp enables ADB over TCP/IP (port 5555) on an Android device
// and returns the connection info needed to connect remotely.
func DeviceEnableAdbTcpIp(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]

	if device.OS != "android" {
		api.GenericResponse(c, http.StatusBadRequest, "ADB over TCP/IP is only supported for Android devices", nil)
		return
	}

	// Only operate on live devices — skip silently if device is still setting up
	if device.ProviderState != "live" {
		api.GenericResponse(c, http.StatusOK, "Device is not yet live, ADB TCP/IP enable skipped", nil)
		return
	}

	device.Logger.LogInfo("adb_tcpip", "Enabling ADB over TCP/IP")

	ip, err := devices.EnableAdbTcpIp(device)
	if err != nil {
		device.Logger.LogError("adb_tcpip", fmt.Sprintf("Failed to enable ADB over TCP/IP - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to enable ADB over TCP/IP - %s", err), nil)
		return
	}

	result := struct {
		IP             string `json:"ip"`
		Port           int    `json:"port"`
		ConnectCommand string `json:"connect_command"`
	}{
		IP:             ip,
		Port:           5555,
		ConnectCommand: fmt.Sprintf("adb connect %s:5555", ip),
	}

	api.GenericResponse(c, http.StatusOK, "", result)
}

// DeviceDisableAdbTcpIp reverts an Android device from ADB over TCP/IP mode back to USB mode.
func DeviceDisableAdbTcpIp(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]

	if device.OS != "android" {
		api.GenericResponse(c, http.StatusBadRequest, "ADB over TCP/IP is only supported for Android devices", nil)
		return
	}

	// Only operate on live devices — skip silently if device is still setting up.
	// The hub calls this on session end; if the provider just restarted and the device
	// is in setup, there is nothing to revert.
	if device.ProviderState != "live" {
		api.GenericResponse(c, http.StatusOK, "Device is not yet live, ADB TCP/IP disable skipped", nil)
		return
	}

	device.Logger.LogInfo("adb_tcpip", "Disabling ADB over TCP/IP")

	if err := devices.DisableAdbTcpIp(device); err != nil {
		device.Logger.LogError("adb_tcpip", fmt.Sprintf("Failed to disable ADB over TCP/IP - %s", err))
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to disable ADB over TCP/IP - %s", err), nil)
		return
	}

	api.GenericResponse(c, http.StatusOK, "ADB over TCP/IP disabled", nil)
}
