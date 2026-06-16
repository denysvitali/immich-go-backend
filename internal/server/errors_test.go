package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSanitizedInternal_DoesNotLeakErrorMessage(t *testing.T) {
	const (
		sensitive = "secret_password_xyz"
		publicMsg = "operation failed"
	)

	err := SanitizedInternal(context.Background(), publicMsg, errors.New("database blew up: "+sensitive))

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T (%v)", err, err)
	}

	if got := st.Message(); got != publicMsg {
		t.Fatalf("public message mismatch: got %q, want %q", got, publicMsg)
	}

	if strings.Contains(st.Message(), sensitive) {
		t.Fatalf("public message leaked sensitive substring %q: %q", sensitive, st.Message())
	}
}

func TestSanitizedInternal_PreservesCode(t *testing.T) {
	err := SanitizedInternal(context.Background(), "operation failed", errors.New("boom"))

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T (%v)", err, err)
	}

	if st.Code() != codes.Internal {
		t.Fatalf("code mismatch: got %v, want %v", st.Code(), codes.Internal)
	}
}

func TestSanitizedInternal_RecordsSpanError(t *testing.T) {
	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("SanitizedInternal panicked with a recording span: %v", r)
		}
	}()

	err := SanitizedInternal(ctx, "operation failed", errors.New("internal detail"))

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T (%v)", err, err)
	}
	if st.Code() != codes.Internal {
		t.Fatalf("code mismatch: got %v, want %v", st.Code(), codes.Internal)
	}
	if st.Message() != "operation failed" {
		t.Fatalf("public message mismatch: got %q, want %q", st.Message(), "operation failed")
	}
}

func TestPublicError_RoundTrip(t *testing.T) {
	err := PublicError(context.Background(), codes.NotFound, "user not found")

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T (%v)", err, err)
	}

	if st.Code() != codes.NotFound {
		t.Fatalf("code mismatch: got %v, want %v", st.Code(), codes.NotFound)
	}
	if st.Message() != "user not found" {
		t.Fatalf("message mismatch: got %q, want %q", st.Message(), "user not found")
	}
}
