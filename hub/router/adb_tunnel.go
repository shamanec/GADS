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
	"GADS/hub/auth"
	"GADS/hub/devices"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
)

func ADBTunnelHandler(c *gin.Context) {
	udid := c.Param("udid")

	var username string
	if claims, err := auth.GetClaimsFromRequest(c); err == nil {
		username = claims.Username
	}

	device, ok := devices.HubDeviceStore.Get(udid)
	if !ok || device == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Device with UDID `%s` not found", udid)})
		return
	}

	device.Mu.RLock()

	if device.Device.OS != "android" {
		device.Mu.RUnlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "ADB tunnel is only available for Android devices"})
		return
	}

	// ADB tunnel is only allowed when the device is actively used from remote control by this user
	if !device.HasUISession() || device.InUseBy != username {
		device.Mu.RUnlock()
		c.JSON(http.StatusConflict, gin.H{"error": "ADB tunnel requires an active remote control session on this device"})
		return
	}
	host := device.Host
	device.Mu.RUnlock()

	// Connect to provider's ADB tunnel WebSocket
	providerURL := url.URL{
		Scheme: "ws",
		Host:   host,
		Path:   fmt.Sprintf("/device/%s/adb-tunnel", udid),
	}

	providerConn, _, _, err := ws.DefaultDialer.Dial(context.Background(), providerURL.String())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Failed to connect to provider ADB tunnel - %s", err)})
		return
	}

	// Upgrade client connection to WebSocket
	clientConn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		providerConn.Close()
		return
	}

	defer func() {
		clientConn.Close()
		providerConn.Close()
	}()

	// Monitor: close the tunnel if the remote control session ends
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				device.Mu.RLock()
				active := device.HasUISession() && device.InUseBy == username
				device.Mu.RUnlock()
				if !active {
					// Remote control session ended, close the tunnel
					clientConn.Close()
					providerConn.Close()
					return
				}
			}
		}
	}()

	// Bidirectional relay: client WebSocket <-> provider WebSocket
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(providerConn, clientConn)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(clientConn, providerConn)
		done <- struct{}{}
	}()

	<-done
	clientConn.Close()
	providerConn.Close()
	<-done
}
