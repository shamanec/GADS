/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package android

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"GADS/common/auth"
	"GADS/device"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// streamServiceName returns the ADB service component name for the configured
// streaming mode. Used to check whether the service is already running and to
// stop it when needed.
func (d *AndroidDevice) streamServiceName() string {
	switch d.info.StreamType {
	case device.AndroidWebRTCGetStreamStreamTypeID:
		return "com.gads.settings/.WebRTCScreenCaptureService"
	default:
		// MJPEG and H264 both use the standard screen-capture service
		return "com.gads.settings/.ScreenCaptureService"
	}
}

// streamActivityName returns the ADB activity name used to start the
// GADS-Settings streaming activity for the configured streaming mode.
func (d *AndroidDevice) streamActivityName() string {
	switch d.info.StreamType {
	case device.AndroidWebRTCGetStreamStreamTypeID:
		return "com.gads.settings/com.gads.settings.webrtc.WebRTCScreenCaptureActivity"
	default:
		return "com.gads.settings/com.gads.settings.streaming.MjpegScreenCaptureActivity"
	}
}

// startStream launches the appropriate video streaming service based on the
// configured StreamType. For H264 WebRTC it starts GADS-Settings as an
// app_process; for MJPEG / GetStream it starts the foreground Activity. Each
// variant runs in a background goroutine that resets the device on exit.
func (d *AndroidDevice) startStream(ctx context.Context) error {
	if d.info.StreamType == device.AndroidWebRTCGadsH264StreamTypeID {
		// H264Server is a Kotlin app_process — kill any existing instance first.
		_, _ = d.cmd.Run(ctx, "adb", "-s", d.info.UDID, "shell", "pkill", "-f", "H264Server")
		time.Sleep(1 * time.Second)

		proc, err := d.cmd.Start(ctx, "adb", "-s", d.info.UDID, "shell",
			"CLASSPATH=/data/local/tmp/gads-settings app_process / com.gads.settings.server.H264Server")
		if err != nil {
			return fmt.Errorf("startStream %s: start H264Server: %w", d.info.UDID, err)
		}
		go func() {
			<-proc.Done
			d.Reset("GADS H264 server exited unexpectedly")
		}()
		return nil
	}

	// MJPEG and GetStream: add recording permissions and start the Activity.
	if _, err := d.cmd.Run(ctx, "adb", "-s", d.info.UDID,
		"shell", "appops", "set", "com.gads.settings", "PROJECT_MEDIA", "allow"); err != nil {
		return fmt.Errorf("startStream %s: add recording permission: %w", d.info.UDID, err)
	}
	time.Sleep(2 * time.Second)

	// Android 15+ requires POST_NOTIFICATIONS or startForeground() can throw.
	if d.semVer != nil && d.semVer.Major() >= 15 {
		if _, err := d.cmd.Run(ctx, "adb", "-s", d.info.UDID,
			"shell", "pm", "grant", "com.gads.settings", "android.permission.POST_NOTIFICATIONS"); err != nil {
			return fmt.Errorf("startStream %s: grant POST_NOTIFICATIONS: %w", d.info.UDID, err)
		}
		time.Sleep(1 * time.Second)
	}

	if _, err := d.cmd.Run(ctx, "adb", "-s", d.info.UDID,
		"shell", "am", "start", "-n", d.streamActivityName()); err != nil {
		return fmt.Errorf("startStream %s: start activity: %w", d.info.UDID, err)
	}
	return nil
}

// updateStreamSettings sends stream configuration (FPS, JPEG quality, scaling
// factor) to the GADS-Settings WebSocket on d.streamPort. The message format
// expected by GADS-Settings is "targetFPS=N:jpegQuality=N:scalingFactor=N"
// (H264 omits jpegQuality).
func (d *AndroidDevice) updateStreamSettings() error {
	u := url.URL{Scheme: "ws", Host: "localhost:" + d.streamPort}
	conn, _, _, err := ws.DefaultDialer.Dial(context.Background(), u.String())
	if err != nil {
		return fmt.Errorf("updateStreamSettings %s: dial: %w", d.info.UDID, err)
	}
	defer conn.Close()

	var msg string
	if d.info.StreamType == device.AndroidWebRTCGadsH264StreamTypeID {
		msg = fmt.Sprintf("targetFPS=%v:scalingFactor=%v",
			d.info.StreamTargetFPS, d.info.StreamScalingFactor)
	} else {
		msg = fmt.Sprintf("targetFPS=%v:jpegQuality=%v:scalingFactor=%v",
			d.info.StreamTargetFPS, d.info.StreamJpegQuality, d.info.StreamScalingFactor)
	}

	if err := wsutil.WriteServerMessage(conn, ws.OpText, []byte(msg)); err != nil {
		return fmt.Errorf("updateStreamSettings %s: send: %w", d.info.UDID, err)
	}
	return nil
}

// updateWebRTCTURNConfig fetches the TURN server config from the store and
// sends ephemeral credentials to the GADS-Settings WebRTC WebSocket. This
// must be called after port forwarding and before any WebRTC offer is made.
// If TURN is not enabled, the function returns nil without sending anything.
func (d *AndroidDevice) updateWebRTCTURNConfig() error {
	turnConfig, err := d.store.GetTURNConfig()
	if err != nil {
		return fmt.Errorf("updateWebRTCTURNConfig %s: get config: %w", d.info.UDID, err)
	}
	if !turnConfig.Enabled {
		return nil
	}
	if turnConfig.Server == "" || turnConfig.SharedSecret == "" {
		return fmt.Errorf("updateWebRTCTURNConfig %s: incomplete config (server=%s)", d.info.UDID, turnConfig.Server)
	}

	ttl := turnConfig.TTL
	if ttl == 0 {
		ttl = 3600
	}
	username, password, _ := auth.GenerateTURNCredentials(turnConfig.SharedSecret, ttl, d.cfg.TURNUsernameSuffix)

	u := url.URL{Scheme: "ws", Host: "localhost:" + d.streamPort}
	conn, _, _, err := ws.DefaultDialer.Dial(context.Background(), u.String())
	if err != nil {
		return fmt.Errorf("updateWebRTCTURNConfig %s: dial: %w", d.info.UDID, err)
	}
	defer conn.Close()

	msg := fmt.Sprintf(`{"type":"turn","server":"%s","port":%d,"username":"%s","password":"%s"}`,
		turnConfig.Server, turnConfig.Port, username, password)

	if err := wsutil.WriteServerMessage(conn, ws.OpText, []byte(msg)); err != nil {
		return fmt.Errorf("updateWebRTCTURNConfig %s: send: %w", d.info.UDID, err)
	}
	return nil
}
