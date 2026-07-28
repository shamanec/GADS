/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

// Android audio path: the APK's AudioWebSocketSender is the WS server on device port 1992
// (forwarded to device.AudioPort); the provider is the client. Body: [8B PTS BE][PCM int16 LE].

import (
	"GADS/common/models"
	"GADS/provider/logger"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// NewPCMAudioExtractorAndroid connects to the Android APK's audio WebSocket and produces
// Opus-encoded frames on the audio channel.
func NewPCMAudioExtractorAndroid(device *models.DBDevice) (*PCMAudioExtractor, error) {
	ctx, cancel := context.WithCancel(context.Background())

	encoder, err := newOpusEncoder()
	if err != nil {
		cancel()
		return nil, err
	}

	extractor := &PCMAudioExtractor{
		device:       device,
		audioChannel: make(chan AudioFrame, 30),
		ctx:          ctx,
		cancel:       cancel,
		encoder:      encoder,
	}

	// Internal Android audio requires the user to approve the MediaProjection dialog,
	// so allow more time for the APK's audio server to come up.
	audioWaitTimeout := 10 * time.Second
	if device.AudioInputType == "internal" || device.AudioInputType == "" {
		audioWaitTimeout = 30 * time.Second
	}
	tcpAddress := "localhost:" + device.AudioPort
	if err := waitForAudioPort(ctx, tcpAddress, audioWaitTimeout); err != nil {
		cancel()
		return nil, fmt.Errorf("audio server not ready: %w", err)
	}

	audioURL := "ws://localhost:" + device.AudioPort
	conn, _, _, err := ws.Dial(ctx, audioURL)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to audio stream: %w", err)
	}
	extractor.conn = conn

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Connected to audio stream for device %s", device.UDID))

	go extractor.extractAudioFrames()

	return extractor, nil
}

// extractAudioFrames reads PCM frames from the APK WebSocket, encodes to Opus, sends to channel.
func (e *PCMAudioExtractor) extractAudioFrames() {
	defer close(e.audioChannel)
	defer e.conn.Close()

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Starting audio frame extraction from WebSocket for device %s", e.device.UDID))

	frameCount := 0

	for {
		select {
		case <-e.ctx.Done():
			logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Stopping audio extraction for device %s", e.device.UDID))
			return
		default:
			msg, _, err := wsutil.ReadServerData(e.conn)
			if err != nil {
				if err != io.EOF {
					logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Error reading audio frame: %v", err))
				}
				return
			}

			pts, pcmData, err := parsePCMMessage(msg)
			if err != nil {
				continue
			}

			// Opus requires exactly 960 samples per frame (20ms @ 48kHz).
			// Pad with silence if fewer samples received; truncate if more.
			const opusFrameSize = 960
			pcmSamples := padOrTruncatePCM(decodePCMToInt16(pcmData), opusFrameSize)

			opusData := make([]byte, 4000) // Max Opus frame size
			n, err := e.encoder.Encode(pcmSamples, opusData)
			if err != nil {
				logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Opus encoding failed: %v", err))
				continue
			}

			frameCount++

			audioFrame := AudioFrame{
				Data: opusData[:n],
				PTS:  pts,
			}

			select {
			case e.audioChannel <- audioFrame:
				if frameCount%100 == 0 {
					logger.ProviderLogger.LogDebug("stream_webrtc", fmt.Sprintf("Processed audio frame #%d for device %s", frameCount, e.device.UDID))
				}
			case <-e.ctx.Done():
				return
			default:
				logger.ProviderLogger.LogWarn("stream_webrtc", fmt.Sprintf("Dropped audio frame #%d for device %s (channel full)", frameCount, e.device.UDID))
			}
		}
	}
}
