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
	"GADS/common/api"
	"GADS/device/manager"
	"GADS/common/db"
	"GADS/common/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// AppiumPluginLog receives log entries from the GADS Appium plugin and stores
// them in MongoDB.
func AppiumPluginLog(c *gin.Context) {
	udid := c.Param("udid")
	if _, ok := manager.Instance.GetDevice(udid); ok {
		body, err := io.ReadAll(c.Request.Body)
		defer c.Request.Body.Close()
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to read log request body - %s", err), nil)
			return
		}

		var appiumPluginLog models.AppiumPluginLog
		json.Unmarshal(body, &appiumPluginLog)

		db.GlobalMongoStore.AddAppiumLog(udid, appiumPluginLog)
		api.GenericResponse(c, http.StatusOK, "Logged successfully", nil)
		return
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device with udid `%s` not found", udid), nil)
}

// AppiumPluginRegister is called by the GADS Appium plugin when the server starts.
func AppiumPluginRegister(c *gin.Context) {
	udid := c.Param("udid")
	if dev, ok := manager.Instance.GetDevice(udid); ok {
		dev.Info().AppiumLastPingTS = time.Now().UnixMilli()
		dev.Info().IsAppiumUp = true
		api.GenericResponse(c, http.StatusOK, "Appium registered as up", nil)
		return
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device with udid `%s` not found", udid), nil)
}

// AppiumPluginAddSession is called by the GADS Appium plugin when a new session starts.
func AppiumPluginAddSession(c *gin.Context) {
	udid := c.Param("udid")
	if dev, ok := manager.Instance.GetDevice(udid); ok {
		sessionID := c.Param("session_id")
		info := dev.Info()
		info.AppiumLastPingTS = time.Now().UnixMilli()
		info.HasAppiumSession = true
		info.AppiumSessionID = sessionID
		info.IsAppiumUp = true
		api.GenericResponse(c, http.StatusOK, "Session added", nil)
		return
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device with udid `%s` not found", udid), nil)
}

// AppiumPluginRemoveSession is called by the GADS Appium plugin when a session ends.
func AppiumPluginRemoveSession(c *gin.Context) {
	udid := c.Param("udid")
	if dev, ok := manager.Instance.GetDevice(udid); ok {
		info := dev.Info()
		info.AppiumLastPingTS = time.Now().UnixMilli()
		info.HasAppiumSession = false
		info.AppiumSessionID = ""
		info.IsAppiumUp = true
		api.GenericResponse(c, http.StatusOK, "Session cleared", nil)
		return
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device with udid `%s` not found", udid), nil)
}

// AppiumPluginPing is called periodically by the GADS Appium plugin to signal
// that the Appium server is still alive.
func AppiumPluginPing(c *gin.Context) {
	udid := c.Param("udid")
	if dev, ok := manager.Instance.GetDevice(udid); ok {
		dev.Info().AppiumLastPingTS = time.Now().UnixMilli()
		dev.Info().IsAppiumUp = true
		api.GenericResponse(c, http.StatusOK, "Ping for Appium server availability successful", nil)
		return
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Device with udid `%s` not found", udid), nil)
}
