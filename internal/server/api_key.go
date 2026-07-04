package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/denysvitali/immich-go-backend/internal/apikeys"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Ensure Server implements ApiKeyServiceServer
var _ immichv1.ApiKeyServiceServer = (*Server)(nil)

// GetApiKeys retrieves all API keys for the current user
func (s *Server) GetApiKeys(ctx context.Context, _ *emptypb.Empty) (*immichv1.GetApiKeysResponse, error) {
	// Get user ID from context (set by auth middleware)
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Create service if not exists
	apiKeyService := apikeys.NewService(s.db.Queries)

	// Get all API keys for the user
	keys, err := apiKeyService.GetAPIKeysByUser(ctx, userID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get API keys", err)
	}

	// Convert to response format
	response := &immichv1.GetApiKeysResponse{
		ApiKeys: make([]*immichv1.ApiKeyResponseDto, len(keys)),
	}

	for i, key := range keys {
		response.ApiKeys[i] = &immichv1.ApiKeyResponseDto{
			Id:        key.ID.String(),
			Name:      key.Name,
			CreatedAt: timestamppb.New(key.CreatedAt.Time),
			UpdatedAt: timestamppb.New(key.UpdatedAt.Time),
		}
	}

	return response, nil
}

// CreateApiKey creates a new API key for the current user
func (s *Server) CreateApiKey(ctx context.Context, req *immichv1.CreateApiKeyRequest) (*immichv1.CreateApiKeyResponse, error) {
	// Validate request
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "API key name is required")
	}

	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Create service
	apiKeyService := apikeys.NewService(s.db.Queries)

	// Create the API key
	apiKey, rawKey, err := apiKeyService.CreateAPIKey(ctx, userID, req.Name)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to create API key", err)
	}

	// Return response with the raw key (only shown once)
	return &immichv1.CreateApiKeyResponse{
		ApiKey: &immichv1.ApiKeyResponseDto{
			Id:        apiKey.ID.String(),
			Name:      apiKey.Name,
			CreatedAt: timestamppb.New(apiKey.CreatedAt.Time),
			UpdatedAt: timestamppb.New(apiKey.UpdatedAt.Time),
		},
		Secret: fmt.Sprintf("immich_%s", rawKey), // Prefix with "immich_" like the real Immich
	}, nil
}

// DeleteApiKey deletes an API key
func (s *Server) DeleteApiKey(ctx context.Context, req *immichv1.DeleteApiKeyRequest) (*emptypb.Empty, error) {
	// Validate request
	keyID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid API key ID")
	}

	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Create service
	apiKeyService := apikeys.NewService(s.db.Queries)

	// Delete the API key
	if err := apiKeyService.DeleteAPIKey(ctx, keyID, userID); err != nil {
		return nil, SanitizedInternal(ctx, "failed to delete API key", err)
	}

	return &emptypb.Empty{}, nil
}

// UpdateApiKey updates an API key's name
func (s *Server) UpdateApiKey(ctx context.Context, req *immichv1.UpdateApiKeyRequest) (*immichv1.ApiKeyResponseDto, error) {
	// Get user ID from context
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}

	keyID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid API key ID")
	}

	pgUserID := pgtype.UUID{Bytes: userID, Valid: true}
	updated, err := s.queries.UpdateApiKeyName(ctx, sqlc.UpdateApiKeyNameParams{
		ID:     pgtype.UUID{Bytes: keyID, Valid: true},
		UserId: pgUserID,
		Name:   req.GetName(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "API key not found")
		}
		return nil, SanitizedInternal(ctx, "failed to update API key", err)
	}

	return &immichv1.ApiKeyResponseDto{
		Id:        uuid.UUID(updated.ID.Bytes).String(),
		Name:      updated.Name,
		CreatedAt: timestamppb.New(updated.CreatedAt.Time),
		UpdatedAt: timestamppb.New(updated.UpdatedAt.Time),
	}, nil
}

// GetMyApiKey returns the API key record used to authenticate the current
// request. The raw key is taken from the x-api-key header (forwarded from the
// HTTP gateway) and looked up by its hash, then verified to belong to the
// authenticated user when claims are present.
func (s *Server) GetMyApiKey(ctx context.Context, _ *emptypb.Empty) (*immichv1.ApiKeyResponseDto, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing request metadata")
	}

	values := md.Get("x-api-key")
	if len(values) == 0 || values[0] == "" {
		return nil, status.Error(codes.InvalidArgument, "current request is not authenticated with an API key")
	}
	rawKey := strings.TrimPrefix(values[0], "immich_")

	apiKeyService := apikeys.NewService(s.db.Queries)
	apiKey, err := apiKeyService.ValidateAPIKey(ctx, rawKey)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid API key")
	}

	// When the request also carries user claims, make sure the key belongs
	// to the same user.
	if userID, err := s.getUserIDFromContext(ctx); err == nil {
		if uuid.UUID(apiKey.UserId.Bytes) != userID {
			return nil, status.Error(codes.PermissionDenied, "API key does not belong to the current user")
		}
	}

	return &immichv1.ApiKeyResponseDto{
		Id:        uuid.UUID(apiKey.ID.Bytes).String(),
		Name:      apiKey.Name,
		CreatedAt: timestamppb.New(apiKey.CreatedAt.Time),
		UpdatedAt: timestamppb.New(apiKey.UpdatedAt.Time),
	}, nil
}

// GetApiKey retrieves a specific API key by ID
func (s *Server) GetApiKey(ctx context.Context, req *immichv1.GetApiKeyRequest) (*immichv1.ApiKeyResponseDto, error) {
	// Get user ID from context
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}

	// Get all API keys for the user and find the requested one
	pgUserID := pgtype.UUID{Bytes: userID, Valid: true}

	keys, err := s.queries.GetApiKeysByUser(ctx, pgUserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get API keys", err)
	}

	// Find the specific key
	for _, key := range keys {
		if uuid.UUID(key.ID.Bytes).String() == req.Id {
			// Return the API key
			return &immichv1.ApiKeyResponseDto{
				Id:        uuid.UUID(key.ID.Bytes).String(),
				Name:      key.Name,
				CreatedAt: timestamppb.New(key.CreatedAt.Time),
				UpdatedAt: timestamppb.New(key.UpdatedAt.Time),
			}, nil
		}
	}

	return nil, status.Error(codes.NotFound, "API key not found")
}
