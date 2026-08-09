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
	conn, hub, _ := dialHubWithHandler(t)
	return conn, hub
}

func dialHubWithHandler(t *testing.T) (*gorilla.Conn, *Hub, <-chan struct{}) {
	t.Helper()

	hub := New()
	go hub.Run()
	t.Cleanup(func() {
		hub.Close()
		select {
		case <-hub.stopped:
		case <-time.After(testDeadline):
			t.Errorf("hub did not stop")
		}
	})

	handlerReturned := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r)
		close(handlerReturned)
	}))
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
	return conn, hub, handlerReturned
}

func TestWebSocketRejectsCrossOriginUpgrade(t *testing.T) {
	hub := New()
	go hub.Run()
	t.Cleanup(hub.Close)

	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	t.Cleanup(srv.Close)

	header := http.Header{}
	header.Set("Origin", "https://attacker.example")
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, resp, err := gorilla.DefaultDialer.Dial(url, header)
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil {
		t.Fatal("cross-origin WebSocket upgrade unexpectedly succeeded")
	}
	if resp == nil {
		t.Fatal("cross-origin WebSocket upgrade returned no HTTP response")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
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
	conn, _, handlerReturned := dialHubWithHandler(t)
	readHandshake(t, conn)

	if err := conn.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	waitForHandler(t, handlerReturned, "the client disconnected")
}

// TestEngineIOCloseStopsClient exercises the exit path that used to close the
// client completion channel in both handleMessage and readPump.
func TestEngineIOCloseStopsClient(t *testing.T) {
	conn, _, handlerReturned := dialHubWithHandler(t)
	readHandshake(t, conn)

	if err := conn.WriteMessage(gorilla.TextMessage, engine.EncodePacket(engine.PacketClose, nil)); err != nil {
		t.Fatalf("write Engine.IO close: %v", err)
	}

	waitForHandler(t, handlerReturned, "an Engine.IO close packet")
}

// TestSocketIODisconnectStopsClient covers the equivalent nested Socket.IO
// disconnect path and guards against multiple pumps racing to stop the client.
func TestSocketIODisconnectStopsClient(t *testing.T) {
	conn, _, handlerReturned := dialHubWithHandler(t)
	readHandshake(t, conn)

	disconnect := engine.EncodePacket(engine.PacketMessage, []byte("1"))
	if err := conn.WriteMessage(gorilla.TextMessage, disconnect); err != nil {
		t.Fatalf("write Socket.IO disconnect: %v", err)
	}

	waitForHandler(t, handlerReturned, "a Socket.IO disconnect packet")
}

// TestPingWithFullSendBufferDoesNotBlock ensures an unresponsive client cannot
// pin readPump forever while it tries to enqueue the mandatory pong response.
func TestPingWithFullSendBufferDoesNotBlock(t *testing.T) {
	client := &Client{
		session: engine.NewSession("full-buffer", time.Second, time.Second),
		send:    make(chan []byte, 1),
		done:    make(chan struct{}),
	}
	client.send <- []byte("already full")

	returned := make(chan struct{})
	go func() {
		client.handleMessage(engine.EncodePacket(engine.PacketPing, nil))
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(testDeadline):
		t.Fatal("ping handling blocked on a full send buffer")
	}

	select {
	case <-client.done:
	case <-time.After(testDeadline):
		t.Fatal("client was not stopped after its send buffer filled")
	}
}

// TestHubCloseStopsClients proves that package owners can terminate Run and
// that shutdown releases active handlers. Repeated Close calls must be safe.
func TestHubCloseStopsClients(t *testing.T) {
	conn, hub, handlerReturned := dialHubWithHandler(t)
	readHandshake(t, conn)

	hub.Close()
	hub.Close()

	select {
	case <-hub.stopped:
	case <-time.After(testDeadline):
		t.Fatal("Hub.Run did not return after Close")
	}
	waitForHandler(t, handlerReturned, "hub shutdown")

	hub.mu.RLock()
	clientCount := len(hub.clients)
	hub.mu.RUnlock()
	if clientCount != 0 {
		t.Fatalf("hub retained %d clients after shutdown", clientCount)
	}
}

func waitForHandler(t *testing.T, handlerReturned <-chan struct{}, after string) {
	t.Helper()
	select {
	case <-handlerReturned:
	case <-time.After(testDeadline):
		t.Fatalf("HandleWebSocket did not return after %s", after)
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
