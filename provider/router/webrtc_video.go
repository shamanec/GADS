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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type WebRTCMessage struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

func DevicesWebRTCSocket(c *gin.Context) {
	udid := c.Param("udid")

	device, ok := devices.DBDeviceMap[udid]
	if !ok || device == nil {
		logger.ProviderLogger.LogError("device_webrtc", fmt.Sprintf("Device with UDID `%s` not found or is nil", udid))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		logger.ProviderLogger.LogError("device_webrtc", fmt.Sprintf("Failed to upgrade connection to websocket for device `%s` - %s", udid, err))
		return
	}
	defer conn.Close()

	u := url.URL{Scheme: "ws", Host: "localhost:" + device.StreamPort, Path: ""}
	deviceConn, _, _, err := ws.DefaultDialer.Dial(context.Background(), u.String())
	if err != nil {
		logger.ProviderLogger.LogError("device_webrtc", fmt.Sprintf("Failed to dial stream websocket for device `%s` - %s", udid, err))
		return
	}
	defer deviceConn.Close()

	logger.ProviderLogger.LogInfo("device_webrtc", fmt.Sprintf("WebRTC connection established for device `%s`", udid))

	done := make(chan struct{})

	go func() {
		defer func() {
			select {
			case <-done:
			default:
				close(done)
			}
		}()

		for {
			select {
			case <-done:
				return
			default:
				msg, op, err := wsutil.ReadServerData(deviceConn)
				if err != nil {
					logger.ProviderLogger.LogDebug("device_webrtc", fmt.Sprintf("WebRTC websocket connection for device `%s` closed - %s", udid, err))
					return
				}

				logger.ProviderLogger.LogDebug("device_webrtc", fmt.Sprintf("Received WebRTC message from device `%s` - %s", udid, string(msg)))

				if op == ws.OpText {
					err = wsutil.WriteServerText(conn, msg)
					if err != nil {
						logger.ProviderLogger.LogError("device_webrtc", fmt.Sprintf("Failed to write WebRTC message from device `%s` to client - %s", udid, err))
						return
					}
				}
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		default:
			msg, op, err := wsutil.ReadClientData(conn)
			if err != nil {
				logger.ProviderLogger.LogDebug("device_webrtc", fmt.Sprintf("Client WebRTC websocket connection for device `%s` closed - %s", udid, err))
				wsutil.WriteServerText(deviceConn, []byte("hangup"))
				return
			}

			logger.ProviderLogger.LogDebug("device_webrtc", fmt.Sprintf("Received WebRTC message from client for device `%s` - %s", udid, string(msg)))

			if op == ws.OpText {
				var message WebRTCMessage
				err = json.Unmarshal(msg, &message)
				if err != nil {
					logger.ProviderLogger.LogError("device_webrtc", fmt.Sprintf("Failed to unmarshal WebRTC message for device `%s` - %s", udid, err))
					wsutil.WriteServerText(deviceConn, []byte("hangup"))
					return
				}

				switch message.Type {
				case "offer", "answer", "candidate":
					logger.ProviderLogger.LogDebug("device_webrtc", fmt.Sprintf("Processing WebRTC %s for device `%s`", message.Type, udid))
					err = wsutil.WriteServerText(deviceConn, msg)
					if err != nil {
						logger.ProviderLogger.LogError("device_webrtc", fmt.Sprintf("Failed to send WebRTC %s to device `%s` - %s", message.Type, udid, err))
						return
					}
				case "hangup":
					logger.ProviderLogger.LogInfo("device_webrtc", fmt.Sprintf("Received hangup for device `%s`", udid))
					wsutil.WriteServerText(deviceConn, msg)
					return
				default:
					err = wsutil.WriteServerText(deviceConn, msg)
					if err != nil {
						logger.ProviderLogger.LogError("device_webrtc", fmt.Sprintf("Failed to forward message to device `%s` - %s", udid, err))
						return
					}
				}
			}
		}
	}
}
