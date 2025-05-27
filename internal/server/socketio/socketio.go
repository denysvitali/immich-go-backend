package socketio

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Socket.IO packet types
const (
	PacketConnect      = "0"
	PacketDisconnect   = "1"
	PacketEvent        = "2"
	PacketAck          = "3"
	PacketConnectError = "4"
	PacketBinaryEvent  = "5"
	PacketBinaryAck    = "6"
)

// SocketIOPacket represents a Socket.IO packet
type SocketIOPacket struct {
	Type      string      `json:"type"`
	Namespace string      `json:"namespace,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	ID        *int        `json:"id,omitempty"`
}

// EncodeSocketIOPacket encodes a Socket.IO packet
func EncodeSocketIOPacket(packet SocketIOPacket) ([]byte, error) {
	result := packet.Type

	// Add namespace if not default
	if packet.Namespace != "" && packet.Namespace != "/" {
		result += packet.Namespace + ","
	}

	// Add acknowledgment ID if present
	if packet.ID != nil {
		result += strconv.Itoa(*packet.ID)
	}

	// Add data if present
	if packet.Data != nil {
		dataBytes, err := json.Marshal(packet.Data)
		if err != nil {
			return nil, err
		}
		result += string(dataBytes)
	}

	return []byte(result), nil
}

// DecodeSocketIOPacket decodes a Socket.IO packet
func DecodeSocketIOPacket(data []byte) (*SocketIOPacket, error) {
	if len(data) == 0 {
		return nil, nil
	}

	packet := &SocketIOPacket{
		Type:      string(data[0]),
		Namespace: "/", // default namespace
	}

	remaining := string(data[1:])
	
	// Parse namespace if present
	if strings.HasPrefix(remaining, "/") {
		commaIdx := strings.Index(remaining, ",")
		if commaIdx > 0 {
			packet.Namespace = remaining[:commaIdx]
			remaining = remaining[commaIdx+1:]
		}
	}

	// Parse acknowledgment ID if present
	idEnd := 0
	for i, char := range remaining {
		if char >= '0' && char <= '9' {
			idEnd = i + 1
		} else {
			break
		}
	}
	
	if idEnd > 0 {
		if id, err := strconv.Atoi(remaining[:idEnd]); err == nil {
			packet.ID = &id
			remaining = remaining[idEnd:]
		}
	}

	// Parse data if present
	if len(remaining) > 0 {
		var data interface{}
		if err := json.Unmarshal([]byte(remaining), &data); err == nil {
			packet.Data = data
		}
	}

	return packet, nil
}

// CreateConnectResponse creates a Socket.IO CONNECT response packet
func CreateConnectResponse(namespace string, sessionID string) SocketIOPacket {
	data := map[string]string{"sid": sessionID}
	return SocketIOPacket{
		Type:      PacketConnect,
		Namespace: namespace,
		Data:      data,
	}
}
