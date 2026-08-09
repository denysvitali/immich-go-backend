package websocket

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"github.com/denysvitali/immich-go-backend/internal/socketio"
	"github.com/denysvitali/immich-go-backend/internal/socketio/engine"
)

type Client struct {
	conn     *websocket.Conn
	session  *engine.Session
	hub      *Hub
	send     chan []byte
	done     chan struct{}
	stopOnce sync.Once
	pumps    sync.WaitGroup
}

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	done       chan struct{}
	stopped    chan struct{}
	stopOnce   sync.Once
	runOnce    sync.Once
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func New() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		done:       make(chan struct{}),
		stopped:    make(chan struct{}),
	}
}

// Run starts the hub
func (h *Hub) Run() {
	h.runOnce.Do(func() {
		defer close(h.stopped)

		for {
			select {
			case client := <-h.register:
				h.mu.Lock()
				h.clients[client] = true
				h.mu.Unlock()
				logrus.WithField("sessionID", client.session.ID).Info("Client registered")

			case client := <-h.unregister:
				h.mu.Lock()
				_, ok := h.clients[client]
				if ok {
					delete(h.clients, client)
				}
				h.mu.Unlock()
				if ok {
					logrus.WithField("sessionID", client.session.ID).Info("Client unregistered")
				}

			case <-h.done:
				h.mu.Lock()
				clients := make([]*Client, 0, len(h.clients))
				for client := range h.clients {
					clients = append(clients, client)
					delete(h.clients, client)
				}
				h.mu.Unlock()

				for _, client := range clients {
					client.stop()
				}
				return
			}
		}
	})
}

// Close stops the hub and disconnects every registered client. It is safe to
// call more than once.
func (h *Hub) Close() {
	h.stopOnce.Do(func() {
		close(h.done)
	})
}

func (h *Hub) registerClient(client *Client) bool {
	select {
	case <-h.done:
		return false
	default:
	}

	select {
	case h.register <- client:
		return true
	case <-h.done:
		return false
	}
}

func (h *Hub) unregisterClient(client *Client) {
	select {
	case h.unregister <- client:
	case <-h.done:
	}
}

// HandleWebSocket handles websocket requests from the peer
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.WithError(err).Error("Failed to upgrade connection to WebSocket")
		return
	}
	defer c.Close()

	// Create handshake response
	handshake := engine.CreateHandshakeResponse(engine.GenerateSessionID())

	// Create session
	session := engine.NewSession(
		handshake.SessionID,
		time.Duration(handshake.PingInterval)*time.Millisecond,
		time.Duration(handshake.PingTimeout)*time.Millisecond,
	)

	client := &Client{
		conn:    c,
		session: session,
		hub:     h,
		send:    make(chan []byte, 256),
		done:    make(chan struct{}),
	}

	// Register client
	if !h.registerClient(client) {
		return
	}
	defer func() {
		client.stop()
		_ = c.Close()
		client.pumps.Wait()
		h.unregisterClient(client)
	}()

	logrus.WithField("sessionID", session.ID).Info("WebSocket connection established")

	// Send Engine.IO handshake (open packet)
	handshakeBytes, err := json.Marshal(handshake)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal handshake response")
		return
	}

	openPacket := engine.EncodePacket(engine.PacketOpen, handshakeBytes)
	if err := c.WriteMessage(websocket.TextMessage, openPacket); err != nil {
		logrus.WithError(err).Error("Failed to write handshake response to WebSocket")
		return
	}

	// Start goroutines
	client.pumps.Add(3)
	go client.writePump()
	go client.readPump()
	go client.pingPump()

	// Wait for client to finish
	<-client.done
}

// stop is the single owner of the client's completion signal. Keeping the
// outbound channel open makes concurrent, best-effort senders safe while the
// pumps observe done and exit.
func (c *Client) stop() {
	c.stopOnce.Do(func() {
		close(c.done)
	})
}

func (c *Client) trySend(message []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}

	select {
	case c.send <- message:
		return true
	case <-c.done:
		return false
	default:
		return false
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer c.pumps.Done()
	defer c.stop()

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logrus.WithError(err).Error("Unexpected websocket close")
			}
			break
		}

		c.handleMessage(data)
		select {
		case <-c.done:
			return
		default:
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	defer c.pumps.Done()
	defer c.stop()

	ticker := time.NewTicker(54 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message := <-c.send:
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				logrus.WithError(err).Error("Failed to write message to websocket")
				return
			}

		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.done:
			return
		}
	}
}

// pingPump sends ping packets and handles pong responses
func (c *Client) pingPump() {
	defer c.pumps.Done()
	defer c.stop()

	ticker := time.NewTicker(c.session.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Send Engine.IO ping packet
			pingPacket := engine.EncodePacket(engine.PacketPing, nil)
			if c.trySend(pingPacket) {
				logrus.WithField("sessionID", c.session.ID).Debug("Sent ping packet")
			} else {
				// Channel is full, client is probably disconnected
				return
			}

		case <-c.done:
			return
		}
	}
}

// handleMessage processes incoming websocket messages
func (c *Client) handleMessage(data []byte) {
	logrus.WithFields(logrus.Fields{
		"sessionID": c.session.ID,
		"data":      string(data),
	}).Debug("Received message")

	// Decode Engine.IO packet
	packetType, payload := engine.DecodePacket(data)

	switch packetType {
	case engine.PacketPong:
		// Update session ping time
		c.session.UpdatePing()
		logrus.WithField("sessionID", c.session.ID).Debug("Received pong packet")

	case engine.PacketMessage:
		// This is a Socket.IO packet wrapped in Engine.IO message
		c.handleSocketIOMessage(payload)

	case engine.PacketClose:
		logrus.WithField("sessionID", c.session.ID).Info("Received close packet")
		c.stop()
	case engine.PacketPing:
		// Handle ping packet
		logrus.WithField("sessionID", c.session.ID).Debug("Received ping packet")
		// Respond with pong
		pongPacket := engine.EncodePacket(engine.PacketPong, nil)
		if !c.trySend(pongPacket) {
			logrus.WithField("sessionID", c.session.ID).Warn("Failed to send pong: client buffer full or closed")
			c.stop()
		}

	default:
		logrus.WithFields(logrus.Fields{
			"sessionID":  c.session.ID,
			"packetType": packetType,
		}).Warn("Received unknown packet type")
	}
}

// handleSocketIOMessage processes Socket.IO messages
func (c *Client) handleSocketIOMessage(data []byte) {
	packet, err := socketio.DecodeSocketIOPacket(data)
	if err != nil {
		logrus.WithError(err).Error("Failed to decode Socket.IO packet")
		return
	}

	logrus.WithFields(logrus.Fields{
		"sessionID": c.session.ID,
		"type":      packet.Type,
		"namespace": packet.Namespace,
		"data":      packet.Data,
	}).Debug("Received Socket.IO packet")

	switch packet.Type {
	case socketio.PacketConnect:
		// Handle Socket.IO connection request
		c.handleSocketIOConnect(packet)

	case socketio.PacketEvent:
		// Handle Socket.IO event
		c.handleSocketIOEvent(packet)

	case socketio.PacketDisconnect:
		// Handle Socket.IO disconnection
		logrus.WithField("sessionID", c.session.ID).Info("Socket.IO disconnect")
		c.stop()

	default:
		logrus.WithFields(logrus.Fields{
			"sessionID": c.session.ID,
			"type":      packet.Type,
		}).Warn("Received unknown Socket.IO packet type")
	}
}

// handleSocketIOConnect handles Socket.IO connection requests
func (c *Client) handleSocketIOConnect(packet *socketio.SocketIOPacket) {
	namespace := packet.Namespace
	if namespace == "" {
		namespace = "/"
	}

	// Create Socket.IO session ID (different from Engine.IO session ID)
	socketioSessionID := engine.GenerateSessionID()

	// Send CONNECT response
	response := socketio.CreateConnectResponse(namespace, socketioSessionID)
	responseBytes, err := socketio.EncodeSocketIOPacket(response)
	if err != nil {
		logrus.WithError(err).Error("Failed to encode Socket.IO connect response")
		return
	}

	// Wrap in Engine.IO message packet
	messagePacket := engine.EncodePacket(engine.PacketMessage, responseBytes)

	if c.trySend(messagePacket) {
		logrus.WithFields(logrus.Fields{
			"sessionID":         c.session.ID,
			"socketioSessionID": socketioSessionID,
			"namespace":         namespace,
		}).Info("Socket.IO connection established")
	} else {
		logrus.Error("Failed to send Socket.IO connect response: channel full")
		c.stop()
	}
}

// handleSocketIOEvent handles Socket.IO events
func (c *Client) handleSocketIOEvent(packet *socketio.SocketIOPacket) {
	logrus.WithFields(logrus.Fields{
		"sessionID": c.session.ID,
		"namespace": packet.Namespace,
		"data":      packet.Data,
		"id":        packet.ID,
	}).Info("Received Socket.IO event")

	// For now, just echo back the event for demonstration
	if packet.ID != nil {
		// Send acknowledgment
		ackPacket := socketio.SocketIOPacket{
			Type:      socketio.PacketAck,
			Namespace: packet.Namespace,
			ID:        packet.ID,
			Data:      []string{"acknowledged"},
		}

		ackBytes, err := socketio.EncodeSocketIOPacket(ackPacket)
		if err != nil {
			logrus.WithError(err).Error("Failed to encode Socket.IO ack response")
			return
		}

		messagePacket := engine.EncodePacket(engine.PacketMessage, ackBytes)
		if c.trySend(messagePacket) {
			logrus.WithField("sessionID", c.session.ID).Debug("Sent Socket.IO acknowledgment")
		} else {
			logrus.Error("Failed to send Socket.IO ack: channel full")
			c.stop()
		}
	}
}
