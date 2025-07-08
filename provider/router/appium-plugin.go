package router

import (
	"GADS/common/api"
	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/devices"
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
		api.GenericResponse(c, http.StatusOK, "Ping for Appium server availability successful", nil)
		return
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device with udid `%s` not found", udid), nil)
}
