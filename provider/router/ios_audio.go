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
func audioPrepareBody(device *models.Device) (io.Reader, error) {
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

// The PCMAudioExtractor's lifecycle is owned by IOSWebRTCSession (see
// ios_stream_webrtc.go: created in Start(), closed in Close()). These endpoints
// are thin proxies that trigger the iOS-side broadcast UX via WDA — provider
// state lives on the session, not here.

// IOSAudioPrepare brings the GADSBroadcast host app + RPSystemBroadcastPickerView
// to the foreground on the iOS device by calling WDA's /gads/audio/prepare endpoint.
// Idempotent. Returns the host-side audio port.
func IOSAudioPrepare(c *gin.Context) {
	udid := c.Param("udid")
	device, ok := devices.DBDeviceMap[udid]
	if !ok || device == nil {
		api.BadRequestResponse(c, fmt.Sprintf("Device with UDID `%s` not found", udid), nil)
		return
	}
	if device.OS != "ios" {
		api.BadRequestResponse(c, "audio prepare is only supported on iOS devices", nil)
		return
	}
	if !device.AudioStreamEnabled {
		api.BadRequestResponse(c, "audio_stream_enabled is false for this device", nil)
		return
	}
	if device.AudioPort == "" {
		api.InternalServerErrorResponse(c, "device has no allocated audio port", nil)
		return
	}

	body, err := audioPrepareBody(device)
	if err != nil {
		api.InternalServerErrorResponse(c, fmt.Sprintf("failed to encode audio prepare body: %v", err), nil)
		return
	}
	if device.AudioBroadcastTarget != "" {
		logger.ProviderLogger.LogInfo("ios_audio", fmt.Sprintf("calling /gads/audio/prepare for device %s with target=%q", udid, device.AudioBroadcastTarget))
	}
	resp, err := wdaRequest(device, http.MethodPost, "gads/audio/prepare", body)
	if err != nil {
		logger.ProviderLogger.LogError("ios_audio", fmt.Sprintf("Failed to call WDA /gads/audio/prepare for device %s: %v", udid, err))
		api.InternalServerErrorResponse(c, fmt.Sprintf("failed to prepare audio on WDA: %v", err), nil)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		api.GenericResponse(c, resp.StatusCode, fmt.Sprintf("WDA prepare returned %d", resp.StatusCode), nil)
		return
	}

	api.OKResponse(c, "audio prepared", gin.H{"audio_port": device.AudioPort})
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
	device, ok := devices.DBDeviceMap[udid]
	if !ok || device == nil {
		api.BadRequestResponse(c, fmt.Sprintf("Device with UDID `%s` not found", udid), nil)
		return
	}
	if device.OS != "ios" {
		api.BadRequestResponse(c, "audio start is only supported on iOS devices", nil)
		return
	}
	if !device.AudioStreamEnabled || device.AudioPort == "" {
		api.BadRequestResponse(c, "audio not enabled or no port allocated", nil)
		return
	}

	session := getIOSWebRTCSession(udid)
	if session == nil {
		api.BadRequestResponse(c, "no active WebRTC session for device — start the stream first", nil)
		return
	}

	api.OKResponse(c, "audio started", gin.H{"audio_port": device.AudioPort})
}

// IOSAudioStop tears down the session-owned audio extractor (audio off, video
// stays alive) and asks WDA to terminate the broadcast.
// Idempotent: returns 200 even if no session/extractor was running.
func IOSAudioStop(c *gin.Context) {
	udid := c.Param("udid")
	device, ok := devices.DBDeviceMap[udid]
	if !ok || device == nil {
		api.BadRequestResponse(c, fmt.Sprintf("Device with UDID `%s` not found", udid), nil)
		return
	}

	session := getIOSWebRTCSession(udid)
	if session != nil && session.audioExtractor != nil {
		session.audioExtractor.Close()
		session.audioExtractor = nil
	}

	if device.OS == "ios" {
		resp, err := wdaRequest(device, http.MethodPost, "gads/audio/stop", nil)
		if err != nil {
			logger.ProviderLogger.LogWarn("ios_audio", fmt.Sprintf("WDA /gads/audio/stop call failed for device %s: %v", udid, err))
		} else {
			resp.Body.Close()
		}
	}

	logger.ProviderLogger.LogInfo("ios_audio", fmt.Sprintf("iOS audio stop requested for device %s", udid))
	api.OKResponse(c, "audio stopped", nil)
}
