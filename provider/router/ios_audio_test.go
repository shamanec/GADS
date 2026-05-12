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
	"io"
	"testing"

	"GADS/common/models"
	"GADS/provider/logger"

	"github.com/gin-gonic/gin"
	logrus "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func init() {
	if logger.ProviderLogger == nil {
		l := logrus.New()
		l.SetOutput(io.Discard)
		logger.ProviderLogger = &logger.CustomLogger{Logger: l}
	}
}

// TestIOSAudioRoutesRegistered ensures the prepare/start/stop endpoints are wired
// into the gin router so callers can reach them.
func TestIOSAudioRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := HandleRequests()

	want := map[string]bool{
		"POST /device/:udid/audio/prepare": false,
		"POST /device/:udid/audio/start":   false,
		"POST /device/:udid/audio/stop":    false,
	}

	for _, route := range r.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}

	for key, found := range want {
		assert.True(t, found, "expected route to be registered: %s", key)
	}
}

// TestNewWebRTCSession_AudioStreamEnabled_AddsAudioTrack asserts that an iOS
// session constructed for an audio-enabled device also creates the Opus track
// and adds it to the peer connection.
func TestNewWebRTCSession_AudioStreamEnabled_AddsAudioTrack(t *testing.T) {
	device := &models.Device{
		UDID:               "audio-track-test-udid",
		OS:                 "ios",
		AudioStreamEnabled: true,
		AudioInputType:     "app_audio",
		AudioPort:          "9999",
	}

	session, err := NewWebRTCSession(device)
	if err != nil {
		t.Fatalf("NewWebRTCSession failed: %v", err)
	}
	defer session.Close()

	assert.NotNil(t, session.audioTrack, "audio track should be created when AudioStreamEnabled=true")
	assert.True(t, device.AudioStreamEnabled, "AudioStreamEnabled should remain true after successful track creation")

	// Audio extractor is created lazily in Start(); not exercised here because
	// it would dial localhost:AudioPort and block on waitForAudioPort.
	assert.Nil(t, session.audioExtractor, "audio extractor should not exist before Start()")
}

func TestNewWebRTCSession_AudioStreamDisabled_NoAudioTrack(t *testing.T) {
	device := &models.Device{
		UDID:               "no-audio-test-udid",
		OS:                 "ios",
		AudioStreamEnabled: false,
	}

	session, err := NewWebRTCSession(device)
	if err != nil {
		t.Fatalf("NewWebRTCSession failed: %v", err)
	}
	defer session.Close()

	assert.Nil(t, session.audioTrack, "audio track should be nil when AudioStreamEnabled=false")
	assert.Nil(t, session.audioExtractor, "audio extractor should be nil when AudioStreamEnabled=false")
}
