package engine

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Engine.IO packet types
const (
	PacketOpen    = "0"
	PacketClose   = "1"
	PacketPing    = "2"
	PacketPong    = "3"
	PacketMessage = "4"
	PacketUpgrade = "5"
	PacketNoop    = "6"
)

type HandshakeResponse struct {
	SessionID    string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval int      `json:"pingInterval"`
	PingTimeout  int      `json:"pingTimeout"`
	MaxPayload   int      `json:"maxPayload"`
}

// GenerateSessionID creates a new session ID
func GenerateSessionID() string {
	bytes := make([]byte, 10)
	_, _ = rand.Read(bytes) // crypto/rand.Read never returns an error
	return hex.EncodeToString(bytes)
}

// CreateHandshakeResponse creates a handshake response with default values
func CreateHandshakeResponse(sessionId string) HandshakeResponse {
	return HandshakeResponse{
		SessionID:    sessionId,
		Upgrades:     []string{},
		PingInterval: 25000, // 25 seconds
		PingTimeout:  20000, // 20 seconds
		MaxPayload:   1000000,
	}
}

// Session represents an Engine.IO session
type Session struct {
	ID           string
	LastPing     time.Time
	PingInterval time.Duration
	PingTimeout  time.Duration
}

// NewSession creates a new session
func NewSession(id string, pingInterval, pingTimeout time.Duration) *Session {
	return &Session{
		ID:           id,
		LastPing:     time.Now(),
		PingInterval: pingInterval,
		PingTimeout:  pingTimeout,
	}
}

// IsExpired checks if the session has expired due to missing pings
func (s *Session) IsExpired() bool {
	return time.Since(s.LastPing) > s.PingInterval+s.PingTimeout
}

// UpdatePing updates the last ping time
func (s *Session) UpdatePing() {
	s.LastPing = time.Now()
}

// EncodePacket encodes a packet with the given type and optional data
func EncodePacket(packetType string, data []byte) []byte {
	if data == nil {
		return []byte(packetType)
	}
	return append([]byte(packetType), data...)
}

// DecodePacket decodes a packet returning the type and data
func DecodePacket(packet []byte) (string, []byte) {
	if len(packet) == 0 {
		return "", nil
	}
	packetType := string(packet[0])
	if len(packet) > 1 {
		return packetType, packet[1:]
	}
	return packetType, nil
}
