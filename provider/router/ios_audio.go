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
	"GADS/provider/devices"
	"GADS/provider/logger"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// The PCMAudioExtractor's lifecycle is owned by IOSWebRTCSession (see
// ios_stream_webrtc.go: created in Start(), closed in Close()). These endpoints
// are thin proxies that trigger the iOS-side broadcast UX via WDA — provider
// state lives on the session, not here.

// IOSAudioPrepare brings the IntegrationApp + RPSystemBroadcastPickerView to
// the foreground on the iOS device by calling WDA's /gads/audio/prepare endpoint.
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

	resp, err := wdaRequest(device, http.MethodPost, "gads/audio/prepare", nil)
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

// IOSAudioStart triggers the WDA-side audio start (picker UX). The provider's
// extractor is already running inside the active IOSWebRTCSession.
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

	// The provider reads frames from the on-device FBAudioBroadcastRelay via
	// go-ios USB forward — no host/port handshake needed (loopback only).
	// We still call /gads/audio/start to trigger the picker UX side-effect.
	resp, err := wdaRequest(device, http.MethodPost, "gads/audio/start", nil)
	if err != nil {
		logger.ProviderLogger.LogWarn("ios_audio", fmt.Sprintf("WDA /gads/audio/start call failed for device %s: %v", udid, err))
	} else {
		resp.Body.Close()
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
