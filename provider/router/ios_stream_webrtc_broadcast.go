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
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

// IOSH264Extractor handles extracting H.264 frames from iOS broadcast extension TCP stream
type IOSH264Extractor struct {
	device      *models.Device
	conn        net.Conn
	h264Channel chan H264Frame
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewIOSH264Extractor creates a new H.264 extractor for iOS broadcast extension TCP stream
func NewIOSH264Extractor(device *models.Device) (*IOSH264Extractor, error) {
	ctx, cancel := context.WithCancel(context.Background())

	extractor := &IOSH264Extractor{
		device:      device,
		h264Channel: make(chan H264Frame, 30), // Buffer for H.264 frames with timestamps
		ctx:         ctx,
		cancel:      cancel,
	}

	// Connect to iOS broadcast extension TCP server
	broadcastServer := "localhost:" + device.StreamPort

	// Dial TCP connection
	conn, err := net.Dial("tcp", broadcastServer)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to iOS broadcast extension: %w", err)
	}

	extractor.conn = conn

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Connected to iOS broadcast H.264 stream for device %s (iOS sends SPS/PPS with keyframes)", device.UDID))

	// Start reading frames from TCP stream
	go extractor.extractH264Frames()

	return extractor, nil
}

// Start begins extracting H.264 frames from the TCP stream (deprecated, now auto-started)
func (e *IOSH264Extractor) Start() {
	// Now a no-op since we start in constructor
}

// extractH264Frames reads H.264 frames from TCP stream with iOS broadcast extension framing
// Message format: [4 bytes length][8 bytes timestamp][H.264 payload]
func (e *IOSH264Extractor) extractH264Frames() {
	defer close(e.h264Channel)
	defer e.conn.Close()

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Starting H.264 frame extraction from TCP for device %s", e.device.UDID))

	frameCount := 0
	buffer := make([]byte, 0, 1024*1024) // 1MB buffer for accumulating data

	for {
		select {
		case <-e.ctx.Done():
			logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Stopping H.264 extraction for device %s", e.device.UDID))
			return
		default:
			// Read data from TCP connection
			readBuf := make([]byte, 65536) // 64KB read buffer
			n, err := e.conn.Read(readBuf)
			if err != nil {
				if err != io.EOF {
					logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Error reading H.264 from TCP for device %s: %s", e.device.UDID, err))
				}
				return
			}

			// Append to accumulation buffer
			buffer = append(buffer, readBuf[:n]...)

			// Process complete messages
			// Message format: [4 bytes length][8 bytes timestamp][payload]
			for len(buffer) >= 12 {
				// Read payload length (first 4 bytes, big-endian)
				payloadLength := binary.BigEndian.Uint32(buffer[0:4])
				messageLength := 4 + 8 + int(payloadLength)

				// Check if we have complete message
				if len(buffer) < messageLength {
					break
				}

				// Extract timestamp (next 8 bytes, big-endian, in microseconds)
				timestamp := int64(binary.BigEndian.Uint64(buffer[4:12]))

				// Extract H.264 payload
				h264Data := make([]byte, payloadLength)
				copy(h264Data, buffer[12:messageLength])

				frameCount++

				// Log every 30 frames
				if frameCount%30 == 0 {
					logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Received frame #%d (PTS=%d, %d bytes) from iOS for device %s", frameCount, timestamp, len(h264Data), e.device.UDID))
				}

				// Send H.264 frame with timestamp to channel (non-blocking)
				frame := H264Frame{
					Data: h264Data,
					PTS:  timestamp,
				}
				select {
				case e.h264Channel <- frame:
				case <-e.ctx.Done():
					return
				default:
					// Drop frame if channel is full (backpressure)
					logger.ProviderLogger.LogWarn("stream_webrtc", fmt.Sprintf("Dropped frame #%d for device %s (channel full)", frameCount, e.device.UDID))
				}

				// Remove processed message from buffer
				buffer = buffer[messageLength:]
			}
		}
	}
}

// GetH264Channel returns the channel that receives H.264 frames
func (e *IOSH264Extractor) GetH264Channel() <-chan H264Frame {
	return e.h264Channel
}

// Close stops the extractor and cleans up resources
func (e *IOSH264Extractor) Close() {
	e.cancel()
	if e.conn != nil {
		e.conn.Close()
	}
}

// IOSWebRTCSession manages a WebRTC peer connection for iOS broadcast streaming
type IOSWebRTCSession struct {
	device         *models.Device
	peerConnection *webrtc.PeerConnection
	videoTrack     *webrtc.TrackLocalStaticSample
	extractor      *IOSH264Extractor
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.Mutex
	iceCandidates  []webrtc.ICECandidateInit
}

// NewIOSWebRTCSession creates a new WebRTC session for iOS broadcast streaming
func NewIOSWebRTCSession(device *models.Device) (*IOSWebRTCSession, error) {
	ctx, cancel := context.WithCancel(context.Background())

	session := &IOSWebRTCSession{
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
		"gads-ios-stream",
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

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Created iOS WebRTC session for device %s", device.UDID))

	return session, nil
}

// Start begins the streaming pipeline
func (s *IOSWebRTCSession) Start() error {
	// Create H.264 extractor
	extractor, err := NewIOSH264Extractor(s.device)
	if err != nil {
		return fmt.Errorf("failed to create H.264 extractor: %w", err)
	}
	s.extractor = extractor
	extractor.Start()

	// Start writing H.264 to WebRTC track
	go s.writeH264ToTrack()

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Started iOS streaming pipeline for device %s", s.device.UDID))
	return nil
}

// writeH264ToTrack reads H.264 data and writes to WebRTC video track
// Uses presentation timestamps from iOS broadcast extension for accurate frame timing
func (s *IOSWebRTCSession) writeH264ToTrack() {
	h264Channel := s.extractor.GetH264Channel()

	// Fallback duration if PTS calculation fails
	fps := 30
	if s.device.StreamTargetFPS > 0 {
		fps = s.device.StreamTargetFPS
	}
	fallbackDuration := time.Second / time.Duration(fps)

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Starting H.264 streaming for device %s (using iOS presentation timestamps)", s.device.UDID))

	frameCount := 0
	var previousPTS int64 = 0

	for {
		select {
		case <-s.ctx.Done():
			return
		case frame, ok := <-h264Channel:
			if !ok {
				logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("H.264 channel closed for device %s", s.device.UDID))
				return
			}

			frameCount++

			// Skip invalid frames
			if len(frame.Data) < 5 {
				continue
			}

			// Calculate frame duration from presentation timestamp delta
			var frameDuration time.Duration
			if frameCount == 1 {
				// First frame - use fallback duration
				frameDuration = fallbackDuration
				previousPTS = frame.PTS
			} else {
				// Calculate actual duration from PTS delta (PTS is in microseconds)
				ptsDelta := frame.PTS - previousPTS
				if ptsDelta > 0 && ptsDelta < 1000000 { // Sanity check: less than 1 second
					frameDuration = time.Duration(ptsDelta) * time.Microsecond
				} else {
					// Timestamp went backwards or is invalid - use fallback
					if ptsDelta <= 0 || ptsDelta >= 1000000 {
						logger.ProviderLogger.LogWarn("stream_webrtc", fmt.Sprintf("Invalid PTS delta for frame #%d (current=%d, prev=%d), using fallback duration", frameCount, frame.PTS, previousPTS))
					}
					frameDuration = fallbackDuration
				}
				previousPTS = frame.PTS
			}

			// Write complete frame to WebRTC track with accurate timing
			if err := s.videoTrack.WriteSample(media.Sample{
				Data:     frame.Data,
				Duration: frameDuration,
			}); err != nil {
				logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Failed to write frame to track for device %s: %s", s.device.UDID, err))
				return
			}

			// Log first frame for debugging
			if frameCount == 1 {
				// Split to analyze NAL types
				nalUnits := extractNALUnits(frame.Data)
				nalTypes := ""
				for i, nalUnit := range nalUnits {
					if len(nalUnit) >= 5 {
						nalType := nalUnit[4] & 0x1F
						if i > 0 {
							nalTypes += ", "
						}
						nalTypes += fmt.Sprintf("%d", nalType)
					}
				}
				logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Frame #1 PTS=%d, NAL types=[%s], size=%d bytes, duration=%v for device %s", frame.PTS, nalTypes, len(frame.Data), frameDuration, s.device.UDID))
			}

			// Log every 30 frames
			if frameCount%30 == 0 {
				logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Sent frame #%d (PTS=%d, %d bytes, duration=%v) to WebRTC for device %s", frameCount, frame.PTS, len(frame.Data), frameDuration, s.device.UDID))
			}
		}
	}
}

// HandleOffer processes SDP offer from client
func (s *IOSWebRTCSession) HandleOffer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
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

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Created answer for iOS device %s", s.device.UDID))
	return &answer, nil
}

// AddICECandidate adds an ICE candidate to the peer connection
func (s *IOSWebRTCSession) AddICECandidate(candidate webrtc.ICECandidateInit) error {
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
func (s *IOSWebRTCSession) OnICECandidate(handler func(*webrtc.ICECandidate)) {
	s.peerConnection.OnICECandidate(handler)
}

// OnConnectionStateChange sets callback for connection state changes
func (s *IOSWebRTCSession) OnConnectionStateChange(handler func(webrtc.PeerConnectionState)) {
	s.peerConnection.OnConnectionStateChange(handler)
}

// Close cleans up all resources
func (s *IOSWebRTCSession) Close() {
	s.cancel()

	if s.extractor != nil {
		s.extractor.Close()
	}

	if s.peerConnection != nil {
		s.peerConnection.Close()
	}

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Closed iOS WebRTC session for device %s", s.device.UDID))
}

// IOSBroadcastWebRTCSocket handles WebRTC signaling for iOS broadcast extension streaming
func IOSBroadcastWebRTCSocket(c *gin.Context) {
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
	session, err := NewIOSWebRTCSession(device)
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
		msg := struct {
			Type      string                   `json:"type"`
			Candidate *webrtc.ICECandidateInit `json:"candidate"`
		}{
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

		var signalingMsg struct {
			Type      string                   `json:"type"`
			SDP       string                   `json:"sdp,omitempty"`
			Candidate *webrtc.ICECandidateInit `json:"candidate,omitempty"`
		}
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

			response := struct {
				Type string `json:"type"`
				SDP  string `json:"sdp"`
			}{
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
