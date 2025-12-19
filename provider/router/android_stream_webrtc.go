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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

// AndroidH264Extractor handles extracting H.264 frames from Android WebSocket stream
type AndroidH264Extractor struct {
	device      *models.Device
	conn        io.ReadWriteCloser
	h264Channel chan []byte
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewAndroidH264Extractor creates a new H.264 extractor for Android WebSocket stream
func NewAndroidH264Extractor(device *models.Device) (*AndroidH264Extractor, error) {
	ctx, cancel := context.WithCancel(context.Background())

	extractor := &AndroidH264Extractor{
		device:      device,
		h264Channel: make(chan []byte, 10), // Buffer for H.264 frames
		ctx:         ctx,
		cancel:      cancel,
	}

	// Connect to Android stream WebSocket
	streamURL := "ws://localhost:" + device.StreamPort

	// Dial WebSocket connection
	conn, _, _, err := ws.Dial(ctx, streamURL)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Android stream: %w", err)
	}

	extractor.conn = conn

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Connected to Android H.264 stream for device %s (Android sends SPS/PPS with keyframes)", device.UDID))

	// Start reading frames from WebSocket
	go extractor.extractH264Frames()

	return extractor, nil
}

// Start begins extracting H.264 frames from the WebSocket stream (deprecated, now auto-started)
func (e *AndroidH264Extractor) Start() {
	// Now a no-op since we start in constructor
}

// extractNALUnits splits H.264 data by Annex-B start codes (0x00 0x00 0x00 0x01)
func extractNALUnits(data []byte) [][]byte {
	var nalUnits [][]byte

	// Find all start code positions
	var positions []int
	for i := 0; i <= len(data)-4; i++ {
		if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x00 && data[i+3] == 0x01 {
			positions = append(positions, i)
		}
	}

	// Extract NAL units between start codes
	for i := 0; i < len(positions); i++ {
		start := positions[i]
		var end int
		if i+1 < len(positions) {
			end = positions[i+1]
		} else {
			end = len(data)
		}
		nalUnits = append(nalUnits, data[start:end])
	}

	return nalUnits
}

// extractH264Frames reads H.264 frames from WebSocket and sends to channel
// Android sends complete frames with SPS/PPS prepended to keyframes
func (e *AndroidH264Extractor) extractH264Frames() {
	defer close(e.h264Channel)
	defer e.conn.Close()

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Starting H.264 frame extraction from WebSocket for device %s", e.device.UDID))

	frameCount := 0

	for {
		select {
		case <-e.ctx.Done():
			logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Stopping H.264 extraction for device %s", e.device.UDID))
			return
		default:
			// Read H.264 frame from WebSocket (same as AndroidStreamMJPEG)
			msg, _, err := wsutil.ReadServerData(e.conn)
			if err != nil {
				if err != io.EOF {
					logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Error reading H.264 from WebSocket for device %s: %s", e.device.UDID, err))
				}
				return
			}

			if len(msg) == 0 {
				continue
			}

			frameCount++

			// Log every 30 frames
			if frameCount%30 == 0 {
				logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Received frame #%d (%d bytes) from Android for device %s", frameCount, len(msg), e.device.UDID))
			}

			// Send H.264 frame to channel (non-blocking)
			select {
			case e.h264Channel <- msg:
			case <-e.ctx.Done():
				return
			default:
				// Drop frame if channel is full (backpressure)
				logger.ProviderLogger.LogWarn("stream_webrtc", fmt.Sprintf("Dropped frame #%d for device %s (channel full)", frameCount, e.device.UDID))
			}
		}
	}
}

// GetH264Channel returns the channel that receives H.264 frames
func (e *AndroidH264Extractor) GetH264Channel() <-chan []byte {
	return e.h264Channel
}

// Close stops the extractor and cleans up resources
func (e *AndroidH264Extractor) Close() {
	e.cancel()
	if e.conn != nil {
		e.conn.Close()
	}
}

// AndroidWebRTCSession manages a WebRTC peer connection for Android streaming
type AndroidWebRTCSession struct {
	device         *models.Device
	peerConnection *webrtc.PeerConnection
	videoTrack     *webrtc.TrackLocalStaticSample
	extractor      *AndroidH264Extractor
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.Mutex
	iceCandidates  []webrtc.ICECandidateInit
}

// NewAndroidWebRTCSession creates a new WebRTC session for Android device streaming
func NewAndroidWebRTCSession(device *models.Device) (*AndroidWebRTCSession, error) {
	ctx, cancel := context.WithCancel(context.Background())

	session := &AndroidWebRTCSession{
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

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Created Android WebRTC session for device %s", device.UDID))

	return session, nil
}

// Start begins the streaming pipeline
func (s *AndroidWebRTCSession) Start() error {
	// Create H.264 extractor
	extractor, err := NewAndroidH264Extractor(s.device)
	if err != nil {
		return fmt.Errorf("failed to create H.264 extractor: %w", err)
	}
	s.extractor = extractor
	extractor.Start()

	// Start writing H.264 to WebRTC track
	go s.writeH264ToTrack()

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Started Android streaming pipeline for device %s", s.device.UDID))
	return nil
}

// writeH264ToTrack reads H.264 data and writes to WebRTC video track
// Android sends SPS/PPS before every keyframe, so we just pass through all NAL units
func (s *AndroidWebRTCSession) writeH264ToTrack() {
	h264Channel := s.extractor.GetH264Channel()

	// Calculate frame duration based on device settings (default 30fps)
	fps := 60
	if s.device.StreamTargetFPS > 0 {
		fps = s.device.StreamTargetFPS
	}
	frameDuration := time.Second / time.Duration(fps)

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Starting H.264 streaming for device %s (Android handles SPS/PPS with keyframes)", s.device.UDID))

	frameCount := 0

	// Simple pass-through: Android sends complete frames with SPS/PPS before keyframes
	for {
		select {
		case <-s.ctx.Done():
			return
		case h264Data, ok := <-h264Channel:
			if !ok {
				logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("H.264 channel closed for device %s", s.device.UDID))
				return
			}

			frameCount++

			// Split frame into individual NAL units (Android sends properly formatted Annex-B)
			nalUnits := extractNALUnits(h264Data)

			if len(nalUnits) == 0 {
				continue
			}

			// Send each NAL unit as a separate RTP packet
			for _, nalUnit := range nalUnits {
				if len(nalUnit) < 5 {
					continue
				}

				// Write NAL unit to WebRTC track
				if err := s.videoTrack.WriteSample(media.Sample{
					Data:     nalUnit,
					Duration: frameDuration,
				}); err != nil {
					logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Failed to write NAL unit to track for device %s: %s", s.device.UDID, err))
					return
				}

				// Log first frame for debugging
				if frameCount == 1 {
					nalType := nalUnit[4] & 0x1F
					logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Frame #1 NAL type=%d, size=%d bytes for device %s", nalType, len(nalUnit), s.device.UDID))
				}
			}

			// Log every 30 frames
			if frameCount%30 == 0 {
				logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Sent frame #%d (%d NAL units, %d bytes) to WebRTC for device %s", frameCount, len(nalUnits), len(h264Data), s.device.UDID))
			}
		}
	}
}

// HandleOffer processes SDP offer from client
func (s *AndroidWebRTCSession) HandleOffer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
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

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Created answer for Android device %s", s.device.UDID))
	return &answer, nil
}

// AddICECandidate adds an ICE candidate to the peer connection
func (s *AndroidWebRTCSession) AddICECandidate(candidate webrtc.ICECandidateInit) error {
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
func (s *AndroidWebRTCSession) OnICECandidate(handler func(*webrtc.ICECandidate)) {
	s.peerConnection.OnICECandidate(handler)
}

// OnConnectionStateChange sets callback for connection state changes
func (s *AndroidWebRTCSession) OnConnectionStateChange(handler func(webrtc.PeerConnectionState)) {
	s.peerConnection.OnConnectionStateChange(handler)
}

// Close cleans up all resources
func (s *AndroidWebRTCSession) Close() {
	s.cancel()

	if s.extractor != nil {
		s.extractor.Close()
	}

	if s.peerConnection != nil {
		s.peerConnection.Close()
	}

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Closed Android WebRTC session for device %s", s.device.UDID))
}

// AndroidWebRTCSocket handles WebRTC signaling for Android devices
func AndroidWebRTCSocket(c *gin.Context) {
	udid := c.Param("udid")

	device, ok := devices.DBDeviceMap[udid]
	if !ok || device == nil {
		logger.ProviderLogger.LogError("android_webrtc", fmt.Sprintf("Device with UDID `%s` not found or is nil", udid))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Upgrade to WebSocket
	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		logger.ProviderLogger.LogError("android_webrtc", fmt.Sprintf("Failed to upgrade connection to websocket for device `%s` - %s", udid, err))
		return
	}
	defer conn.Close()

	// Create WebRTC session
	session, err := NewAndroidWebRTCSession(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_webrtc", fmt.Sprintf("Failed to create WebRTC session for device `%s` - %s", udid, err))
		wsutil.WriteServerText(conn, []byte(`{"type":"error","message":"Failed to create WebRTC session"}`))
		return
	}
	defer session.Close()

	// Start streaming pipeline
	if err := session.Start(); err != nil {
		logger.ProviderLogger.LogError("android_webrtc", fmt.Sprintf("Failed to start streaming pipeline for device `%s` - %s", udid, err))
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
			logger.ProviderLogger.LogError("android_webrtc", fmt.Sprintf("Failed to marshal ICE candidate for device %s: %s", udid, err))
			return
		}

		if err := wsutil.WriteServerText(conn, data); err != nil {
			logger.ProviderLogger.LogError("android_webrtc", fmt.Sprintf("Failed to send ICE candidate for device %s: %s", udid, err))
		}
	})

	// Handle connection state changes
	session.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		logger.ProviderLogger.LogInfo("android_webrtc", fmt.Sprintf("WebRTC connection state for device %s: %s", udid, state.String()))

		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			conn.Close()
		}
	})

	logger.ProviderLogger.LogInfo("android_webrtc", fmt.Sprintf("WebRTC signaling established for device `%s`", udid))

	// Handle signaling messages
	for {
		msg, _, err := wsutil.ReadClientData(conn)
		if err != nil {
			logger.ProviderLogger.LogDebug("android_webrtc", fmt.Sprintf("Client WebRTC websocket connection for device `%s` closed - %s", udid, err))
			return
		}

		var signalingMsg WebRTCSignalingMessage
		if err := json.Unmarshal(msg, &signalingMsg); err != nil {
			logger.ProviderLogger.LogError("android_webrtc", fmt.Sprintf("Failed to unmarshal signaling message for device `%s` - %s", udid, err))
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
				logger.ProviderLogger.LogError("android_webrtc", fmt.Sprintf("Failed to handle offer for device `%s` - %s", udid, err))
				wsutil.WriteServerText(conn, []byte(`{"type":"error","message":"Failed to handle offer"}`))
				return
			}

			response := WebRTCSignalingMessage{
				Type: "answer",
				SDP:  answer.SDP,
			}

			data, err := json.Marshal(response)
			if err != nil {
				logger.ProviderLogger.LogError("android_webrtc", fmt.Sprintf("Failed to marshal answer for device `%s` - %s", udid, err))
				return
			}

			if err := wsutil.WriteServerText(conn, data); err != nil {
				logger.ProviderLogger.LogError("android_webrtc", fmt.Sprintf("Failed to send answer for device `%s` - %s", udid, err))
				return
			}

			logger.ProviderLogger.LogInfo("android_webrtc", fmt.Sprintf("Sent answer to client for device %s", udid))

		case "candidate":
			if signalingMsg.Candidate != nil {
				if err := session.AddICECandidate(*signalingMsg.Candidate); err != nil {
					logger.ProviderLogger.LogWarn("android_webrtc", fmt.Sprintf("Failed to add ICE candidate for device `%s` - %s", udid, err))
				}
			}

		case "hangup":
			logger.ProviderLogger.LogInfo("android_webrtc", fmt.Sprintf("Received hangup for device `%s`", udid))
			return

		default:
			logger.ProviderLogger.LogWarn("android_webrtc", fmt.Sprintf("Unknown signaling message type `%s` for device `%s`", signalingMsg.Type, udid))
		}
	}
}
