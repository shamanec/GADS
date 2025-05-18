package router

import (
	"GADS/provider/devices"
	"GADS/provider/logger"
	"context"
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

	u := url.URL{Scheme: "ws", Host: "localhost:" + device.StreamPort, Path: ""}
	deviceConn, _, _, err := ws.DefaultDialer.Dial(context.Background(), u.String())
	if err != nil {
		logger.ProviderLogger.LogError("device_webrtc", fmt.Sprintf("Failed to dial stream websocket for device `%s` - %s", udid, err))
		return
	}
	defer deviceConn.Close()

	go func() {
		for {
			msg, op, err := wsutil.ReadServerData(deviceConn)
			if err != nil {
				logger.ProviderLogger.LogError("device_webrtc", fmt.Sprintf("WebRTC websocket connection for device `%s` got disconnected.", udid))
				return
			}

			logger.ProviderLogger.LogDebug("device_webrtc", fmt.Sprintf("Received WebRTC message over socket from device `%s` - %s", udid, string(msg)))
			if op == ws.OpText {
				err = wsutil.WriteServerText(conn, msg)
				if err != nil {
					logger.ProviderLogger.LogError("device_webrtc", fmt.Sprintf("Failed to write WebRTC message from device `%s` to hub connection - %s", udid, err))
					return
				}
			}
		}
	}()

	for {
		msg, op, err := wsutil.ReadClientData(conn)
		if err != nil {
			logger.ProviderLogger.LogError("device_webrtc", fmt.Sprintf("Hub WebRTC websocket connection for device `%s` was lost - %s", udid, err))
			return
		}

		logger.ProviderLogger.LogDebug("device_webrtc", fmt.Sprintf("Received WebRTC message over socket from hub for device `%s` - %s", udid, string(msg)))
		if op == ws.OpText {
			err = wsutil.WriteClientText(deviceConn, msg)
			if err != nil {
				logger.ProviderLogger.LogError("device_webrtc", fmt.Sprintf("Failed to write WebRTC message to websocket connection of device `%s` - %s", udid, err))
			}
		}
	}
}
