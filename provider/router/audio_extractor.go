/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

// PCMAudioExtractor is media-agnostic: it consumes raw 48kHz/mono/16-bit-LE PCM frames
// and encodes them to Opus for WebRTC delivery.
//
// Wire formats by platform:
//   - Android: WebSocket message body = [8B PTS BE][PCM int16 LE]. Provider is the CLIENT.
//   - iOS:    Raw TCP framed by gads-broadcast-extension's h264 envelope:
//             [4B payloadLen BE][8B PTS BE][payloadLen bytes PCM int16 LE].
//             Provider connects via go-ios USB forward (loopback only inside the
//             device). For audio frames payloadLen is always 1920 (20 ms @ 48 kHz mono).

import (
	"GADS/common/models"
	"GADS/provider/logger"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/hraban/opus"
)

// AudioFrame represents an audio frame with its presentation timestamp
type AudioFrame struct {
	Data []byte // Opus-encoded audio data
	PTS  int64  // Presentation timestamp in microseconds
}

// parsePCMMessage parses the wire format sent by audio producers:
// [8 bytes big-endian PTS][PCM data]
// Returns the PTS and raw PCM bytes, or an error when the message is too short.
func parsePCMMessage(msg []byte) (pts int64, pcmData []byte, err error) {
	if len(msg) < 8 {
		return 0, nil, fmt.Errorf("message too short: got %d bytes, need at least 8", len(msg))
	}
	pts = int64(binary.BigEndian.Uint64(msg[0:8]))
	pcmData = msg[8:]
	return pts, pcmData, nil
}

// decodePCMToInt16 converts raw PCM bytes (little-endian int16 pairs) to []int16.
// Trailing odd bytes are ignored.
func decodePCMToInt16(pcmData []byte) []int16 {
	numSamples := len(pcmData) / 2
	samples := make([]int16, numSamples)
	for i := 0; i < numSamples; i++ {
		samples[i] = int16(pcmData[i*2]) | int16(pcmData[i*2+1])<<8
	}
	return samples
}

// padOrTruncatePCM returns a slice of exactly size samples from input.
// Missing samples are filled with zeros; excess samples are discarded.
func padOrTruncatePCM(samples []int16, size int) []int16 {
	out := make([]int16, size)
	copy(out, samples)
	return out
}

// PCMAudioExtractor handles extracting PCM audio from a producer WebSocket stream and encoding to Opus.
// Producers (Android APK, iOS WDA) send PCM in the same wire format; this extractor is media-agnostic.
type PCMAudioExtractor struct {
	device       *models.Device
	conn         io.ReadWriteCloser
	audioChannel chan AudioFrame
	ctx          context.Context
	cancel       context.CancelFunc
	encoder      *opus.Encoder
}

// waitForAudioPort waits up to maxWait for the audio TCP port to accept connections.
func waitForAudioPort(ctx context.Context, address string, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return fmt.Errorf("audio port %s not ready after %v", address, maxWait)
}

// iOS warm-up tunables: bridges the gap between /gads/audio/prepare returning
// 200 and gads-broadcast-extension's SampleHandler.broadcastStarted actually
// binding device port 8766. iproxy accepts the host-side connect immediately,
// so the extractor's first ReadFull may see EOF if the on-device listener
// isn't up yet. We tolerate that for a bounded window by reconnecting.
//
// The cold path includes WDA's autotap (~2-6 s, longer when Phase 2 horizontal
// swipe is needed because GADSBroadcast wasn't iOS's persisted picker
// selection) plus the mandatory iOS 3 s broadcast countdown. 20 s covers the
// worst case (Phase 2 + slow device animations) with margin.
const iosFirstHeaderMaxRetries = 100 // 100 × 200 ms = 20 s
const iosFirstHeaderRetryDelay = 200 * time.Millisecond

// readFirstHeaderWithReconnect performs the first 12-byte header read of an
// iOS audio session, tolerating a transient EOF by closing and redialing the
// USB-forwarded TCP connection. After the first frame is read, the steady-
// state loop in extractAudioFramesIOSRaw treats any EOF as a legitimate
// stream end.
//
// On success the freshly populated header is in `header` and e.conn points at
// the connection that produced it. On failure (real read error, or no frame
// after iosFirstHeaderMaxRetries × iosFirstHeaderRetryDelay) returns a
// descriptive error and leaves e.conn closed.
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

		// EOF before the first frame — the appex listener is probably not bound
		// yet. Reconnect and retry.
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

// NewPCMAudioExtractor creates a new audio extractor that connects to the device's audio WebSocket
// and produces Opus-encoded frames on the audio channel.
func NewPCMAudioExtractor(device *models.Device) (*PCMAudioExtractor, error) {
	ctx, cancel := context.WithCancel(context.Background())

	extractor := &PCMAudioExtractor{
		device:       device,
		audioChannel: make(chan AudioFrame, 30),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Create Opus encoder (48kHz, 1 channel, audio application)
	encoder, err := opus.NewEncoder(48000, 1, opus.AppAudio)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create Opus encoder: %w", err)
	}
	if err := encoder.SetBitrate(64000); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to set opus bitrate: %w", err)
	}
	if err := encoder.SetInBandFEC(true); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to enable opus fec: %w", err)
	}
	if err := encoder.SetPacketLossPerc(5); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to set opus packet loss perc: %w", err)
	}
	if err := encoder.SetMaxBandwidth(opus.Fullband); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to set opus max bandwidth: %w", err)
	}
	extractor.encoder = encoder

	if device.OS == "ios" {
		// iOS path: connect to gads-broadcast-extension's TCP server via the
		// go-ios USB forward (host:device.AudioPort → device:8766). Frames are
		// the unified h264 envelope: [4B payloadLen BE][8B PTS BE][payloadLen B PCM].
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

	// Android path: provider is the WebSocket CLIENT (APK is the server on device.AudioPort).
	// Internal Android audio requires the user to approve the MediaProjection dialog, so allow more time.
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

// extractAudioFramesIOSRaw reads h264-envelope frames from the
// gads-broadcast-extension TCP stream:
//
//	[4 B payloadLen BE][8 B PTS BE][payloadLen B PCM Int16 LE]
//
// For audio, payloadLen is always 1920 (20 ms @ 48 kHz mono). Frames with a
// different length are logged and dropped (port 8766 is audio-only today; if
// the extension ever multiplexes other types onto it, a type byte would need
// to be added — out of scope for this iteration).
//
// Encodes each PCM frame to Opus and emits on audioChannel.
func (e *PCMAudioExtractor) extractAudioFramesIOSRaw() {
	defer close(e.audioChannel)
	defer func() {
		if e.conn != nil {
			e.conn.Close()
		}
	}()

	const expectedPayloadLen = 1920
	const opusFrameSize = 960
	// Hard cap on payload size to prevent allocating gigabytes if the upstream
	// stream goes haywire; comfortably above expectedPayloadLen for headroom.
	const maxPayloadLen = 64 * 1024

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Starting iOS audio frame extraction (8766/h264-envelope) for device %s", e.device.UDID))
	frameCount := 0
	header := make([]byte, 12)

	// First-frame warm-up: the on-device broadcast extension may not have
	// bound port 8766 yet at the moment iproxy accepts our connect. Tolerate
	// transient EOFs by reconnecting until we get a real header.
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

// startIOSAudioServer (deprecated, unused).
func (e *PCMAudioExtractor) startIOSAudioServer() error {
	listener, err := net.Listen("tcp", "0.0.0.0:"+e.device.AudioPort)
	if err != nil {
		return fmt.Errorf("failed to bind audio WS server: %w", err)
	}

	connCh := make(chan io.ReadWriteCloser, 1)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("iOS audio WS request from %s: method=%s path=%q upgrade=%q connection=%q sec-ws-key=%q sec-ws-version=%q sec-ws-protocol=%q user-agent=%q", r.RemoteAddr, r.Method, r.URL.Path, r.Header.Get("Upgrade"), r.Header.Get("Connection"), r.Header.Get("Sec-WebSocket-Key"), r.Header.Get("Sec-WebSocket-Version"), r.Header.Get("Sec-WebSocket-Protocol"), r.Header.Get("User-Agent")))
			conn, _, _, upgradeErr := ws.UpgradeHTTP(r, w)
			if upgradeErr != nil {
				logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("iOS audio WS upgrade failed for device %s: %v", e.device.UDID, upgradeErr))
				return
			}
			select {
			case connCh <- conn:
				logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("iOS audio WS client connected for device %s from %s", e.device.UDID, r.RemoteAddr))
			default:
				logger.ProviderLogger.LogWarn("stream_webrtc", fmt.Sprintf("iOS audio WS extra client rejected for device %s from %s", e.device.UDID, r.RemoteAddr))
				conn.Close()
			}
		}),
	}

	go func() {
		_ = server.Serve(listener)
	}()

	go func() {
		<-e.ctx.Done()
		_ = server.Close()
		_ = listener.Close()
	}()

	go func() {
		select {
		case conn := <-connCh:
			e.conn = conn
			e.extractAudioFramesIOS()
		case <-e.ctx.Done():
			return
		}
	}()

	return nil
}

// extractAudioFrames reads PCM frames from WebSocket, encodes to Opus, sends to channel
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

// extractAudioFramesIOS reads PCM frames from a server-side WebSocket conn (iOS path).
// Symmetric to extractAudioFrames but uses ReadClientData since the provider is the server.
func (e *PCMAudioExtractor) extractAudioFramesIOS() {
	defer close(e.audioChannel)
	defer func() {
		if e.conn != nil {
			e.conn.Close()
		}
	}()

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Starting iOS audio frame extraction for device %s", e.device.UDID))

	frameCount := 0

	for {
		select {
		case <-e.ctx.Done():
			logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Stopping iOS audio extraction for device %s", e.device.UDID))
			return
		default:
			msg, _, err := wsutil.ReadClientData(e.conn)
			if err != nil {
				if err != io.EOF {
					logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Error reading iOS audio frame: %v", err))
				}
				return
			}

			pts, pcmData, err := parsePCMMessage(msg)
			if err != nil {
				continue
			}

			const opusFrameSize = 960
			pcmSamples := padOrTruncatePCM(decodePCMToInt16(pcmData), opusFrameSize)

			opusData := make([]byte, 4000)
			n, err := e.encoder.Encode(pcmSamples, opusData)
			if err != nil {
				logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("iOS Opus encoding failed: %v", err))
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
					logger.ProviderLogger.LogDebug("stream_webrtc", fmt.Sprintf("Processed iOS audio frame #%d for device %s", frameCount, e.device.UDID))
				}
			case <-e.ctx.Done():
				return
			default:
				logger.ProviderLogger.LogWarn("stream_webrtc", fmt.Sprintf("Dropped iOS audio frame #%d for device %s (channel full)", frameCount, e.device.UDID))
			}
		}
	}
}

// GetAudioChannel returns the channel for reading audio frames
func (e *PCMAudioExtractor) GetAudioChannel() <-chan AudioFrame {
	return e.audioChannel
}

// Close stops the extractor
func (e *PCMAudioExtractor) Close() {
	e.cancel()
	if e.conn != nil {
		e.conn.Close()
	}
}
