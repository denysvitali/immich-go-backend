package stacks

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the StacksService
type Server struct {
	immichv1.UnimplementedStacksServiceServer
	service *Service
}

// NewServer creates a new stacks server
func NewServer(service *Service) *Server {
	return &Server{
		service: service,
	}
}

// CreateStack creates a new asset stack
func (s *Server) CreateStack(ctx context.Context, request *immichv1.CreateStackRequest) (*immichv1.StackResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Convert request
	req := CreateStackRequest{
		AssetIDs: request.GetAssetIds(),
	}

	// Call service
	response, err := s.service.CreateStack(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create stack: %v", err)
	}

	// Convert response
	return &immichv1.StackResponse{
		Id:             response.ID,
		PrimaryAssetId: response.PrimaryAssetID,
		AssetIds:       response.AssetIDs,
		AssetCount:     response.AssetCount,
		CreatedAt:      timestamppb.New(response.CreatedAt),
		UpdatedAt:      timestamppb.New(response.UpdatedAt),
	}, nil
}

// GetStack retrieves a stack by ID
func (s *Server) GetStack(ctx context.Context, request *immichv1.GetStackRequest) (*immichv1.StackResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Call service
	response, err := s.service.GetStack(ctx, request.GetId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get stack: %v", err)
	}

	// Convert response
	return &immichv1.StackResponse{
		Id:             response.ID,
		PrimaryAssetId: response.PrimaryAssetID,
		AssetIds:       response.AssetIDs,
		AssetCount:     response.AssetCount,
		CreatedAt:      timestamppb.New(response.CreatedAt),
		UpdatedAt:      timestamppb.New(response.UpdatedAt),
	}, nil
}

// UpdateStack updates an existing stack
func (s *Server) UpdateStack(ctx context.Context, request *immichv1.UpdateStackRequest) (*immichv1.StackResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Convert request
	req := UpdateStackRequest{}
	if request.PrimaryAssetId != nil {
		primaryAssetID := request.GetPrimaryAssetId()
		req.PrimaryAssetID = &primaryAssetID
	}

	// Call service
	response, err := s.service.UpdateStack(ctx, request.GetId(), req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update stack: %v", err)
	}

	// Convert response
	return &immichv1.StackResponse{
		Id:             response.ID,
		PrimaryAssetId: response.PrimaryAssetID,
		AssetIds:       response.AssetIDs,
		AssetCount:     response.AssetCount,
		CreatedAt:      timestamppb.New(response.CreatedAt),
		UpdatedAt:      timestamppb.New(response.UpdatedAt),
	}, nil
}

// DeleteStack removes a stack
func (s *Server) DeleteStack(ctx context.Context, request *immichv1.DeleteStackRequest) (*emptypb.Empty, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Call service
	err := s.service.DeleteStack(ctx, request.GetId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete stack: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteStacks removes multiple stacks
func (s *Server) DeleteStacks(ctx context.Context, request *immichv1.DeleteStacksRequest) (*emptypb.Empty, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Call service
	err := s.service.DeleteStacks(ctx, request.GetIds())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete stacks: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// SearchStacks searches for stacks based on criteria
func (s *Server) SearchStacks(ctx context.Context, request *immichv1.SearchStacksRequest) (*immichv1.SearchStacksResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Convert request
	req := SearchStacksRequest{}
	if request.UserId != nil {
		userID := request.GetUserId()
		req.UserID = &userID
	}
	if request.PrimaryAssetId != nil {
		primaryAssetID := request.GetPrimaryAssetId()
		req.PrimaryAssetID = &primaryAssetID
	}

	// Call service
	response, err := s.service.SearchStacks(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to search stacks: %v", err)
	}

	// Convert response
	stacks := make([]*immichv1.StackResponse, len(response.Stacks))
	for i, stack := range response.Stacks {
		stacks[i] = &immichv1.StackResponse{
			Id:             stack.ID,
			PrimaryAssetId: stack.PrimaryAssetID,
			AssetIds:       stack.AssetIDs,
			AssetCount:     stack.AssetCount,
			CreatedAt:      timestamppb.New(stack.CreatedAt),
			UpdatedAt:      timestamppb.New(stack.UpdatedAt),
		}
	}

	return &immichv1.SearchStacksResponse{
		Stacks: stacks,
	}, nil
}
