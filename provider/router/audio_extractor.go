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
// and encodes them to Opus for WebRTC delivery. Both Android and iOS produce frames
// in the same wire format ([8B BE PTS][PCM int16 LE]).
//
// Producer/consumer roles differ between platforms:
//   - Android: APK runs a WebSocket SERVER on device.AudioPort; provider is the CLIENT (ws.Dial).
//   - iOS:     WDA's FBAudioWebSocketClient is the CLIENT; provider is the SERVER (Listen + Upgrade).
//             The iOS path is selected via device.OS == "ios" in NewPCMAudioExtractor.

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
		// iOS path: connect to the WDA FBAudioBroadcastRelay via the go-ios USB
		// forward (host:device.AudioPort → device:9202). Raw TCP, same wire format
		// as the Android WebSocket payload (`[8B PTS BE][1920B PCM Int16 LE]`).
		tcpAddress := "localhost:" + device.AudioPort
		if err := waitForAudioPort(ctx, tcpAddress, 30*time.Second); err != nil {
			cancel()
			return nil, fmt.Errorf("iOS audio relay not ready: %w", err)
		}
		conn, err := net.Dial("tcp", tcpAddress)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to connect to iOS audio relay: %w", err)
		}
		extractor.conn = conn
		logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Connected to iOS audio relay at %s for device %s", tcpAddress, device.UDID))
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

// extractAudioFramesIOSRaw reads fixed-size 1928-byte frames from the iOS
// FBAudioBroadcastRelay TCP stream (no WebSocket framing). Each frame is
// [8 B PTS BE][1920 B PCM Int16 LE]. Encodes to Opus and emits to audioChannel.
func (e *PCMAudioExtractor) extractAudioFramesIOSRaw() {
	defer close(e.audioChannel)
	defer func() {
		if e.conn != nil {
			e.conn.Close()
		}
	}()

	const frameTotal = 1928
	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Starting iOS raw audio frame extraction for device %s", e.device.UDID))
	frameCount := 0
	buf := make([]byte, frameTotal)

	for {
		select {
		case <-e.ctx.Done():
			return
		default:
		}

		if _, err := io.ReadFull(e.conn, buf); err != nil {
			if err != io.EOF {
				logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("iOS raw audio read error: %v", err))
			}
			return
		}

		pts, pcmData, err := parsePCMMessage(buf)
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
