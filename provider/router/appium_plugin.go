package router

import (
	"GADS/common/api"
	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/devices"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"
)

// AppiumPluginLog The plugin sends all logs from the server so we can store them in Mongo without having to parse output from the exec command
func AppiumPluginLog(c *gin.Context) {
	udid := c.Param("udid")
	if _, ok := devices.DevManager.Get(udid); ok {
		// Read the log request body
		body, err := io.ReadAll(c.Request.Body)
		defer c.Request.Body.Close()
		if err != nil {
			api.InternalError(c, fmt.Sprintf("Failed to read log request body - %s", err))
			return
		}

		// Unmarshal into a struct suitable to insert in Mongo
		var appiumPluginLog models.AppiumPluginLog
		err = json.Unmarshal(body, &appiumPluginLog)

		db.GlobalMongoStore.AddAppiumLog(udid, appiumPluginLog)
		api.OKMessage(c, "Logged successfully")
		return
	}
	api.NotFound(c, fmt.Sprintf("Device with udid `%s` not found", udid))
}

// AppiumPluginRegister The plugin sends a notification request when the server is started
func AppiumPluginRegister(c *gin.Context) {
	udid := c.Param("udid")
	if dev, ok := devices.DevManager.Get(udid); ok {
		dev.SetAppiumLastPingTS(time.Now().UnixMilli())
		dev.SetAppiumUp(true)
		api.OKMessage(c, "Appium registered as up")
		return
	}
	api.NotFound(c, fmt.Sprintf("Device with udid `%s` not found", udid))
}

// AppiumPluginAddSession The plugin sends a notification request when a new session is started
func AppiumPluginAddSession(c *gin.Context) {
	udid := c.Param("udid")
	if dev, ok := devices.DevManager.Get(udid); ok {
		sessionID := c.Param("session_id")
		dev.SetAppiumLastPingTS(time.Now().UnixMilli())
		var sessionCaps map[string]interface{}
		// Newer plugins also send the resolved session capabilities in the body -
		// optional, older plugins post with no body at all
		body, err := io.ReadAll(c.Request.Body)
		defer c.Request.Body.Close()
		if err == nil && len(body) > 0 {
			_ = json.Unmarshal(body, &sessionCaps)
		}
		dev.UpdateAppiumSession(sessionID, sessionCaps)
		dev.SetAppiumUp(true)
		api.OKMessage(c, "Session added")
		return
	}
	api.NotFound(c, fmt.Sprintf("Device with udid `%s` not found", udid))
}

// AppiumPluginRemoveSession The plugin sends a notification request when the session is deleted
func AppiumPluginRemoveSession(c *gin.Context) {
	udid := c.Param("udid")
	if dev, ok := devices.DevManager.Get(udid); ok {
		dev.SetAppiumLastPingTS(time.Now().UnixMilli())
		var event struct {
			SessionID string `json:"session_id"`
		}
		if c.Request.Body != nil {
			defer c.Request.Body.Close()
			if err := json.NewDecoder(c.Request.Body).Decode(&event); err != nil && err != io.EOF {
				api.InternalError(c, "Invalid session cleanup notification")
				return
			}
		}
		cleared := dev.ClearAppiumSession(event.SessionID)
		if log := dev.GetLogger(); log != nil {
			log.LogDebugf("appium_lifecycle", "Cleanup notification udid=%s old_session=%s cleared=%t current_session=%s appium_port=%s", udid, event.SessionID, cleared, dev.GetAppiumSessionID(), dev.GetAppiumPort())
		}
		dev.SetAppiumUp(true)
		api.OKMessage(c, "Session cleared")
		return
	}
	api.NotFound(c, fmt.Sprintf("Device with udid `%s` not found", udid))
}

// AppiumPluginPing The plugin periodically sends pings so we can keep track if the server is up
func AppiumPluginPing(c *gin.Context) {
	udid := c.Param("udid")
	if dev, ok := devices.DevManager.Get(udid); ok {
		dev.SetAppiumLastPingTS(time.Now().UnixMilli())
		dev.SetAppiumUp(true)
		api.OKMessage(c, "Ping for Appium server availability successful")
		return
	}
	api.NotFound(c, fmt.Sprintf("Device with udid `%s` not found", udid))
}

// AppiumPluginCommand The plugin reports driver command activity (throttled on its
// side) so the provider knows a test is actively driving the device even when the
// commands do not pass through the hub proxy
func AppiumPluginCommand(c *gin.Context) {
	udid := c.Param("udid")
	if dev, ok := devices.DevManager.Get(udid); ok {
		now := time.Now().UnixMilli()
		dev.SetAppiumLastPingTS(now)
		dev.SetAppiumLastCommandTS(now)
		dev.SetAppiumUp(true)
		api.OKMessage(c, "Command activity recorded")
		return
	}
	api.NotFound(c, fmt.Sprintf("Device with udid `%s` not found", udid))
}
