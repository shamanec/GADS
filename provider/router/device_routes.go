package router

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"GADS/provider/devices"
	"GADS/provider/models"
	"github.com/gin-gonic/gin"
)

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
	dev := devices.DeviceMap[udid]
	bool, err := devices.GetDeviceHealth(dev)
	if err != nil {
		dev.Logger.LogInfo("device", fmt.Sprintf("Could not check device health - %s", err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	if bool {
		dev.Logger.LogInfo("device", "Device is healthy")
		c.Writer.WriteHeader(200)
		return
	}

	dev.Logger.LogError("device", "Device is not healthy")
	c.Writer.WriteHeader(500)
}

// Call the respective Appium/WDA endpoint to go to Homescreen
func DeviceHome(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DeviceMap[udid]
	device.Logger.LogInfo("appium_interact", "Navigating to Home/Springboard")

	// Send the request
	homeResponse, err := appiumHome(device)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to navigate to Home/Springboard - %s", err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer homeResponse.Body.Close()

	// Read the response body
	homeResponseBody, err := io.ReadAll(homeResponse.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to navigate to Home/Springboard - %s", err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Writer.WriteHeader(homeResponse.StatusCode)
	copyHeaders(c.Writer.Header(), homeResponse.Header)
	fmt.Fprintf(c.Writer, string(homeResponseBody))
}

// Call respective Appium/WDA endpoint to lock the device
func DeviceLock(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DeviceMap[udid]
	device.Logger.LogInfo("appium_interact", "Locking device")

	lockResponse, err := appiumLockUnlock(device, "lock")
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to lock device - %s", err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer lockResponse.Body.Close()

	// Read the response body
	lockResponseBody, err := io.ReadAll(lockResponse.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to lock device - %s", err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Writer.WriteHeader(lockResponse.StatusCode)
	copyHeaders(c.Writer.Header(), lockResponse.Header)
	fmt.Fprintf(c.Writer, string(lockResponseBody))
}

// Call the respective Appium/WDA endpoint to unlock the device
func DeviceUnlock(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DeviceMap[udid]
	device.Logger.LogInfo("appium_interact", "Unlocking device")

	lockResponse, err := appiumLockUnlock(device, "unlock")
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to unlock device - %s", err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer lockResponse.Body.Close()

	// Read the response body
	lockResponseBody, err := io.ReadAll(lockResponse.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to unlock device - %s", err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Writer.WriteHeader(lockResponse.StatusCode)
	copyHeaders(c.Writer.Header(), lockResponse.Header)
	fmt.Fprintf(c.Writer, string(lockResponseBody))
}

// Call the respective Appium/WDA endpoint to take a screenshot of the device screen
func DeviceScreenshot(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DeviceMap[udid]
	device.Logger.LogInfo("appium_interact", "Getting screenshot from device")

	screenshotResp, err := appiumScreenshot(device)
	defer screenshotResp.Body.Close()

	// Read the response body
	screenshotRespBody, err := io.ReadAll(screenshotResp.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to get screenshot from device - %s", err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Writer.WriteHeader(screenshotResp.StatusCode)
	copyHeaders(c.Writer.Header(), screenshotResp.Header)
	fmt.Fprintf(c.Writer, string(screenshotRespBody))
}

//======================================
// Appium source

func DeviceAppiumSource(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DeviceMap[udid]
	device.Logger.LogInfo("appium_interact", "Getting Appium source from device")

	sourceResp, err := appiumSource(device)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to get Appium source from device - %s", err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Read the response body
	body, err := io.ReadAll(sourceResp.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to get Appium source from device - %s", err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer sourceResp.Body.Close()

	c.Writer.WriteHeader(sourceResp.StatusCode)
	copyHeaders(c.Writer.Header(), sourceResp.Header)
	fmt.Fprint(c.Writer, string(body))
}

//=======================================
// ACTIONS

func DeviceTypeText(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DeviceMap[udid]

	var requestBody models.ActionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to type text to active element - %s", err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	device.Logger.LogInfo("appium_interact", fmt.Sprintf("Typing `%s` to active element", requestBody.TextToType))

	typeResp, err := appiumTypeText(device, requestBody.TextToType)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to type `%s` to active element - %s", requestBody.TextToType, err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	body, err := io.ReadAll(typeResp.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to type `%s` to active element - %s", requestBody.TextToType, err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer typeResp.Body.Close()

	c.Writer.WriteHeader(typeResp.StatusCode)
	copyHeaders(c.Writer.Header(), typeResp.Header)
	fmt.Fprintf(c.Writer, string(body))
}

func DeviceClearText(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DeviceMap[udid]
	device.Logger.LogInfo("appium_interact", "Clearing text from active element")

	clearResp, err := appiumClearText(device)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Could not clear text from active element - %s", err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	body, err := io.ReadAll(clearResp.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Could not clear text from active element - %s", err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer clearResp.Body.Close()

	c.Writer.WriteHeader(clearResp.StatusCode)
	copyHeaders(c.Writer.Header(), clearResp.Header)
	fmt.Fprintf(c.Writer, string(body))
}

func DeviceTap(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DeviceMap[udid]

	var requestBody models.ActionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	device.Logger.LogInfo("appium_interact", fmt.Sprintf("Tapping at coordinates X:%v Y:%v", fmt.Sprintf("%.2f", requestBody.X), fmt.Sprintf("%.2f", requestBody.Y)))

	tapResp, err := appiumTap(device, requestBody.X, requestBody.Y)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to tap at coordinates X:%v Y:%v - %s", fmt.Sprintf("%.2f", requestBody.X), fmt.Sprintf("%.2f", requestBody.Y), err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer tapResp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(tapResp.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to tap at coordinates X:%v Y:%v` - %s", fmt.Sprintf("%.2f", requestBody.X), fmt.Sprintf("%.2f", requestBody.Y), err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Writer.WriteHeader(tapResp.StatusCode)
	copyHeaders(c.Writer.Header(), tapResp.Header)
	fmt.Fprint(c.Writer, string(body))
}

func DeviceTouchAndHold(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DeviceMap[udid]

	var requestBody models.ActionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	device.Logger.LogInfo("appium_interact", fmt.Sprintf("Touch and hold at coordinates X:%v Y:%v", fmt.Sprintf("%.2f", requestBody.X), fmt.Sprintf("%.2f", requestBody.Y)))

	touchAndHoldResp, err := appiumTouchAndHold(device, requestBody.X, requestBody.Y)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to touch and hold at coordinates X:%v Y:%v - %s", fmt.Sprintf("%.2f", requestBody.X), fmt.Sprintf("%.2f", requestBody.Y), err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer touchAndHoldResp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(touchAndHoldResp.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to touch and hold at coordinates X:%v Y:%v` - %s", fmt.Sprintf("%.2f", requestBody.X), fmt.Sprintf("%.2f", requestBody.Y), err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Writer.WriteHeader(touchAndHoldResp.StatusCode)
	copyHeaders(c.Writer.Header(), touchAndHoldResp.Header)
	fmt.Fprint(c.Writer, string(body))
}

func DeviceSwipe(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DeviceMap[udid]

	var requestBody models.ActionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to decode request body when performing swipe - %s", err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	device.Logger.LogInfo("appium_interact", fmt.Sprintf("Swiping from X:%v Y:%v to X:%v Y:%v", fmt.Sprintf("%.3f", requestBody.X), fmt.Sprintf("%.3f", requestBody.Y), fmt.Sprintf("%.3f", requestBody.EndX), fmt.Sprintf("%.3f", requestBody.EndY)))

	swipeResp, err := appiumSwipe(device, requestBody.X, requestBody.Y, requestBody.EndX, requestBody.EndY)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to swipe from X:%v Y:%v to X:%v Y:%v - %s", fmt.Sprintf("%.3f", requestBody.X), fmt.Sprintf("%.3f", requestBody.Y), fmt.Sprintf("%.3f", requestBody.EndX), fmt.Sprintf("%.3f", requestBody.EndY), err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer swipeResp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(swipeResp.Body)
	if err != nil {
		device.Logger.LogError("appium_interact", fmt.Sprintf("Failed to swipe from X:%v Y:%v to X:%v Y:%v - %s", fmt.Sprintf("%.3f", requestBody.X), fmt.Sprintf("%.3f", requestBody.Y), fmt.Sprintf("%.3f", requestBody.EndX), fmt.Sprintf("%.3f", requestBody.EndY), err))
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Writer.WriteHeader(swipeResp.StatusCode)
	copyHeaders(c.Writer.Header(), swipeResp.Header)
	fmt.Fprint(c.Writer, string(body))
}
