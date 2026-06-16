package server

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SanitizedInternal builds a gRPC status error with codes.Internal and a
// caller-provided public message. The underlying err is NEVER included in the
// status message returned to the client — instead it is:
//   - recorded on the active OTel span (if any) so traces carry the full
//     failure detail, and
//   - emitted via the project logger under the "internal server error" key
//     together with the public message for server-side triage.
//
// Callers should use this in place of status.Errorf(codes.Internal, ...)
// anywhere the err details could leak internal information (database errors,
// storage paths, signed-URL contents, etc.).
//
// If ctx has no active span (e.g. unit-test contexts that pass
// context.Background()), the helper still returns the sanitized status error
// and emits the log line.
func SanitizedInternal(ctx context.Context, publicMsg string, err error) error {
	if err == nil {
		err = status.Error(codes.Internal, publicMsg)
	}
	span := oteltrace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetStatus(otelcodes.Error, publicMsg)
		span.RecordError(err, oteltrace.WithAttributes(
			attribute.String("public_message", publicMsg),
		))
	}
	logrus.WithError(err).
		WithField("public_message", publicMsg).
		Error("internal server error")
	return status.Error(codes.Internal, publicMsg)
}

// PublicError returns a gRPC status error for an already-sanitized path where
// the caller has logged the underlying failure itself. It marks the active
// span as errored (when recording) but never leaks err into the public
// message and does not log.
func PublicError(ctx context.Context, code codes.Code, publicMsg string) error {
	span := oteltrace.SpanFromContext(ctx)
	if span.IsRecording() && code != codes.OK {
		span.SetStatus(otelcodes.Error, publicMsg)
	}
	return status.Error(code, publicMsg)
}
