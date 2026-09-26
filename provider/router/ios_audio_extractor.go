/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

// iOS audio: gads-broadcast-extension serves PCM on device port 8766 (USB-forwarded to
// device.AudioPort) as [4B len BE][8B PTS BE][PCM int16 LE], 1920 B/frame (20 ms @ 48 kHz mono).

import (
	"GADS/common/models"
	"GADS/provider/logger"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// Warm-up window: the appex may not have bound port 8766 when iproxy accepts our connect, so
// the first read can EOF. 20 s covers WDA autotap (~2-6 s) plus the iOS 3 s broadcast countdown.
const iosFirstHeaderMaxRetries = 100 // 100 × 200 ms = 20 s
const iosFirstHeaderRetryDelay = 200 * time.Millisecond

// NewPCMAudioExtractorIOS connects to the gads-broadcast-extension TCP server via the go-ios USB
// forward (host:device.AudioPort → device:8766) and produces Opus-encoded frames on the channel.
func NewPCMAudioExtractorIOS(device *models.DBDevice) (*PCMAudioExtractor, error) {
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

	tcpAddress := "localhost:" + device.AudioPort
	if err := waitForAudioPort(ctx, tcpAddress, 30*time.Second); err != nil {
		cancel()
		return nil, fmt.Errorf("iOS audio source not ready: %w", err)
	}
	conn, err := net.Dial("tcp", tcpAddress)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to iOS audio source: %w", err)
	}
	extractor.conn = conn
	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Connected to iOS audio source at %s for device %s", tcpAddress, device.UDID))

	go extractor.extractAudioFramesIOSRaw()
	return extractor, nil
}

// readFirstHeaderWithReconnect reads the first 12-byte header, redialing on transient EOF until
// the appex binds. Once the first frame lands, the steady-state loop treats EOF as stream end.
func (e *PCMAudioExtractor) readFirstHeaderWithReconnect(header []byte) error {
	address := "localhost:" + e.device.AudioPort
	for attempt := 1; attempt <= iosFirstHeaderMaxRetries; attempt++ {
		select {
		case <-e.ctx.Done():
			return e.ctx.Err()
		default:
		}

		_, err := io.ReadFull(e.conn, header)
		if err == nil {
			return nil
		}
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("first-header read error: %w", err)
		}

		// EOF before the first frame — the appex listener is probably not bound yet. Reconnect.
		_ = e.conn.Close()
		select {
		case <-e.ctx.Done():
			return e.ctx.Err()
		case <-time.After(iosFirstHeaderRetryDelay):
		}
		conn, dialErr := net.Dial("tcp", address)
		if dialErr != nil {
			logger.ProviderLogger.LogWarn("stream_webrtc", fmt.Sprintf("iOS audio reconnect attempt %d/%d failed for device %s: %v", attempt, iosFirstHeaderMaxRetries, e.device.UDID, dialErr))
			continue
		}
		e.conn = conn
	}
	return fmt.Errorf("appex never emitted a frame after %d retries (~%s)", iosFirstHeaderMaxRetries, time.Duration(iosFirstHeaderMaxRetries)*iosFirstHeaderRetryDelay)
}

// extractAudioFramesIOSRaw reads h264-envelope frames from the gads-broadcast-extension TCP stream,
// encodes each PCM frame to Opus and emits it on audioChannel.
func (e *PCMAudioExtractor) extractAudioFramesIOSRaw() {
	defer close(e.audioChannel)
	defer func() {
		if e.conn != nil {
			e.conn.Close()
		}
	}()

	const expectedPayloadLen = 1920
	const opusFrameSize = 960
	// Hard cap to prevent allocating gigabytes if the upstream stream goes haywire.
	const maxPayloadLen = 64 * 1024

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Starting iOS audio frame extraction (8766/h264-envelope) for device %s", e.device.UDID))
	frameCount := 0
	header := make([]byte, 12)

	// First-frame warm-up: the on-device broadcast extension may not have bound port 8766 yet at
	// the moment iproxy accepts our connect. Tolerate transient EOFs by reconnecting.
	if err := e.readFirstHeaderWithReconnect(header); err != nil {
		logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("iOS audio extractor warm-up failed for device %s: %v", e.device.UDID, err))
		return
	}
	firstIteration := true

	for {
		select {
		case <-e.ctx.Done():
			return
		default:
		}

		if !firstIteration {
			if _, err := io.ReadFull(e.conn, header); err != nil {
				if err != io.EOF {
					logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("iOS audio header read error: %v", err))
				}
				return
			}
		}
		firstIteration = false

		payloadLen := binary.BigEndian.Uint32(header[0:4])
		pts := int64(binary.BigEndian.Uint64(header[4:12]))

		if payloadLen == 0 || payloadLen > maxPayloadLen {
			logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("iOS audio frame has implausible payloadLen=%d for device %s — aborting stream", payloadLen, e.device.UDID))
			return
		}

		pcmData := make([]byte, payloadLen)
		if _, err := io.ReadFull(e.conn, pcmData); err != nil {
			if err != io.EOF {
				logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("iOS audio payload read error (len=%d): %v", payloadLen, err))
			}
			return
		}

		if payloadLen != expectedPayloadLen {
			logger.ProviderLogger.LogWarn("stream_webrtc", fmt.Sprintf("Dropping iOS audio frame with unexpected payloadLen=%d (want %d) for device %s", payloadLen, expectedPayloadLen, e.device.UDID))
			continue
		}

		pcmSamples := padOrTruncatePCM(decodePCMToInt16(pcmData), opusFrameSize)

		opusData := make([]byte, 4000)
		n, err := e.encoder.Encode(pcmSamples, opusData)
		if err != nil {
			logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("iOS Opus encoding failed: %v", err))
			continue
		}
		frameCount++

		select {
		case e.audioChannel <- AudioFrame{Data: opusData[:n], PTS: pts}:
			if frameCount%100 == 0 {
				logger.ProviderLogger.LogDebug("stream_webrtc", fmt.Sprintf("Processed iOS audio frame #%d for device %s", frameCount, e.device.UDID))
			}
		case <-e.ctx.Done():
			return
		default:
			logger.ProviderLogger.LogWarn("stream_webrtc", fmt.Sprintf("Dropped iOS audio frame #%d for device %s", frameCount, e.device.UDID))
		}
	}
}
