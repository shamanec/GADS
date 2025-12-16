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
	"GADS/common/models"
	"GADS/provider/devices"
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

// WDAJPEGExtractor handles extracting JPEG frames from WebDriverAgent MJPEG stream
type WDAJPEGExtractor struct {
	device       *models.Device
	httpResp     *http.Response
	multiReader  *multipart.Reader
	jpegChannel  chan []byte
	ctx          context.Context
	cancel       context.CancelFunc
	ffmpegCmd    *exec.Cmd
	ffmpegStdin  io.WriteCloser
	ffmpegStdout io.ReadCloser
}

// NewWDAJPEGExtractor creates a new JPEG extractor for WebDriverAgent stream
func NewWDAJPEGExtractor(device *models.Device) (*WDAJPEGExtractor, error) {
	ctx, cancel := context.WithCancel(context.Background())

	extractor := &WDAJPEGExtractor{
		device:      device,
		jpegChannel: make(chan []byte, 30), // Buffer for 30 frames
		ctx:         ctx,
		cancel:      cancel,
	}

	// Connect to WDA MJPEG stream
	streamUrl := "http://localhost:" + device.WDAStreamPort
	req, err := http.NewRequestWithContext(ctx, "GET", streamUrl, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to WDA stream: %w", err)
	}

	extractor.httpResp = resp

	// Parse multipart boundary
	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		cancel()
		resp.Body.Close()
		return nil, fmt.Errorf("failed to parse content-type: %w", err)
	}

	if !strings.HasPrefix(mediaType, "multipart/") {
		cancel()
		resp.Body.Close()
		return nil, fmt.Errorf("invalid media type: %s", mediaType)
	}

	// Clean boundary string (remove leading --)
	boundary := strings.Replace(params["boundary"], "--", "", -1)
	extractor.multiReader = multipart.NewReader(resp.Body, boundary)

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Connected to WDA MJPEG stream for device %s", device.UDID))

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

// Close stops the extractor and cleans up resources
func (e *WDAJPEGExtractor) Close() {
	e.cancel()
	if e.httpResp != nil && e.httpResp.Body != nil {
		e.httpResp.Body.Close()
	}
}

// FFmpegH264Encoder handles encoding JPEG frames to H.264
type FFmpegH264Encoder struct {
	device       *models.Device
	jpegInput    <-chan []byte
	h264Output   chan []byte
	ctx          context.Context
	cancel       context.CancelFunc
	ffmpegCmd    *exec.Cmd
	ffmpegStdin  io.WriteCloser
	ffmpegStdout io.ReadCloser
}

// NewFFmpegH264Encoder creates a new H.264 encoder using FFmpeg
func NewFFmpegH264Encoder(device *models.Device, jpegInput <-chan []byte) (*FFmpegH264Encoder, error) {
	ctx, cancel := context.WithCancel(context.Background())

	encoder := &FFmpegH264Encoder{
		device:     device,
		jpegInput:  jpegInput,
		h264Output: make(chan []byte, 30),
		ctx:        ctx,
		cancel:     cancel,
	}

	// Create FFmpeg command for JPEG to H.264 encoding
	// -f image2pipe: input is a stream of images
	// -framerate 30: assume 30fps input (adjust based on device settings)
	// -i -: read from stdin
	// -c:v libx264: use H.264 encoder
	// -preset ultrafast: minimize encoding latency
	// -tune zerolatency: optimize for low-latency streaming
	// -pix_fmt yuv420p: compatible pixel format
	// -g 60: keyframe every 2 seconds (more frequent for better seeking/recovery)
	// -keyint_min 30: minimum keyframe interval
	// -x264opts: additional x264 options for better streaming
	//   - no-scenecut: disable scene change detection (more predictable keyframes)
	//   - bframes=0: no B-frames for lower latency
	// -bf 0: no B-frames
	// -refs 1: single reference frame for speed
	// -profile:v baseline: most compatible profile
	// -level 3.1: compatibility level
	// -f h264: output raw H.264 stream
	// -: write to stdout
	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-f", "image2pipe",
		"-framerate", "30",
		"-i", "-",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-pix_fmt", "yuv420p",
		"-g", "60",
		"-keyint_min", "30",
		"-x264opts", "no-scenecut:bframes=0",
		"-bf", "0",
		"-refs", "1",
		"-profile:v", "baseline",
		"-level", "3.1",
		"-f", "h264",
		"-",
	)

	// Get stdin pipe for writing JPEGs
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	encoder.ffmpegStdin = stdin

	// Get stdout pipe for reading H.264 stream
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	encoder.ffmpegStdout = stdout

	// Start FFmpeg process
	if err := cmd.Start(); err != nil {
		cancel()
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	encoder.ffmpegCmd = cmd
	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Started FFmpeg H.264 encoder for device %s", device.UDID))

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
	e.cancel()
	if e.ffmpegStdin != nil {
		e.ffmpegStdin.Close()
	}
	if e.ffmpegStdout != nil {
		e.ffmpegStdout.Close()
	}
	if e.ffmpegCmd != nil && e.ffmpegCmd.Process != nil {
		e.ffmpegCmd.Process.Kill()
	}
}

// WebRTCSession manages a single WebRTC peer connection for streaming
type WebRTCSession struct {
	device          *models.Device
	peerConnection  *webrtc.PeerConnection
	videoTrack      *webrtc.TrackLocalStaticSample
	extractor       *WDAJPEGExtractor
	encoder         *FFmpegH264Encoder
	ctx             context.Context
	cancel          context.CancelFunc
	mu              sync.Mutex
	iceCandidates   []webrtc.ICECandidateInit
	pendingOffer    *webrtc.SessionDescription
}

// NewWebRTCSession creates a new WebRTC session for device streaming
func NewWebRTCSession(device *models.Device) (*WebRTCSession, error) {
	ctx, cancel := context.WithCancel(context.Background())

	session := &WebRTCSession{
		device:        device,
		ctx:           ctx,
		cancel:        cancel,
		iceCandidates: make([]webrtc.ICECandidateInit, 0),
	}

	// Create WebRTC configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create peer connection
	pc, err := webrtc.NewPeerConnection(config)
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
	Type      string                     `json:"type"`
	SDP       string                     `json:"sdp,omitempty"`
	Candidate *webrtc.ICECandidateInit   `json:"candidate,omitempty"`
}

// IOSWebRTCSocket handles WebRTC signaling for iOS devices
func IOSWebRTCSocket(c *gin.Context) {
	udid := c.Param("udid")

	device, ok := devices.DBDeviceMap[udid]
	if !ok || device == nil {
		logger.ProviderLogger.LogError("ios_webrtc", fmt.Sprintf("Device with UDID `%s` not found or is nil", udid))
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
	session, err := NewWebRTCSession(device)
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

		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			conn.Close()
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
