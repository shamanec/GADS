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
	"GADS/common/models"
	"GADS/provider/devices"
	"GADS/provider/logger"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// audioPrepareBody returns the JSON body to send to WDA's /gads/audio/prepare.
// When the device has a configured AudioBroadcastTarget, the target is forwarded
// so WDA can auto-select the matching broadcast extension in the picker. When
// empty, we send no body — WDA falls back to its hardcoded default.
func audioPrepareBody(device *models.DBDevice) (io.Reader, error) {
	if device == nil || device.AudioBroadcastTarget == "" {
		return nil, nil
	}
	payload := struct {
		Target string `json:"target"`
	}{Target: device.AudioBroadcastTarget}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}

// resolveIOSDevice looks up the device via DevManager and returns its
// PlatformDevice + DBDevice after verifying it is an iOS device. Sends an HTTP
// error to the client on failure and returns (nil, nil, false).
func resolveIOSDevice(c *gin.Context, udid string) (devices.PlatformDevice, *models.DBDevice, bool) {
	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.BadRequest(c, fmt.Sprintf("Device with UDID `%s` not found", udid))
		return nil, nil, false
	}
	if platDev.GetOS() != "ios" {
		api.BadRequest(c, "this endpoint is only supported on iOS devices")
		return nil, nil, false
	}
	rcDev, ok := platDev.(devices.RemoteControllable)
	if !ok {
		api.InternalError(c, "device does not support remote control")
		return nil, nil, false
	}
	return platDev, rcDev.GetDBDevice(), true
}

// IOSAudioPrepare brings the GADSBroadcast host app + RPSystemBroadcastPickerView
// to the foreground on the iOS device by calling WDA's /gads/audio/prepare endpoint.
// Idempotent. Returns the host-side audio port.
func IOSAudioPrepare(c *gin.Context) {
	udid := c.Param("udid")
	platDev, device, ok := resolveIOSDevice(c, udid)
	if !ok {
		return
	}
	if !device.AudioStreamEnabled {
		api.BadRequest(c, "audio_stream_enabled is false for this device")
		return
	}
	if device.AudioPort == "" {
		api.InternalError(c, "device has no allocated audio port")
		return
	}

	body, err := audioPrepareBody(device)
	if err != nil {
		api.InternalError(c, fmt.Sprintf("failed to encode audio prepare body: %v", err))
		return
	}
	if device.AudioBroadcastTarget != "" {
		logger.ProviderLogger.LogInfo("ios_audio", fmt.Sprintf("calling /gads/audio/prepare for device %s with target=%q", udid, device.AudioBroadcastTarget))
	}
	resp, err := wdaRequest(platDev, http.MethodPost, "gads/audio/prepare", body)
	if err != nil {
		logger.ProviderLogger.LogError("ios_audio", fmt.Sprintf("Failed to call WDA /gads/audio/prepare for device %s: %v", udid, err))
		api.InternalError(c, fmt.Sprintf("failed to prepare audio on WDA: %v", err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		api.ErrorResponse(c, resp.StatusCode, fmt.Sprintf("WDA prepare returned %d", resp.StatusCode))
		return
	}

	api.OK(c, "audio prepared", gin.H{"audio_port": device.AudioPort})
}

// IOSAudioStart is a thin idempotent endpoint that confirms the audio source
// is ready for the caller. With gads-broadcast-extension as the production
// audio path, the runner is no longer in the data path — the extension writes
// PCM directly to device:8766, and the provider's extractor (started inside
// the active IOSWebRTCSession) reads from there. So there is no /gads/audio/start
// call to make on the WDA side; this handler stays callable for backward
// compatibility but performs no WDA round-trip.
func IOSAudioStart(c *gin.Context) {
	udid := c.Param("udid")
	_, device, ok := resolveIOSDevice(c, udid)
	if !ok {
		return
	}
	if !device.AudioStreamEnabled || device.AudioPort == "" {
		api.BadRequest(c, "audio not enabled or no port allocated")
		return
	}

	session := getIOSWebRTCSession(udid)
	if session == nil {
		api.BadRequest(c, "no active WebRTC session for device — start the stream first")
		return
	}

	api.OK(c, "audio started", gin.H{"audio_port": device.AudioPort})
}

// IOSAudioStop tears down the session-owned audio extractor (audio off, video
// stays alive) and asks WDA to terminate the broadcast.
// Idempotent: returns 200 even if no session/extractor was running.
func IOSAudioStop(c *gin.Context) {
	udid := c.Param("udid")
	platDev, _, ok := resolveIOSDevice(c, udid)
	if !ok {
		return
	}

	session := getIOSWebRTCSession(udid)
	if session != nil && session.audioExtractor != nil {
		session.audioExtractor.Close()
		session.audioExtractor = nil
	}

	resp, err := wdaRequest(platDev, http.MethodPost, "gads/audio/stop", nil)
	if err != nil {
		logger.ProviderLogger.LogWarn("ios_audio", fmt.Sprintf("WDA /gads/audio/stop call failed for device %s: %v", udid, err))
	} else {
		resp.Body.Close()
	}

	logger.ProviderLogger.LogInfo("ios_audio", fmt.Sprintf("iOS audio stop requested for device %s", udid))
	api.OKMessage(c, "audio stopped")
}
