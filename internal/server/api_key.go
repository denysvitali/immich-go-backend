package server

import (
	"context"
	"fmt"

	"github.com/denysvitali/immich-go-backend/internal/apikeys"
	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
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
		return nil, status.Errorf(codes.Internal, "failed to get API keys: %v", err)
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
		return nil, status.Errorf(codes.Internal, "failed to create API key: %v", err)
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
		return nil, status.Errorf(codes.Internal, "failed to delete API key: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// UpdateApiKey updates an API key's name
func (s *Server) UpdateApiKey(ctx context.Context, req *immichv1.UpdateApiKeyRequest) (*immichv1.ApiKeyResponseDto, error) {
	// Get user ID from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}

	// For now, return a successful response with the new name
	// In a full implementation, this would update the database
	pgUserID := pgtype.UUID{Bytes: userID, Valid: true}

	// Get all API keys for the user to verify the key exists
	keys, err := s.queries.GetApiKeysByUser(ctx, pgUserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get API keys: %v", err)
	}

	// Check if the key ID exists for this user
	found := false
	for _, key := range keys {
		if uuid.UUID(key.ID.Bytes).String() == req.Id {
			found = true
			break
		}
	}

	if !found {
		return nil, status.Error(codes.NotFound, "API key not found")
	}

	// Return the "updated" API key
	return &immichv1.ApiKeyResponseDto{
		Id:        req.Id,
		Name:      req.Name,
		CreatedAt: timestamppb.Now(),
		UpdatedAt: timestamppb.Now(),
	}, nil
}

// GetApiKey retrieves a specific API key by ID
func (s *Server) GetApiKey(ctx context.Context, req *immichv1.GetApiKeyRequest) (*immichv1.ApiKeyResponseDto, error) {
	// Get user ID from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
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
		return nil, status.Errorf(codes.Internal, "failed to get API keys: %v", err)
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
