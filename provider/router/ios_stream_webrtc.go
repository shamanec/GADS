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
	"GADS/common/utils"
	"GADS/provider/manager"
	"GADS/common/models"
	"GADS/provider/logger"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

// parseJPEGDimensions extracts width and height from JPEG data
// JPEG format: SOI (FFD8) followed by markers (FF XX)
// SOF0 (FFC0) or SOF2 (FFC2) contains dimensions: length(2) + precision(1) + height(2) + width(2)
func parseJPEGDimensions(data []byte) (width, height int, err error) {
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0, 0, fmt.Errorf("invalid JPEG: missing SOI marker")
	}

	offset := 2
	for offset < len(data)-1 {
		// Find marker
		if data[offset] != 0xFF {
			offset++
			continue
		}

		marker := data[offset+1]
		offset += 2

		// Skip padding bytes
		if marker == 0xFF {
			continue
		}

		// SOI, EOI, RST markers have no length
		if marker == 0xD8 || marker == 0xD9 || (marker >= 0xD0 && marker <= 0xD7) {
			continue
		}

		// Need at least 2 bytes for length
		if offset+2 > len(data) {
			break
		}

		length := int(data[offset])<<8 | int(data[offset+1])

		// SOF markers (C0-CF except C4, C8, CC)
		if (marker >= 0xC0 && marker <= 0xCF) && marker != 0xC4 && marker != 0xC8 && marker != 0xCC {
			if offset+7 > len(data) {
				return 0, 0, fmt.Errorf("invalid JPEG: truncated SOF segment")
			}
			// Skip length (2 bytes) and precision (1 byte)
			height = int(data[offset+3])<<8 | int(data[offset+4])
			width = int(data[offset+5])<<8 | int(data[offset+6])
			return width, height, nil
		}

		offset += length
	}

	return 0, 0, fmt.Errorf("invalid JPEG: no SOF marker found")
}

// WDAJPEGExtractor handles extracting JPEG frames from WebDriverAgent MJPEG stream
type WDAJPEGExtractor struct {
	device             *models.DeviceInfo
	httpResp           *http.Response
	multiReader        *multipart.Reader
	jpegChannel        chan []byte
	ctx                context.Context
	cancel             context.CancelFunc
	ffmpegCmd          *exec.Cmd
	ffmpegStdin        io.WriteCloser
	ffmpegStdout       io.ReadCloser
	currentWidth       int
	currentHeight      int
	orientationChanged chan struct{}
	frameCount         int
}

// NewWDAJPEGExtractor creates a new JPEG extractor for WebDriverAgent stream
func NewWDAJPEGExtractor(device *models.DeviceInfo) (*WDAJPEGExtractor, error) {
	ctx, cancel := context.WithCancel(context.Background())
	success := false
	defer func() {
		if !success {
			cancel()
		}
	}()

	extractor := &WDAJPEGExtractor{
		device:             device,
		jpegChannel:        make(chan []byte, 5), // Smaller buffer for lower latency
		ctx:                ctx,
		cancel:             cancel,
		orientationChanged: make(chan struct{}, 1),
	}

	// Connect to WDA MJPEG stream
	streamUrl := "http://localhost:" + device.WDAStreamPort
	req, err := http.NewRequestWithContext(ctx, "GET", streamUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WDA stream: %w", err)
	}
	defer func() {
		if !success && resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()

	extractor.httpResp = resp

	// Parse multipart boundary
	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse content-type: %w", err)
	}

	if !strings.HasPrefix(mediaType, "multipart/") {
		return nil, fmt.Errorf("invalid media type: %s", mediaType)
	}

	// Clean boundary string (remove leading --)
	boundary := strings.Replace(params["boundary"], "--", "", -1)
	extractor.multiReader = multipart.NewReader(resp.Body, boundary)

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Connected to WDA MJPEG stream for device %s", device.UDID))

	success = true
	return extractor, nil
}

// Start begins extracting JPEGs from the WDA stream
func (e *WDAJPEGExtractor) Start() {
	go e.extractJPEGs()
}

// extractJPEGs reads JPEG frames from multipart stream and sends to channel
func (e *WDAJPEGExtractor) extractJPEGs() {
	defer close(e.jpegChannel)
	defer e.httpResp.Body.Close()

	for {
		select {
		case <-e.ctx.Done():
			logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Stopping JPEG extraction for device %s", e.device.UDID))
			return
		default:
			part, err := e.multiReader.NextPart()
			if err == io.EOF {
				logger.ProviderLogger.LogWarn("stream_webrtc", fmt.Sprintf("WDA stream ended for device %s", e.device.UDID))
				return
			}
			if err != nil {
				logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Error reading multipart for device %s: %s", e.device.UDID, err))
				return
			}

			jpegData, err := io.ReadAll(part)
			if err != nil {
				logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Error reading JPEG data for device %s: %s", e.device.UDID, err))
				continue
			}

			// Only check dimensions every 30 frames (~1 second at 30fps) to reduce overhead
			e.frameCount++
			if e.frameCount >= 30 {
				e.frameCount = 0
				width, height, err := parseJPEGDimensions(jpegData)
				if err != nil {
					logger.ProviderLogger.LogDebug("stream_webrtc",
						fmt.Sprintf("Failed to parse JPEG dimensions for device %s: %s", e.device.UDID, err))
				} else {
					// Check if orientation changed (portrait <-> landscape)
					if e.currentWidth > 0 && e.currentHeight > 0 {
						wasLandscape := e.currentWidth > e.currentHeight
						isLandscape := width > height
						if wasLandscape != isLandscape {
							logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Orientation changed for device %s: %dx%d -> %dx%d", e.device.UDID, e.currentWidth, e.currentHeight, width, height))
							// Non-blocking send to orientation change channel
							select {
							case e.orientationChanged <- struct{}{}:
							default:
								// Channel full, skip (already notified)
							}
						}
					}
					e.currentWidth = width
					e.currentHeight = height
				}
			}

			// Send JPEG to channel (non-blocking)
			select {
			case e.jpegChannel <- jpegData:
			case <-e.ctx.Done():
				return
			default:
				// Drop frame if channel is full (backpressure)
				logger.ProviderLogger.LogDebug("stream_webrtc", fmt.Sprintf("Dropped frame for device %s (channel full)", e.device.UDID))
			}
		}
	}
}

// GetJPEGChannel returns the channel that receives JPEG frames
func (e *WDAJPEGExtractor) GetJPEGChannel() <-chan []byte {
	return e.jpegChannel
}

// GetOrientationChangeChannel returns the channel that signals orientation changes
func (e *WDAJPEGExtractor) GetOrientationChangeChannel() <-chan struct{} {
	return e.orientationChanged
}

// Close stops the extractor and cleans up resources
func (e *WDAJPEGExtractor) Close() {
	e.cancel()
	if e.httpResp != nil && e.httpResp.Body != nil {
		e.httpResp.Body.Close()
	}
}

// FFmpegH264Encoder handles encoding JPEG frames to H.264
type FFmpegH264Encoder struct {
	device       *models.DeviceInfo
	jpegInput    <-chan []byte
	h264Output   chan []byte
	ctx          context.Context
	cancel       context.CancelFunc
	ffmpegCmd    *exec.Cmd
	ffmpegStdin  io.WriteCloser
	ffmpegStdout io.ReadCloser
}

// NewFFmpegH264Encoder creates a new H.264 encoder using FFmpeg
func NewFFmpegH264Encoder(device *models.DeviceInfo, jpegInput <-chan []byte) (*FFmpegH264Encoder, error) {
	ctx, cancel := context.WithCancel(context.Background())
	success := false
	defer func() {
		if !success {
			cancel()
		}
	}()

	encoder := &FFmpegH264Encoder{
		device:     device,
		jpegInput:  jpegInput,
		h264Output: make(chan []byte, 10), // Smaller buffer for lower latency
		ctx:        ctx,
		cancel:     cancel,
	}

	// Create FFmpeg command for JPEG to H.264 encoding with minimal latency
	// -probesize 32: minimal probing to reduce startup delay
	// -fflags nobuffer: disable buffering
	// -flags low_delay: optimize for low delay
	// -f image2pipe: input is a stream of images
	// -framerate 30: assume 30fps input
	// -i -: read from stdin
	// -c:v libx264: use H.264 encoder
	// -preset ultrafast: fastest encoding preset
	// -tune zerolatency: optimize for zero latency streaming
	// -pix_fmt yuv420p: compatible pixel format
	// -g 30: keyframe every 1 second (more frequent for lower latency)
	// -keyint_min 15: minimum keyframe interval
	// -x264opts: low-latency x264 options
	//   - no-scenecut: disable scene detection
	//   - rc-lookahead=0: disable lookahead (critical for latency)
	//   - sync-lookahead=0: disable sync lookahead
	//   - sliced-threads: reduce threading latency
	//   - bframes=0: no B-frames
	// -bf 0: no B-frames
	// -refs 1: single reference frame
	// -sc_threshold 0: disable scene change detection
	// -profile:v baseline: most compatible profile
	// -level 3.1: compatibility level
	// -max_delay 0: no buffering delay
	// -fflags nobuffer+flush_packets: disable buffering and flush immediately
	// -f h264: output raw H.264 stream
	// -: write to stdout
	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-probesize", "32",
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-f", "image2pipe",
		"-framerate", fmt.Sprintf("%v", device.StreamTargetFPS),
		"-i", "-",
		"-vf", "scale='trunc(iw/2)*2:trunc(ih/2)*2'",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-pix_fmt", "yuv420p",
		"-g", "30",
		"-keyint_min", "15",
		"-x264opts", "no-scenecut:rc-lookahead=0:sync-lookahead=0:sliced-threads:bframes=0",
		"-bf", "0",
		"-refs", "1",
		"-sc_threshold", "0",
		"-profile:v", "baseline",
		"-level", "3.1",
		"-max_delay", "0",
		"-fflags", "+nobuffer+flush_packets",
		"-f", "h264",
		"-",
	)

	// Get stdin pipe for writing JPEGs
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	defer func() {
		if !success {
			stdin.Close()
		}
	}()
	encoder.ffmpegStdin = stdin

	// Get stdout pipe for reading H.264 stream
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	defer func() {
		if !success {
			stdout.Close()
		}
	}()
	encoder.ffmpegStdout = stdout

	// Start FFmpeg process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	encoder.ffmpegCmd = cmd
	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Started FFmpeg H.264 encoder for device %s", device.UDID))

	success = true
	return encoder, nil
}

// Start begins encoding JPEGs to H.264
func (e *FFmpegH264Encoder) Start() {
	go e.writeJPEGsToFFmpeg()
	go e.readH264FromFFmpeg()
	go e.waitForFFmpeg()
}

// writeJPEGsToFFmpeg reads JPEGs from input channel and writes to FFmpeg stdin
func (e *FFmpegH264Encoder) writeJPEGsToFFmpeg() {
	defer e.ffmpegStdin.Close()

	for {
		select {
		case <-e.ctx.Done():
			return
		case jpegData, ok := <-e.jpegInput:
			if !ok {
				logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("JPEG input closed for device %s", e.device.UDID))
				return
			}

			// Write JPEG to FFmpeg stdin
			_, err := e.ffmpegStdin.Write(jpegData)
			if err != nil {
				logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Failed to write JPEG to FFmpeg for device %s: %s", e.device.UDID, err))
				return
			}
		}
	}
}

// readH264FromFFmpeg reads H.264 NAL units from FFmpeg stdout
func (e *FFmpegH264Encoder) readH264FromFFmpeg() {
	defer close(e.h264Output)

	const nalStartCode = "\x00\x00\x00\x01"
	var nalBuffer []byte
	readBuffer := make([]byte, 32768) // 32KB read buffer

	for {
		select {
		case <-e.ctx.Done():
			return
		default:
			n, err := e.ffmpegStdout.Read(readBuffer)
			if err != nil {
				if err != io.EOF {
					logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Error reading H.264 from FFmpeg for device %s: %s", e.device.UDID, err))
				}
				// Send any remaining buffered data
				if len(nalBuffer) > 0 {
					e.sendNALUnit(nalBuffer)
				}
				return
			}

			if n > 0 {
				nalBuffer = append(nalBuffer, readBuffer[:n]...)
				nalBuffer = e.processNALBuffer(nalBuffer)
			}
		}
	}
}

// processNALBuffer extracts complete NAL units from buffer
func (e *FFmpegH264Encoder) processNALBuffer(buffer []byte) []byte {
	const nalStartCode = "\x00\x00\x00\x01"
	startCodeBytes := []byte(nalStartCode)

	for {
		// Find first NAL start code
		firstStart := bytes.Index(buffer, startCodeBytes)
		if firstStart == -1 {
			// No start code found, keep buffer as-is
			return buffer
		}

		// Find next NAL start code
		nextStart := bytes.Index(buffer[firstStart+4:], startCodeBytes)
		if nextStart == -1 {
			// No second start code, wait for more data
			return buffer
		}

		// Extract complete NAL unit (including start code)
		nextStart += firstStart + 4
		nalUnit := buffer[firstStart:nextStart]

		// Send the NAL unit
		e.sendNALUnit(nalUnit)

		// Remove processed NAL unit from buffer
		buffer = buffer[nextStart:]
	}
}

// sendNALUnit sends a complete NAL unit to the output channel
func (e *FFmpegH264Encoder) sendNALUnit(nalUnit []byte) {
	if len(nalUnit) == 0 {
		return
	}

	// Make a copy to avoid data races
	nalCopy := make([]byte, len(nalUnit))
	copy(nalCopy, nalUnit)

	select {
	case e.h264Output <- nalCopy:
	case <-e.ctx.Done():
		return
	default:
		logger.ProviderLogger.LogDebug("stream_webrtc", fmt.Sprintf("Dropped NAL unit for device %s (channel full)", e.device.UDID))
	}
}

// waitForFFmpeg waits for FFmpeg process to exit
func (e *FFmpegH264Encoder) waitForFFmpeg() {
	if err := e.ffmpegCmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("FFmpeg process exited with error for device %s: %s", e.device.UDID, err))
	} else {
		logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("FFmpeg process exited for device %s", e.device.UDID))
	}
	e.cancel()
}

// GetH264Channel returns the channel that receives H.264 data
func (e *FFmpegH264Encoder) GetH264Channel() <-chan []byte {
	return e.h264Output
}

// Close stops the encoder and cleans up resources
func (e *FFmpegH264Encoder) Close() {
	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Closing FFmpeg encoder for device %s", e.device.UDID))

	// Cancel context first to signal goroutines
	e.cancel()

	// Kill FFmpeg process - this will unblock any pending Read/Write operations
	if e.ffmpegCmd != nil && e.ffmpegCmd.Process != nil {
		if err := e.ffmpegCmd.Process.Kill(); err != nil {
			logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Failed to kill FFmpeg process for device %s: %s", e.device.UDID, err))
		} else {
			logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Killed FFmpeg process for device %s", e.device.UDID))
		}
	}

	// Close pipes after killing process
	if e.ffmpegStdin != nil {
		e.ffmpegStdin.Close()
	}
	if e.ffmpegStdout != nil {
		e.ffmpegStdout.Close()
	}
}

// WebRTCSession manages a single WebRTC peer connection for streaming
type WebRTCSession struct {
	device                  *models.DeviceInfo
	peerConnection          *webrtc.PeerConnection
	videoTrack              *webrtc.TrackLocalStaticSample
	extractor               *WDAJPEGExtractor
	encoder                 *FFmpegH264Encoder
	ctx                     context.Context
	cancel                  context.CancelFunc
	mu                      sync.Mutex
	iceCandidates           []webrtc.ICECandidateInit
	pendingOffer            *webrtc.SessionDescription
	onOrientationChangeFunc func()
}

// NewWebRTCSession creates a new WebRTC session for device streaming
func NewWebRTCSession(device *models.DeviceInfo) (*WebRTCSession, error) {
	ctx, cancel := context.WithCancel(context.Background())

	session := &WebRTCSession{
		device:        device,
		ctx:           ctx,
		cancel:        cancel,
		iceCandidates: make([]webrtc.ICECandidateInit, 0),
	}

	// Create WebRTC configuration
	webrtcConfig := utils.GenerateWebRTCConfig()

	// Create peer connection
	pc, err := webrtc.NewPeerConnection(webrtcConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}
	session.peerConnection = pc

	// Create video track
	videoTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
		"video",
		"gads-stream",
	)
	if err != nil {
		cancel()
		pc.Close()
		return nil, fmt.Errorf("failed to create video track: %w", err)
	}
	session.videoTrack = videoTrack

	// Add track to peer connection
	rtpSender, err := pc.AddTrack(videoTrack)
	if err != nil {
		cancel()
		pc.Close()
		return nil, fmt.Errorf("failed to add track: %w", err)
	}

	// Handle RTCP packets
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Created WebRTC session for device %s", device.UDID))

	return session, nil
}

// Start begins the streaming pipeline
func (s *WebRTCSession) Start() error {
	// Create JPEG extractor
	extractor, err := NewWDAJPEGExtractor(s.device)
	if err != nil {
		return fmt.Errorf("failed to create JPEG extractor: %w", err)
	}
	s.extractor = extractor
	extractor.Start()

	// Create H.264 encoder
	encoder, err := NewFFmpegH264Encoder(s.device, extractor.GetJPEGChannel())
	if err != nil {
		extractor.Close()
		return fmt.Errorf("failed to create H.264 encoder: %w", err)
	}
	s.encoder = encoder
	encoder.Start()

	// Start writing H.264 to WebRTC track
	go s.writeH264ToTrack()

	// Start watching for orientation changes to signal client
	go s.watchOrientationChanges()

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Started streaming pipeline for device %s", s.device.UDID))
	return nil
}

// writeH264ToTrack reads H.264 data and writes to WebRTC video track
func (s *WebRTCSession) writeH264ToTrack() {
	h264Channel := s.encoder.GetH264Channel()

	// Calculate frame duration based on device settings (default 30fps)
	fps := 30
	if s.device.StreamTargetFPS > 0 {
		fps = s.device.StreamTargetFPS
	}
	frameDuration := time.Second / time.Duration(fps)

	for {
		select {
		case <-s.ctx.Done():
			return
		case h264Data, ok := <-h264Channel:
			if !ok {
				logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("H.264 channel closed for device %s", s.device.UDID))
				return
			}

			// Write complete NAL unit to track
			if err := s.videoTrack.WriteSample(media.Sample{
				Data:     h264Data,
				Duration: frameDuration,
			}); err != nil {
				logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Failed to write sample to track for device %s: %s", s.device.UDID, err))
				return
			}
		}
	}
}

// watchOrientationChanges monitors for orientation changes and signals the client to reconnect
func (s *WebRTCSession) watchOrientationChanges() {
	orientationChan := s.extractor.GetOrientationChangeChannel()

	for {
		select {
		case <-s.ctx.Done():
			return
		case _, ok := <-orientationChan:
			if !ok {
				return
			}

			logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Orientation changed for device %s, signaling client to reconnect", s.device.UDID))

			// Signal the client to reconnect via callback
			if s.onOrientationChangeFunc != nil {
				s.onOrientationChangeFunc()
			}
		}
	}
}

// OnOrientationChange sets callback for orientation changes
func (s *WebRTCSession) OnOrientationChange(handler func()) {
	s.onOrientationChangeFunc = handler
}

// HandleOffer processes SDP offer from client
func (s *WebRTCSession) HandleOffer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.peerConnection.SetRemoteDescription(offer); err != nil {
		return nil, fmt.Errorf("failed to set remote description: %w", err)
	}

	// Create answer
	answer, err := s.peerConnection.CreateAnswer(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create answer: %w", err)
	}

	if err := s.peerConnection.SetLocalDescription(answer); err != nil {
		return nil, fmt.Errorf("failed to set local description: %w", err)
	}

	// Add any pending ICE candidates
	for _, candidate := range s.iceCandidates {
		if err := s.peerConnection.AddICECandidate(candidate); err != nil {
			logger.ProviderLogger.LogWarn("stream_webrtc", fmt.Sprintf("Failed to add ICE candidate for device %s: %s", s.device.UDID, err))
		}
	}
	s.iceCandidates = nil

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Created answer for device %s", s.device.UDID))
	return &answer, nil
}

// AddICECandidate adds an ICE candidate to the peer connection
func (s *WebRTCSession) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.peerConnection.RemoteDescription() == nil {
		// Queue candidate until remote description is set
		s.iceCandidates = append(s.iceCandidates, candidate)
		return nil
	}

	return s.peerConnection.AddICECandidate(candidate)
}

// OnICECandidate sets callback for ICE candidates
func (s *WebRTCSession) OnICECandidate(handler func(*webrtc.ICECandidate)) {
	s.peerConnection.OnICECandidate(handler)
}

// OnConnectionStateChange sets callback for connection state changes
func (s *WebRTCSession) OnConnectionStateChange(handler func(webrtc.PeerConnectionState)) {
	s.peerConnection.OnConnectionStateChange(handler)
}

// Close cleans up all resources
func (s *WebRTCSession) Close() {
	s.cancel()

	if s.encoder != nil {
		s.encoder.Close()
	}

	if s.extractor != nil {
		s.extractor.Close()
	}

	if s.peerConnection != nil {
		s.peerConnection.Close()
	}

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Closed WebRTC session for device %s", s.device.UDID))
}

// WebRTCSignalingMessage represents WebRTC signaling messages
type WebRTCSignalingMessage struct {
	Type      string                   `json:"type"`
	SDP       string                   `json:"sdp,omitempty"`
	Candidate *webrtc.ICECandidateInit `json:"candidate,omitempty"`
}

// IOSWebRTCSocket handles WebRTC signaling for iOS devices
func IOSWebRTCSocket(c *gin.Context) {
	udid := c.Param("udid")

	dev, ok := manager.Instance.GetDevice(udid)
	if !ok {
		logger.ProviderLogger.LogError("ios_webrtc", fmt.Sprintf("Device with UDID `%s` not found", udid))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Upgrade to WebSocket
	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		logger.ProviderLogger.LogError("ios_webrtc", fmt.Sprintf("Failed to upgrade connection to websocket for device `%s` - %s", udid, err))
		return
	}
	defer conn.Close()

	// Create WebRTC session
	session, err := NewWebRTCSession(dev.Info())
	if err != nil {
		logger.ProviderLogger.LogError("ios_webrtc", fmt.Sprintf("Failed to create WebRTC session for device `%s` - %s", udid, err))
		wsutil.WriteServerText(conn, []byte(`{"type":"error","message":"Failed to create WebRTC session"}`))
		return
	}
	defer session.Close()

	// Start streaming pipeline
	if err := session.Start(); err != nil {
		logger.ProviderLogger.LogError("ios_webrtc", fmt.Sprintf("Failed to start streaming pipeline for device `%s` - %s", udid, err))
		wsutil.WriteServerText(conn, []byte(`{"type":"error","message":"Failed to start streaming"}`))
		return
	}

	// Handle ICE candidates
	session.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		candidateJSON := candidate.ToJSON()
		msg := WebRTCSignalingMessage{
			Type:      "candidate",
			Candidate: &candidateJSON,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			logger.ProviderLogger.LogError("ios_webrtc", fmt.Sprintf("Failed to marshal ICE candidate for device %s: %s", udid, err))
			return
		}

		if err := wsutil.WriteServerText(conn, data); err != nil {
			logger.ProviderLogger.LogError("ios_webrtc", fmt.Sprintf("Failed to send ICE candidate for device %s: %s", udid, err))
		}
	})

	// Handle connection state changes
	session.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		logger.ProviderLogger.LogInfo("ios_webrtc", fmt.Sprintf("WebRTC connection state for device %s: %s", udid, state.String()))

		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateDisconnected {
			logger.ProviderLogger.LogInfo("ios_webrtc", fmt.Sprintf("WebRTC connection ended for device %s, closing websocket", udid))
			conn.Close()
		}
	})

	// Handle orientation changes - signal client to reconnect
	session.OnOrientationChange(func() {
		msg := `{"type":"orientation_changed"}`
		if err := wsutil.WriteServerText(conn, []byte(msg)); err != nil {
			logger.ProviderLogger.LogError("ios_webrtc", fmt.Sprintf("Failed to send orientation change for device %s: %s", udid, err))
		}
	})

	logger.ProviderLogger.LogInfo("ios_webrtc", fmt.Sprintf("WebRTC signaling established for device `%s`", udid))

	// Handle signaling messages
	for {
		msg, _, err := wsutil.ReadClientData(conn)
		if err != nil {
			logger.ProviderLogger.LogDebug("ios_webrtc", fmt.Sprintf("Client WebRTC websocket connection for device `%s` closed - %s", udid, err))
			return
		}

		var signalingMsg WebRTCSignalingMessage
		if err := json.Unmarshal(msg, &signalingMsg); err != nil {
			logger.ProviderLogger.LogError("ios_webrtc", fmt.Sprintf("Failed to unmarshal signaling message for device `%s` - %s", udid, err))
			continue
		}

		switch signalingMsg.Type {
		case "offer":
			offer := webrtc.SessionDescription{
				Type: webrtc.SDPTypeOffer,
				SDP:  signalingMsg.SDP,
			}

			answer, err := session.HandleOffer(offer)
			if err != nil {
				logger.ProviderLogger.LogError("ios_webrtc", fmt.Sprintf("Failed to handle offer for device `%s` - %s", udid, err))
				wsutil.WriteServerText(conn, []byte(`{"type":"error","message":"Failed to handle offer"}`))
				return
			}

			response := WebRTCSignalingMessage{
				Type: "answer",
				SDP:  answer.SDP,
			}

			data, err := json.Marshal(response)
			if err != nil {
				logger.ProviderLogger.LogError("ios_webrtc", fmt.Sprintf("Failed to marshal answer for device `%s` - %s", udid, err))
				return
			}

			if err := wsutil.WriteServerText(conn, data); err != nil {
				logger.ProviderLogger.LogError("ios_webrtc", fmt.Sprintf("Failed to send answer for device `%s` - %s", udid, err))
				return
			}

			logger.ProviderLogger.LogInfo("ios_webrtc", fmt.Sprintf("Sent answer to client for device %s", udid))

		case "candidate":
			if signalingMsg.Candidate != nil {
				if err := session.AddICECandidate(*signalingMsg.Candidate); err != nil {
					logger.ProviderLogger.LogWarn("ios_webrtc", fmt.Sprintf("Failed to add ICE candidate for device `%s` - %s", udid, err))
				}
			}

		case "hangup":
			logger.ProviderLogger.LogInfo("ios_webrtc", fmt.Sprintf("Received hangup for device `%s`", udid))
			return

		default:
			logger.ProviderLogger.LogWarn("ios_webrtc", fmt.Sprintf("Unknown signaling message type `%s` for device `%s`", signalingMsg.Type, udid))
		}
	}
}
