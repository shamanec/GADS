package router

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/google/uuid"
)

type WebRTCSessionState string

const (
	StateActive     WebRTCSessionState = "Active"     // Offer and Answer messages have been exchanged
	StateCreating   WebRTCSessionState = "Creating"   // Creating session, offer sent
	StateReady      WebRTCSessionState = "Ready"      // Both clients available and ready to initiate session
	StateImpossible WebRTCSessionState = "Impossible" // Less than two clients
)

// MessageType defines the message types used in WebRTC signaling.
type MessageType string

const (
	MessageState  MessageType = "STATE"
	MessageOffer  MessageType = "OFFER"
	MessageAnswer MessageType = "ANSWER"
	MessageICE    MessageType = "ICE"
)

// SessionManager manages WebRTC sessions.
type SessionManager struct {
	mu      sync.Mutex
	clients map[uuid.UUID]net.Conn
	state   WebRTCSessionState
}

var sessionManager = &SessionManager{
	clients: make(map[uuid.UUID]net.Conn),
	state:   StateImpossible,
}

// handleWebSocket handles WebSocket connections for WebRTC signaling.
func WebRTCSocket(c *gin.Context) {
	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	sessionID := uuid.New()
	log.Printf("Client connected: %s", sessionID)

	if !sessionManager.addClient(sessionID, conn) {
		wsutil.WriteServerMessage(conn, ws.OpClose, nil)
		conn.Close()
		return
	}

	defer sessionManager.removeClient(sessionID)

	for {
		msg, op, err := wsutil.ReadClientData(conn)
		if err != nil {
			log.Printf("Client %s disconnected: %v", sessionID, err)
			return
		}

		if op == ws.OpText {
			sessionManager.handleMessage(sessionID, string(msg))
		}
	}
}

// addClient registers a new client session.
func (s *SessionManager) addClient(sessionID uuid.UUID, conn net.Conn) bool {
	s.mu.Lock()

	if len(s.clients) >= 2 {
		return false // Only two clients allowed
	}

	s.clients[sessionID] = conn
	if len(s.clients) == 2 {
		s.state = StateReady
	}
	s.mu.Unlock()
	s.notifyAboutStateUpdate()

	return true
}

// removeClient removes a client session.
func (s *SessionManager) removeClient(sessionID uuid.UUID) {
	s.mu.Lock()

	delete(s.clients, sessionID)
	s.state = StateImpossible
	s.mu.Unlock()
	s.notifyAboutStateUpdate()
}

// handleMessage processes incoming messages.
func (s *SessionManager) handleMessage(sessionID uuid.UUID, message string) {
	switch {
	case hasPrefixIgnoreCase(message, string(MessageState)):
		s.handleState(sessionID)
	case hasPrefixIgnoreCase(message, string(MessageOffer)):
		s.handleOffer(sessionID, message)
	case hasPrefixIgnoreCase(message, string(MessageAnswer)):
		s.handleAnswer(sessionID, message)
	case hasPrefixIgnoreCase(message, string(MessageICE)):
		s.handleICE(sessionID, message)
	}
}

// handleState sends the current WebRTC session state.
func (s *SessionManager) handleState(sessionID uuid.UUID) {
	s.sendToClient(sessionID, fmt.Sprintf("%s %s", MessageState, s.state))
}

// handleOffer forwards an offer to the other client.
func (s *SessionManager) handleOffer(sessionID uuid.UUID, message string) {
	s.mu.Lock()

	if s.state != StateReady {
		log.Printf("Invalid state for offer: %s", s.state)
		s.mu.Unlock()
		return
	}

	s.state = StateCreating
	s.mu.Unlock()
	s.notifyAboutStateUpdate()
	s.sendToOtherClient(sessionID, message)
}

// handleAnswer forwards an answer to the other client.
func (s *SessionManager) handleAnswer(sessionID uuid.UUID, message string) {
	s.mu.Lock()
	if s.state != StateCreating {
		log.Printf("Invalid state for answer: %s", s.state)
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	s.sendToOtherClient(sessionID, message)

	s.mu.Lock()
	s.state = StateActive
	s.mu.Unlock()

	s.notifyAboutStateUpdate()
}

// handleICE forwards ICE candidates to the other client.
func (s *SessionManager) handleICE(sessionID uuid.UUID, message string) {
	s.sendToOtherClient(sessionID, message)
}

// notifyAboutStateUpdate broadcasts the current WebRTC state.
func (s *SessionManager) notifyAboutStateUpdate() {
	for id := range s.clients {
		s.sendToClient(id, fmt.Sprintf("%s %s", MessageState, s.state))
	}
}

// sendToClient sends a message to a specific client.
func (s *SessionManager) sendToClient(sessionID uuid.UUID, message string) {
	s.mu.Lock()

	if conn, exists := s.clients[sessionID]; exists {
		_ = wsutil.WriteServerMessage(conn, ws.OpText, []byte(message))
	}
	s.mu.Unlock()
}

// sendToOtherClient sends a message to the other connected client.
func (s *SessionManager) sendToOtherClient(excludeID uuid.UUID, message string) {
	s.mu.Lock()

	for id, conn := range s.clients {
		if id != excludeID {
			_ = wsutil.WriteServerMessage(conn, ws.OpText, []byte(message))
			break
		}
	}
	s.mu.Unlock()
}

// hasPrefixIgnoreCase checks if a string has a prefix, case-insensitive.
func hasPrefixIgnoreCase(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}
