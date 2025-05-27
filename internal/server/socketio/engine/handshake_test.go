package engine_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/denysvitali/immich-go-backend/internal/server/socketio/engine"
)

func TestHandshakeResponse(t *testing.T) {
	sessionId := "foo"
	resp := engine.CreateHandshakeResponse(sessionId)
	responseBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal handshake response: %v", err)
	}
	got := engine.EncodePacket(engine.PacketOpen, responseBytes)
	want := []byte("0{\"sid\":\"foo\",\"upgrades\":[],\"pingInterval\":25000,\"pingTimeout\":20000,\"maxPayload\":1000000}")
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Handshake response mismatch (-want +got):\n%s", diff)
	}
}
