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
	"GADS/device/manager"
	"GADS/device"
	"GADS/provider/logger"
	"context"
	"encoding/binary"
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

// AndroidH264Frame represents a frame with its presentation timestamp
type AndroidH264Frame struct {
	Data      []byte
	Timestamp uint64
}

// AndroidWebRTCSession manages a WebRTC peer connection for Android streaming
type AndroidWebRTCSession struct {
	device         *device.DeviceInfo
	peerConnection *webrtc.PeerConnection
	videoTrack     *webrtc.TrackLocalStaticSample
	wsConn         io.ReadWriteCloser
	frameChannel   chan AndroidH264Frame
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.Mutex
	iceCandidates  []webrtc.ICECandidateInit

	// Timestamp tracking
	firstTimestamp uint64
	lastTimestamp  uint64
	frameCount     int
}

// NewAndroidWebRTCSession creates a new WebRTC session for Android device streaming
func NewAndroidWebRTCSession(info *device.DeviceInfo) (*AndroidWebRTCSession, error) {
	ctx, cancel := context.WithCancel(context.Background())

	session := &AndroidWebRTCSession{
		device:        info,
		ctx:           ctx,
		cancel:        cancel,
		iceCandidates: make([]webrtc.ICECandidateInit, 0),
		frameChannel:  make(chan AndroidH264Frame, 30), // Buffer 30 frames
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

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Created Android WebRTC session for device %s", info.UDID))

	return session, nil
}

// Start begins the streaming pipeline
func (s *AndroidWebRTCSession) Start() error {
	// Connect to Android stream WebSocket
	streamURL := "ws://localhost:" + s.device.StreamPort

	// Dial WebSocket connection
	conn, _, _, err := ws.Dial(s.ctx, streamURL)
	if err != nil {
		return fmt.Errorf("failed to connect to Android stream: %w", err)
	}
	s.wsConn = conn

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Connected to Android H.264 stream for device %s", s.device.UDID))

	// Start reading frames from WebSocket
	go s.readFrames()

	// Start writing frames to WebRTC track
	go s.writeFrames()

	return nil
}

// readFrames reads H.264 frames from WebSocket and sends to channel
func (s *AndroidWebRTCSession) readFrames() {
	defer close(s.frameChannel)
	defer s.wsConn.Close()

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Starting frame reading for device %s", s.device.UDID))

	readCount := 0
	for {
		select {
		case <-s.ctx.Done():
			logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Stopping frame reading for device %s", s.device.UDID))
			return
		default:
		}

		// Read H.264 frame from WebSocket
		// Android sends: [8 bytes PTS][H.264 data]
		msg, _, err := wsutil.ReadServerData(s.wsConn)
		if err != nil {
			if err != io.EOF {
				logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Error reading from WebSocket for device %s: %s", s.device.UDID, err))
			}
			return
		}

		// Need at least 8 bytes for PTS + some H.264 data
		if len(msg) < 13 {
			continue
		}

		readCount++

		// Extract presentation timestamp (first 8 bytes, big-endian)
		timestamp := binary.BigEndian.Uint64(msg[0:8])

		// Extract H.264 data (everything after first 8 bytes)
		payload := msg[8:]

		// Send to channel (non-blocking to prevent read stalls)
		frame := AndroidH264Frame{
			Data:      payload,
			Timestamp: timestamp,
		}

		select {
		case s.frameChannel <- frame:
			// Successfully queued
		case <-s.ctx.Done():
			return
		default:
			// Channel full - drop frame to prevent blocking
			if readCount%30 == 0 {
				logger.ProviderLogger.LogWarn("stream_webrtc", fmt.Sprintf("Dropped frame (channel full) for device %s", s.device.UDID))
			}
		}
	}
}

// writeFrames reads frames from channel and writes to WebRTC track
func (s *AndroidWebRTCSession) writeFrames() {
	fallbackDuration := time.Second / time.Duration(30) // Default 30fps
	if s.device.StreamTargetFPS > 0 {
		fallbackDuration = time.Second / time.Duration(s.device.StreamTargetFPS)
	}

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Starting frame writing for device %s", s.device.UDID))

	for {
		select {
		case <-s.ctx.Done():
			logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Stopping frame writing for device %s", s.device.UDID))
			return
		case frame, ok := <-s.frameChannel:
			if !ok {
				logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Frame channel closed for device %s", s.device.UDID))
				return
			}

			if len(frame.Data) < 5 {
				continue
			}

			s.mu.Lock()
			s.frameCount++

			// Calculate duration from timestamps
			var duration time.Duration
			if s.firstTimestamp == 0 {
				s.firstTimestamp = frame.Timestamp
				s.lastTimestamp = frame.Timestamp
				duration = fallbackDuration
			} else {
				timestampDiff := frame.Timestamp - s.lastTimestamp
				duration = time.Duration(timestampDiff) * time.Microsecond
				s.lastTimestamp = frame.Timestamp

				// Sanity check
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
			// 	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Streaming frame #%d (%d bytes, duration=%v) for device %s", frameNum, len(frame.Data), duration, s.device.UDID))
			// }

			// Write to WebRTC track
			if track != nil {
				if err := track.WriteSample(media.Sample{
					Data:     frame.Data,
					Duration: duration,
				}); err != nil {
					logger.ProviderLogger.LogError("stream_webrtc", fmt.Sprintf("Failed to write sample for device %s: %s", s.device.UDID, err))
					return
				}
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

	if s.wsConn != nil {
		s.wsConn.Close()
	}

	if s.peerConnection != nil {
		s.peerConnection.Close()
	}

	logger.ProviderLogger.LogInfo("stream_webrtc", fmt.Sprintf("Closed Android WebRTC session for device %s", s.device.UDID))
}

// AndroidWebRTCSocket handles WebRTC signaling for Android devices
func AndroidWebRTCSocket(c *gin.Context) {
	udid := c.Param("udid")

	dev, ok := manager.Instance.GetDevice(udid)
	if !ok {
		logger.ProviderLogger.LogError("android_webrtc", fmt.Sprintf("Device with UDID `%s` not found", udid))
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
	session, err := NewAndroidWebRTCSession(dev.Info())
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
		msg := struct {
			Type      string                   `json:"type"`
			Candidate *webrtc.ICECandidateInit `json:"candidate"`
		}{
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

		var signalingMsg struct {
			Type      string                   `json:"type"`
			SDP       string                   `json:"sdp,omitempty"`
			Candidate *webrtc.ICECandidateInit `json:"candidate,omitempty"`
		}
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

			response := struct {
				Type string `json:"type"`
				SDP  string `json:"sdp"`
			}{
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
