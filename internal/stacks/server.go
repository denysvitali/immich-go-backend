package stacks

import (
	"context"
	"errors"
	"strings"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type stackService interface {
	CreateStack(ctx context.Context, userID string, req CreateStackRequest) (*StackResponse, error)
	GetStack(ctx context.Context, userID, stackID string) (*StackResponse, error)
	UpdateStack(ctx context.Context, userID, stackID string, req UpdateStackRequest) (*StackResponse, error)
	DeleteStack(ctx context.Context, userID, stackID string) error
	DeleteStacks(ctx context.Context, userID string, stackIDs []string) error
	SearchStacks(ctx context.Context, req SearchStacksRequest) (*SearchStacksResponse, error)
	RemoveAssetFromStack(ctx context.Context, userID, stackID, assetID string) error
}

// Server implements the StacksService
type Server struct {
	immichv1.UnimplementedStacksServiceServer
	service stackService
}

// NewServer creates a new stacks server
func NewServer(service stackService) *Server {
	return &Server{
		service: service,
	}
}

func currentUserIDFromContext(ctx context.Context) (string, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return "", err
	}

	return userID.String(), nil
}

func stackResponse(response *StackResponse) *immichv1.StackResponse {
	if response == nil {
		return nil
	}

	return &immichv1.StackResponse{
		Id:             response.ID,
		PrimaryAssetId: response.PrimaryAssetID,
		AssetIds:       response.AssetIDs,
		AssetCount:     response.AssetCount,
		CreatedAt:      timestamppb.New(response.CreatedAt),
		UpdatedAt:      timestamppb.New(response.UpdatedAt),
	}
}

func stackResponses(responses []*StackResponse) []*immichv1.StackResponse {
	stacks := make([]*immichv1.StackResponse, len(responses))
	for i, response := range responses {
		stacks[i] = stackResponse(response)
	}

	return stacks
}

func stackStatusError(err error, fallback string) error {
	if err == nil {
		return nil
	}
	if code := status.Code(err); code != codes.Unknown {
		return err
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "invalid "), strings.Contains(msg, " is required"):
		return status.Error(codes.InvalidArgument, msg)
	case errors.Is(err, pgx.ErrNoRows), strings.Contains(msg, "not found"):
		return status.Error(codes.NotFound, msg)
	case strings.Contains(msg, "access denied"):
		return status.Error(codes.PermissionDenied, "access denied")
	default:
		return status.Errorf(codes.Internal, "%s: %v", fallback, err)
	}
}

// CreateStack creates a new asset stack
func (s *Server) CreateStack(ctx context.Context, request *immichv1.CreateStackRequest) (*immichv1.StackResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Convert request
	req := CreateStackRequest{
		AssetIDs: request.GetAssetIds(),
	}

	// Call service
	response, err := s.service.CreateStack(ctx, userID, req)
	if err != nil {
		return nil, stackStatusError(err, "failed to create stack")
	}

	return stackResponse(response), nil
}

// GetStack retrieves a stack by ID
func (s *Server) GetStack(ctx context.Context, request *immichv1.GetStackRequest) (*immichv1.StackResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Call service
	response, err := s.service.GetStack(ctx, userID, request.GetId())
	if err != nil {
		return nil, stackStatusError(err, "failed to get stack")
	}

	return stackResponse(response), nil
}

// UpdateStack updates an existing stack
func (s *Server) UpdateStack(ctx context.Context, request *immichv1.UpdateStackRequest) (*immichv1.StackResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Convert request
	req := UpdateStackRequest{}
	if request.PrimaryAssetId != nil {
		primaryAssetID := request.GetPrimaryAssetId()
		req.PrimaryAssetID = &primaryAssetID
	}

	// Call service
	response, err := s.service.UpdateStack(ctx, userID, request.GetId(), req)
	if err != nil {
		return nil, stackStatusError(err, "failed to update stack")
	}

	return stackResponse(response), nil
}

// DeleteStack removes a stack
func (s *Server) DeleteStack(ctx context.Context, request *immichv1.DeleteStackRequest) (*emptypb.Empty, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Call service
	err = s.service.DeleteStack(ctx, userID, request.GetId())
	if err != nil {
		return nil, stackStatusError(err, "failed to delete stack")
	}

	return &emptypb.Empty{}, nil
}

// DeleteStacks removes multiple stacks
func (s *Server) DeleteStacks(ctx context.Context, request *immichv1.DeleteStacksRequest) (*emptypb.Empty, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Call service
	err = s.service.DeleteStacks(ctx, userID, request.GetIds())
	if err != nil {
		return nil, stackStatusError(err, "failed to delete stacks")
	}

	return &emptypb.Empty{}, nil
}

// SearchStacks searches for stacks based on criteria
func (s *Server) SearchStacks(ctx context.Context, request *immichv1.SearchStacksRequest) (*immichv1.SearchStacksResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Convert request
	req := SearchStacksRequest{UserID: &userID}
	if request.PrimaryAssetId != nil {
		primaryAssetID := request.GetPrimaryAssetId()
		req.PrimaryAssetID = &primaryAssetID
	}

	// Call service
	response, err := s.service.SearchStacks(ctx, req)
	if err != nil {
		return nil, stackStatusError(err, "failed to search stacks")
	}

	return &immichv1.SearchStacksResponse{
		Stacks: stackResponses(response.Stacks),
	}, nil
}

// RemoveAssetFromStack removes a single asset from a stack owned by the
// current user.
func (s *Server) RemoveAssetFromStack(ctx context.Context, request *immichv1.RemoveAssetFromStackRequest) (*emptypb.Empty, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	err = s.service.RemoveAssetFromStack(ctx, userID, request.GetId(), request.GetAssetId())
	if err != nil {
		return nil, stackStatusError(err, "failed to remove asset from stack")
	}

	return &emptypb.Empty{}, nil
}
