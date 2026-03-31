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
	"GADS/common/utils"
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

// IOSWebRTCSession manages a WebRTC peer connection for iOS broadcast streaming
type IOSWebRTCSession struct {
	device          *models.DBDevice
	streamPort      string
	streamTargetFPS int
	peerConnection  *webrtc.PeerConnection
	videoTrack      *webrtc.TrackLocalStaticSample
	tcpConn         net.Conn
	ctx             context.Context
	cancel          context.CancelFunc
	mu              sync.Mutex
	iceCandidates   []webrtc.ICECandidateInit

	// Timestamp tracking
	firstTimestamp uint64
	lastTimestamp  uint64
	frameCount     int
}

// NewIOSWebRTCSession creates a new WebRTC session for iOS broadcast streaming
func NewIOSWebRTCSession(device *models.DBDevice, streamPort string, streamTargetFPS int) (*IOSWebRTCSession, error) {
	ctx, cancel := context.WithCancel(context.Background())

	session := &IOSWebRTCSession{
		device:          device,
		streamPort:      streamPort,
		streamTargetFPS: streamTargetFPS,
		ctx:             ctx,
		cancel:          cancel,
		iceCandidates:   make([]webrtc.ICECandidateInit, 0),
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
	// Connect to iOS broadcast extension TCP server
	broadcastServer := "localhost:" + s.streamPort

	conn, err := net.Dial("tcp", broadcastServer)
	if err != nil {
		return fmt.Errorf("failed to connect to iOS broadcast extension: %w", err)
	}
	s.tcpConn = conn

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Connected to iOS broadcast H.264 stream for device %s", s.device.UDID))

	// Start reading and processing frames
	go s.readAndStreamFrames()

	return nil
}

// readAndStreamFrames reads H.264 frames from TCP and writes directly to WebRTC track
func (s *IOSWebRTCSession) readAndStreamFrames() {
	defer s.tcpConn.Close()

	buffer := make([]byte, 0, 1024*1024)                // 1MB buffer
	fallbackDuration := time.Second / time.Duration(30) // Default 30fps
	if s.streamTargetFPS > 0 {
		fallbackDuration = time.Second / time.Duration(s.streamTargetFPS)
	}

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Starting H.264 streaming for device %s", s.device.UDID))

	readBuf := make([]byte, 65536)
	for {
		// Check for cancellation
		select {
		case <-s.ctx.Done():
			logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Stopping H.264 streaming for device %s", s.device.UDID))
			return
		default:
		}

		// Read data from TCP connection
		n, err := s.tcpConn.Read(readBuf)
		if err != nil {
			if err != io.EOF {
				logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Error reading from TCP for device %s: %s", s.device.UDID, err))
			}
			return
		}

		buffer = append(buffer, readBuf[:n]...)

		// Process complete messages
		// Message format: [4 bytes length][8 bytes timestamp][payload]
		for len(buffer) >= 12 {
			// Read payload length
			payloadLength := binary.BigEndian.Uint32(buffer[0:4])
			messageLength := 4 + 8 + int(payloadLength)

			// Check if we have complete message
			if len(buffer) < messageLength {
				break
			}

			// Extract message
			timestamp := binary.BigEndian.Uint64(buffer[4:12])
			payload := buffer[12:messageLength]

			// Process frame
			s.processFrame(payload, timestamp, fallbackDuration)

			// Remove processed message from buffer
			buffer = buffer[messageLength:]
		}
	}
}

// processFrame processes a single H.264 frame
func (s *IOSWebRTCSession) processFrame(payload []byte, timestamp uint64, fallbackDuration time.Duration) {
	if len(payload) < 5 {
		return
	}

	// Check if this is H.264 data (starts with start code)
	if !(payload[0] == 0x00 && payload[1] == 0x00 && payload[2] == 0x00 && payload[3] == 0x01) {
		// Not H.264 - skip (likely JSON metadata)
		return
	}

	s.mu.Lock()
	s.frameCount++

	// Calculate duration from timestamps
	var duration time.Duration
	if s.firstTimestamp == 0 {
		s.firstTimestamp = timestamp
		s.lastTimestamp = timestamp
		duration = fallbackDuration // Default for first frame
	} else {
		// Calculate duration from timestamp difference (microseconds)
		timestampDiff := timestamp - s.lastTimestamp
		duration = time.Duration(timestampDiff) * time.Microsecond
		s.lastTimestamp = timestamp

		// Sanity check - if duration is crazy, use default
		if duration < time.Millisecond*10 || duration > time.Millisecond*100 {
			duration = fallbackDuration
		}
	}

	track := s.videoTrack
	s.mu.Unlock()

	// Log every 30 frames
	// Left for potential debugging
	// frameNum := s.frameCount
	// if frameNum%30 == 0 {
	// 	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Streaming frame #%d (%d bytes, duration=%v) for device %s", frameNum, len(payload), duration, s.device.UDID))
	// }

	// Write to WebRTC track
	if track != nil {
		if err := track.WriteSample(media.Sample{
			Data:     payload,
			Duration: duration,
		}); err != nil {
			logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Failed to write sample for device %s: %s", s.device.UDID, err))
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

	if s.tcpConn != nil {
		s.tcpConn.Close()
	}

	if s.peerConnection != nil {
		s.peerConnection.Close()
	}

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Closed iOS WebRTC session for device %s", s.device.UDID))
}

// IOSBroadcastWebRTCSocket handles WebRTC signaling for iOS broadcast extension streaming
func IOSBroadcastWebRTCSocket(c *gin.Context) {
	udid := c.Param("udid")

	platDev, deviceFound := devices.DevManager.Get(udid)
	if !deviceFound {
		logger.ProviderLogger.LogError("ios_webrtc", fmt.Sprintf("Device with UDID `%s` not found", udid))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	rcDev, isRcDevice := platDev.(devices.RemoteControllable)
	if !isRcDevice {
		logger.ProviderLogger.LogError("ios_webrtc", fmt.Sprintf("Device `%s` does not support streaming", udid))
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
	session, err := NewIOSWebRTCSession(rcDev.GetDBDevice(), rcDev.GetStreamPort(), rcDev.GetStreamTargetFPS())
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
