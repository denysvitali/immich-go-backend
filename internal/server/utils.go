package server

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// getUserIDFromContext extracts the user ID from the gRPC context
func (s *Server) getUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.UUID{}, status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Try to get user ID from authorization header
	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return uuid.UUID{}, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	// Extract token from "Bearer <token>" format
	authHeader := authHeaders[0]
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return uuid.UUID{}, status.Error(codes.Unauthenticated, "invalid authorization header format")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	
	// Validate token and get user info
	userInfo, err := s.authService.ValidateToken(token)
	if err != nil {
		return uuid.UUID{}, status.Error(codes.Unauthenticated, "invalid token")
	}

	// Parse user ID string to UUID
	userID, err := uuid.Parse(userInfo.ID)
	if err != nil {
		return uuid.UUID{}, status.Error(codes.Internal, "invalid user ID format")
	}

	return userID, nil
}

// timestampFromTime converts a Go time.Time to a protobuf timestamp
func timestampFromTime(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// timeFromTimestamp converts a protobuf timestamp to a Go time.Time
func timeFromTimestamp(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}
