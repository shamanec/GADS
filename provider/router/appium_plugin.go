package router

import (
	"GADS/common/api"
	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/devices"
	"GADS/provider/minio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// AppiumPluginLog The plugin sends all logs from the server so we can store them in Mongo without having to parse output from the exec command
func AppiumPluginLog(c *gin.Context) {
	udid := c.Param("udid")
	if _, ok := devices.DBDeviceMap[udid]; ok {
		// Read the log request body
		body, err := io.ReadAll(c.Request.Body)
		defer c.Request.Body.Close()
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to read log request body - %s", err), nil)
			return
		}

		// Unmarshal into a struct suitable to insert in Mongo
		var appiumPluginLog models.AppiumPluginLog
		err = json.Unmarshal(body, &appiumPluginLog)

		db.GlobalMongoStore.AddAppiumLog(udid, appiumPluginLog)
		api.GenericResponse(c, http.StatusOK, "Logged successfully", nil)
		return
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device with udid `%s` not found", udid), nil)
}

// AppiumPluginSessionLog The plugin sends session action logs so we can store them in Mongo for Appium execution reporting
func AppiumPluginSessionLog(c *gin.Context) {
	udid := c.Param("udid")
	if _, ok := devices.DBDeviceMap[udid]; ok {
		// Read the log request body
		body, err := io.ReadAll(c.Request.Body)
		defer c.Request.Body.Close()
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to read log request body - %s", err), nil)
			return
		}

		var appiumPluginSessionLog models.AppiumPluginSessionLog
		err = json.Unmarshal(body, &appiumPluginSessionLog)

		db.GlobalMongoStore.AddAppiumSessionLog(appiumPluginSessionLog.Tenant, appiumPluginSessionLog)
		api.GenericResponse(c, http.StatusOK, "Logged successfully", nil)
		return
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device with udid `%s` not found", udid), nil)
}

// AppiumPluginScreenshot The plugin sends a screenshot request for particular commands, provider gets a screenshot from device and stores it in Minio in the respective buildId/sessionId bucket
// to show in reports later
func AppiumPluginScreenshot(c *gin.Context) {
	udid := c.Param("udid")
	if device, ok := devices.DBDeviceMap[udid]; ok {
		if device.OS == "ios" {
			return
		}

		// Read the request body
		requestBody, err := io.ReadAll(c.Request.Body)
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, "Failed to read screenshot request body", nil)
			return
		}

		// Try to unmarshal the request body
		var screenshotReq models.AppiumPluginScreenshotRequest
		err = json.Unmarshal(requestBody, &screenshotReq)
		if err != nil {
			api.GenericResponse(c, http.StatusBadRequest, "Failed to unmarshal screenshot request body", nil)
			return
		}

		// Try to get a screenshot from the Android GADS server app
		screenshotResp, err := androidRemoteServerRequest(device, http.MethodGet, "screenshot", nil)
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, "Failed to take screenshot from Android server app", nil)
			return
		}

		// Read the screenshot response body
		bodyBytes, err := io.ReadAll(screenshotResp.Body)
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, "Failed to read screenshot response from Android server app", nil)
			return
		}

		// Try to unmarshal the screenshot response
		var screenshotResponse models.AppiumPluginScreenshotResponse
		err = json.Unmarshal(bodyBytes, &screenshotResponse)
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, "Failed to marshal screenshot response from Android server app", nil)
			return
		}

		// Store the screenshot in Minio
		filename := screenshotReq.SequenceNumber
		if screenshotReq.IsAfterCommand {
			filename += "_after"
		}
		filename += ".jpg"

		objectPath, err := minio.GlobalMinioClient.StoreScreenshot(screenshotReq.BuildID, screenshotReq.SessionID, filename, screenshotResponse.Screenshot)
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to store screenshot in Minio: %s", err), nil)
			return
		}

		api.GenericResponse(c, http.StatusOK, fmt.Sprintf("Screenshot stored successfully at %s", objectPath), nil)
		return
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device with udid `%s` not found", udid), nil)
}

// AppiumPluginRegister The plugin sends a notification request when the server is started
func AppiumPluginRegister(c *gin.Context) {
	udid := c.Param("udid")
	if dev, ok := devices.DBDeviceMap[udid]; ok {
		dev.AppiumLastPingTS = time.Now().UnixMilli()
		dev.IsAppiumUp = true
		api.GenericResponse(c, http.StatusOK, "Appium registered as up", nil)
		return
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device with udid `%s` not found", udid), nil)
}

// AppiumPluginAddSession The plugin sends a notification request when a new session is started
func AppiumPluginAddSession(c *gin.Context) {
	udid := c.Param("udid")
	if dev, ok := devices.DBDeviceMap[udid]; ok {
		sessionID := c.Param("session_id")
		dev.AppiumLastPingTS = time.Now().UnixMilli()
		dev.HasAppiumSession = true
		dev.AppiumSessionID = sessionID
		dev.IsAppiumUp = true
		api.GenericResponse(c, http.StatusOK, "Session added", nil)
		return
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device with udid `%s` not found", udid), nil)
}

// AppiumPluginRemoveSession The plugin sends a notification request when the session is deleted
func AppiumPluginRemoveSession(c *gin.Context) {
	udid := c.Param("udid")
	if dev, ok := devices.DBDeviceMap[udid]; ok {
		dev.AppiumLastPingTS = time.Now().UnixMilli()
		dev.HasAppiumSession = false
		dev.AppiumSessionID = ""
		dev.IsAppiumUp = true
		api.GenericResponse(c, http.StatusOK, "Session cleared", nil)
		return
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device with udid `%s` not found", udid), nil)
}

// AppiumPluginPing The plugin periodically sends pings so we can keep track if the server is up
func AppiumPluginPing(c *gin.Context) {
	udid := c.Param("udid")
	if dev, ok := devices.DBDeviceMap[udid]; ok {
		dev.AppiumLastPingTS = time.Now().UnixMilli()
		dev.IsAppiumUp = true
		api.GenericResponse(c, http.StatusOK, "Ping for Appium server availability successful", nil)
		return
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device with udid `%s` not found", udid), nil)
}
