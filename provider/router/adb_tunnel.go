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
	"GADS/provider/devices"
	"GADS/provider/logger"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
)

func ADBTunnelProxy(c *gin.Context) {
	udid := c.Param("udid")
	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		logger.ProviderLogger.LogError("ADBTunnelProxy", fmt.Sprintf("Device with UDID `%s` not found", udid))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	androidDev, ok := platDev.(*devices.AndroidDevice)
	if !ok {
		logger.ProviderLogger.LogError("ADBTunnelProxy", fmt.Sprintf("Device `%s` is not an Android device", udid))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	adbPort := androidDev.GetADBPort()
	if adbPort == "" {
		logger.ProviderLogger.LogError("ADBTunnelProxy", fmt.Sprintf("ADB port not allocated for device `%s`", udid))
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	// Connect to the local ADB forwarded port
	adbConn, err := net.Dial("tcp", "localhost:"+adbPort)
	if err != nil {
		logger.ProviderLogger.LogError("ADBTunnelProxy", fmt.Sprintf("Failed to connect to ADB port for device `%s` - %s", udid, err))
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	// Upgrade the HTTP connection to WebSocket
	wsConn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		logger.ProviderLogger.LogError("ADBTunnelProxy", fmt.Sprintf("Failed to upgrade to WebSocket for device `%s` - %s", udid, err))
		adbConn.Close()
		return
	}

	logger.ProviderLogger.LogInfo("ADBTunnelProxy", fmt.Sprintf("ADB tunnel established for device `%s`", udid))

	// Bidirectional relay: WebSocket <-> ADB TCP
	done := make(chan struct{})

	go func() {
		io.Copy(adbConn, wsConn)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(wsConn, adbConn)
		done <- struct{}{}
	}()

	// Wait for either direction to finish
	<-done
	wsConn.Close()
	adbConn.Close()
	// Wait for the other goroutine to finish
	<-done

	logger.ProviderLogger.LogInfo("ADBTunnelProxy", fmt.Sprintf("ADB tunnel closed for device `%s`", udid))
}
