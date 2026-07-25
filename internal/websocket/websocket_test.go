package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"

	"github.com/denysvitali/immich-go-backend/internal/socketio/engine"
)

// These tests pin the observable socket.io/engine.io behaviour of Hub and
// Client before either is restructured. They talk to a real HTTP server over a
// real WebSocket, so they describe the wire contract the Immich clients rely
// on, not the internals.

const testDeadline = 5 * time.Second

// dialHub starts a Hub behind an httptest server and returns a connected
// client. Both are torn down when the test ends.
func dialHub(t *testing.T) (*gorilla.Conn, *Hub) {
	t.Helper()

	hub := New()
	go hub.Run()

	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	t.Cleanup(srv.Close)

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, resp, err := gorilla.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", url, err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	t.Cleanup(func() { _ = conn.Close() })

	if err := conn.SetReadDeadline(time.Now().Add(testDeadline)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	return conn, hub
}

// TestHandshakeIsFirstPacket pins that the server opens with an Engine.IO OPEN
// packet carrying a session ID and the ping timings, which is what every
// socket.io client waits for before sending anything.
func TestHandshakeIsFirstPacket(t *testing.T) {
	conn, _ := dialHub(t)

	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read handshake: %v", err)
	}

	packetType, payload := engine.DecodePacket(data)
	if packetType != engine.PacketOpen {
		t.Fatalf("first packet type = %v, want PacketOpen", packetType)
	}

	var handshake struct {
		SessionID    string `json:"sid"`
		PingInterval int    `json:"pingInterval"`
		PingTimeout  int    `json:"pingTimeout"`
	}
	if err := json.Unmarshal(payload, &handshake); err != nil {
		t.Fatalf("decode handshake payload %q: %v", payload, err)
	}
	if handshake.SessionID == "" {
		t.Error("handshake has no session ID")
	}
	if handshake.PingInterval <= 0 {
		t.Errorf("pingInterval = %d, want > 0", handshake.PingInterval)
	}
	if handshake.PingTimeout <= 0 {
		t.Errorf("pingTimeout = %d, want > 0", handshake.PingTimeout)
	}
}

// TestClientPingGetsPong pins that an Engine.IO PING from the client is
// answered with a PONG.
func TestClientPingGetsPong(t *testing.T) {
	conn, _ := dialHub(t)
	readHandshake(t, conn)

	if err := conn.WriteMessage(gorilla.TextMessage, engine.EncodePacket(engine.PacketPing, nil)); err != nil {
		t.Fatalf("write ping: %v", err)
	}

	packetType, _ := readPacket(t, conn)
	if packetType != engine.PacketPong {
		t.Errorf("reply to PING = %v, want PacketPong", packetType)
	}
}

// TestSocketIOConnectGetsConnectAck pins the socket.io namespace handshake:
// a CONNECT packet is answered with a CONNECT carrying a fresh socket.io sid.
func TestSocketIOConnectGetsConnectAck(t *testing.T) {
	conn, _ := dialHub(t)
	readHandshake(t, conn)

	// socket.io CONNECT to the default namespace, wrapped in an Engine.IO
	// MESSAGE packet: "0" is the socket.io CONNECT type.
	if err := conn.WriteMessage(gorilla.TextMessage,
		engine.EncodePacket(engine.PacketMessage, []byte("0"))); err != nil {
		t.Fatalf("write socket.io connect: %v", err)
	}

	packetType, payload := readPacket(t, conn)
	if packetType != engine.PacketMessage {
		t.Fatalf("reply type = %v, want PacketMessage", packetType)
	}
	if len(payload) == 0 || payload[0] != '0' {
		t.Errorf("reply payload = %q, want a socket.io CONNECT (leading '0')", payload)
	}
	if !strings.Contains(string(payload), "sid") {
		t.Errorf("reply payload = %q, want a socket.io session ID", payload)
	}
}

// TestClientCloseUnregisters pins that dropping the TCP connection tears the
// client down: HandleWebSocket returns, which means readPump exited and the
// hub processed the unregister.
func TestClientCloseUnregisters(t *testing.T) {
	hub := New()
	go hub.Run()

	handlerReturned := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r)
		close(handlerReturned)
	}))
	t.Cleanup(srv.Close)

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, resp, err := gorilla.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	if err := conn.SetReadDeadline(time.Now().Add(testDeadline)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	readHandshake(t, conn)

	if err := conn.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	select {
	case <-handlerReturned:
	case <-time.After(testDeadline):
		t.Fatal("HandleWebSocket did not return after the client disconnected")
	}
}

func readHandshake(t *testing.T, conn *gorilla.Conn) {
	t.Helper()

	packetType, _ := readPacket(t, conn)
	if packetType != engine.PacketOpen {
		t.Fatalf("expected the handshake first, got %v", packetType)
	}
}

func readPacket(t *testing.T, conn *gorilla.Conn) (string, []byte) {
	t.Helper()

	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		// Skip the periodic WebSocket-level pings writePump emits; they are
		// not Engine.IO packets.
		if messageType != gorilla.TextMessage {
			continue
		}
		packetType, payload := engine.DecodePacket(data)
		return packetType, payload
	}
}
