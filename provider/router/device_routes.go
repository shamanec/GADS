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
	"GADS/device/manager"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"GADS/common/api"
	"GADS/common/models"
	"GADS/common/utils"
	"GADS/device"
	"GADS/provider/logger"

	"github.com/gin-gonic/gin"
)

var netClient = &http.Client{Timeout: 120 * 1e9} // 120-second timeout

// copyHeaders copies all headers from source to destination.
func copyHeaders(destination, source http.Header) {
	for name, values := range source {
		for _, v := range values {
			destination.Add(name, v)
		}
	}
}

// DeviceHealth returns whether the device is currently connected.
func DeviceHealth(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := manager.Instance.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device `%s` not found", udid), nil)
		return
	}
	if dev.Info().Connected {
		api.GenericResponse(c, http.StatusOK, "Device is healthy", nil)
		return
	}
	api.GenericResponse(c, http.StatusInternalServerError, "Device is not healthy", nil)
}

// DeviceHome navigates the device to the home screen.
func DeviceHome(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := manager.Instance.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device `%s` not found", udid), nil)
		return
	}
	ctrl, ok := dev.(device.Controllable)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, "Device does not support remote control", nil)
		return
	}
	if err := ctrl.Home(); err != nil {
		logger.ProviderLogger.LogError("device_control", fmt.Sprintf("Failed to navigate home for %s: %v", udid, err))
		api.GenericResponse(c, http.StatusInternalServerError, "Failed to navigate to Home", nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, "Navigated to Home", nil)
}

// DeviceGetClipboard returns the current clipboard contents of the device.
func DeviceGetClipboard(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := manager.Instance.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device `%s` not found", udid), nil)
		return
	}
	ctrl, ok := dev.(device.Controllable)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, "Device does not support remote control", nil)
		return
	}
	text, err := ctrl.GetClipboard()
	if err != nil {
		logger.ProviderLogger.LogError("device_control", fmt.Sprintf("Failed to get clipboard for %s: %v", udid, err))
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to get clipboard: %v", err), nil)
		return
	}
	// The old implementation base64-decoded a WDA response; the new Controllable
	// returns the plaintext value directly. Decode if it looks base64-encoded
	// (iOS GetClipboard may return base64 from WDA).
	if decoded, err := base64.StdEncoding.DecodeString(text); err == nil && len(decoded) > 0 {
		api.GenericResponse(c, http.StatusOK, string(decoded), nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, text, nil)
}

// DeviceLock locks the device screen.
func DeviceLock(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := manager.Instance.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device `%s` not found", udid), nil)
		return
	}
	ctrl, ok := dev.(device.Controllable)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, "Device does not support remote control", nil)
		return
	}
	if err := ctrl.Lock(); err != nil {
		logger.ProviderLogger.LogError("device_control", fmt.Sprintf("Failed to lock %s: %v", udid, err))
		api.GenericResponse(c, http.StatusInternalServerError, "Failed to lock device", nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, "Device locked", nil)
}

// DeviceUnlock unlocks the device screen.
func DeviceUnlock(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := manager.Instance.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device `%s` not found", udid), nil)
		return
	}
	ctrl, ok := dev.(device.Controllable)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, "Device does not support remote control", nil)
		return
	}
	if err := ctrl.Unlock(); err != nil {
		logger.ProviderLogger.LogError("device_control", fmt.Sprintf("Failed to unlock %s: %v", udid, err))
		api.GenericResponse(c, http.StatusInternalServerError, "Failed to unlock device", nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, "Device unlocked", nil)
}

// DeviceScreenshot captures the device screen and returns a base64-encoded PNG.
func DeviceScreenshot(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := manager.Instance.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device `%s` not found", udid), nil)
		return
	}
	ctrl, ok := dev.(device.Controllable)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, "Device does not support remote control", nil)
		return
	}
	imgBytes, err := ctrl.Screenshot()
	if err != nil {
		logger.ProviderLogger.LogError("device_control", fmt.Sprintf("Failed to get screenshot for %s: %v", udid, err))
		api.GenericResponse(c, http.StatusInternalServerError, "Failed to get screenshot", nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, base64.StdEncoding.EncodeToString(imgBytes), nil)
}

// DeviceAppiumSource returns the Appium page source for the device.
func DeviceAppiumSource(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := manager.Instance.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device `%s` not found", udid), nil)
		return
	}
	sourceResp, err := appiumSource(dev.Info())
	if err != nil {
		logger.ProviderLogger.LogError("device_control", fmt.Sprintf("Failed to get Appium source for %s: %v", udid, err))
		api.GenericResponse(c, http.StatusInternalServerError, "Failed to get Appium source", nil)
		return
	}
	defer sourceResp.Body.Close()
	body, err := io.ReadAll(sourceResp.Body)
	if err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, "Failed to read Appium source response", nil)
		return
	}
	api.GenericResponse(c, sourceResp.StatusCode, string(body), nil)
}

// DeviceTypeText types text on the device using the platform-specific input method.
func DeviceTypeText(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := manager.Instance.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device `%s` not found", udid), nil)
		return
	}
	ctrl, ok := dev.(device.Controllable)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, "Device does not support remote control", nil)
		return
	}

	var requestBody models.ActionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	if err := ctrl.TypeText(requestBody.TextToType); err != nil {
		logger.ProviderLogger.LogError("device_control", fmt.Sprintf("Failed to type text on %s: %v", udid, err))
		api.GenericResponse(c, http.StatusInternalServerError, "Failed to type text", nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, "Text typed", nil)
}

// DeviceTap performs a tap at the given coordinates.
func DeviceTap(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := manager.Instance.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device `%s` not found", udid), nil)
		return
	}
	ctrl, ok := dev.(device.Controllable)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, "Device does not support remote control", nil)
		return
	}

	var requestBody models.ActionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	if err := ctrl.Tap(requestBody.X, requestBody.Y); err != nil {
		logger.ProviderLogger.LogError("device_control", fmt.Sprintf("Failed to tap on %s: %v", udid, err))
		api.GenericResponse(c, http.StatusInternalServerError, "Failed to tap", nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, "Tap performed", nil)
}

// DeviceTouchAndHold performs a long-press at the given coordinates.
func DeviceTouchAndHold(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := manager.Instance.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device `%s` not found", udid), nil)
		return
	}
	ctrl, ok := dev.(device.Controllable)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, "Device does not support remote control", nil)
		return
	}

	var requestBody models.ActionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	if err := ctrl.TouchAndHold(requestBody.X, requestBody.Y, requestBody.Duration); err != nil {
		logger.ProviderLogger.LogError("device_control", fmt.Sprintf("Failed to touch-and-hold on %s: %v", udid, err))
		api.GenericResponse(c, http.StatusInternalServerError, "Failed to touch and hold", nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, "Touch and hold performed", nil)
}

// DeviceSwipe performs a swipe gesture.
func DeviceSwipe(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := manager.Instance.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device `%s` not found", udid), nil)
		return
	}
	ctrl, ok := dev.(device.Controllable)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, "Device does not support remote control", nil)
		return
	}

	var requestBody models.ActionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	if err := ctrl.Swipe(requestBody.X, requestBody.Y, requestBody.EndX, requestBody.EndY); err != nil {
		logger.ProviderLogger.LogError("device_control", fmt.Sprintf("Failed to swipe on %s: %v", udid, err))
		api.GenericResponse(c, http.StatusInternalServerError, "Failed to swipe", nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, "Swipe performed", nil)
}

// DeviceExecuteCustomAction dispatches a named automation action to the device.
func DeviceExecuteCustomAction(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := manager.Instance.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device `%s` not found", udid), nil)
		return
	}
	ctrl, ok := dev.(device.Controllable)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, "Device does not support remote control", nil)
		return
	}

	var requestBody models.ExecuteCustomActionRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		api.GenericResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if requestBody.ActionType == "" {
		api.GenericResponse(c, http.StatusBadRequest, "action_type is required", nil)
		return
	}

	if err := executeCustomAction(ctrl, requestBody.ActionType, requestBody.Parameters); err != nil {
		logger.ProviderLogger.LogError("device_control", fmt.Sprintf("Failed to execute custom action '%s' on %s: %v", requestBody.ActionType, udid, err))
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, "Action executed", nil)
}

// executeCustomAction dispatches a named action to the Controllable device.
func executeCustomAction(ctrl device.Controllable, actionType string, params map[string]any) error {
	if params == nil {
		params = make(map[string]any)
	}

	switch actionType {
	case "tap":
		x, y := utils.GetFloat(params, "x", 0), utils.GetFloat(params, "y", 0)
		return ctrl.Tap(x, y)
	case "double_tap":
		x, y := utils.GetFloat(params, "x", 0), utils.GetFloat(params, "y", 0)
		return ctrl.DoubleTap(x, y)
	case "swipe":
		return ctrl.Swipe(
			utils.GetFloat(params, "x", 0), utils.GetFloat(params, "y", 0),
			utils.GetFloat(params, "endX", 0), utils.GetFloat(params, "endY", 0),
		)
	case "touch_and_hold":
		duration := utils.GetFloat(params, "duration", 1000)
		return ctrl.TouchAndHold(utils.GetFloat(params, "x", 0), utils.GetFloat(params, "y", 0), duration)
	case "pinch":
		return ctrl.Pinch(utils.GetFloat(params, "x", 0), utils.GetFloat(params, "y", 0), utils.GetFloat(params, "scale", 1.0))
	case "type_text":
		return ctrl.TypeText(utils.GetString(params, "text", ""))
	case "home":
		return ctrl.Home()
	case "lock":
		return ctrl.Lock()
	case "unlock":
		return ctrl.Unlock()
	case "pinch_in":
		return ctrl.Pinch(utils.GetFloat(params, "x", 250), utils.GetFloat(params, "y", 500), 0.5)
	case "pinch_out":
		return ctrl.Pinch(utils.GetFloat(params, "x", 250), utils.GetFloat(params, "y", 500), 2.0)
	default:
		return fmt.Errorf("unsupported action type: %s", actionType)
	}
}
