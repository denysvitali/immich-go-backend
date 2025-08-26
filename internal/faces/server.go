package faces

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server implements the FacesService
type Server struct {
	immichv1.UnimplementedFacesServiceServer
	service *Service
}

// NewServer creates a new faces server
func NewServer(service *Service) *Server {
	return &Server{
		service: service,
	}
}

// GetFaces retrieves faces, optionally filtered by ID
func (s *Server) GetFaces(ctx context.Context, request *immichv1.GetFacesRequest) (*immichv1.GetFacesResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Convert request
	req := GetFacesRequest{}
	if request.Id != nil {
		id := request.GetId()
		req.ID = &id
	}

	// Call service
	response, err := s.service.GetFaces(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get faces: %v", err)
	}

	// Convert response
	faces := make([]*immichv1.FaceResponse, len(response.Faces))
	for i, face := range response.Faces {
		faces[i] = &immichv1.FaceResponse{
			Id:       face.ID,
			AssetId:  face.AssetID,
			PersonId: face.PersonID,
			BoundingBox: &immichv1.BoundingBox{
				X1: face.BoundingBox.X1,
				Y1: face.BoundingBox.Y1,
				X2: face.BoundingBox.X2,
				Y2: face.BoundingBox.Y2,
			},
			ImageWidth:  face.ImageWidth,
			ImageHeight: face.ImageHeight,
		}
	}

	return &immichv1.GetFacesResponse{
		Faces: faces,
	}, nil
}

// CreateFace creates a new face detection record
func (s *Server) CreateFace(ctx context.Context, request *immichv1.CreateFaceRequest) (*immichv1.FaceResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Validate request
	if request.GetAssetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "asset_id is required")
	}
	if request.GetPersonId() == "" {
		return nil, status.Error(codes.InvalidArgument, "person_id is required")
	}
	if request.GetBoundingBox() == nil {
		return nil, status.Error(codes.InvalidArgument, "bounding_box is required")
	}

	// Convert request
	req := CreateFaceRequest{
		AssetID:  request.GetAssetId(),
		PersonID: request.GetPersonId(),
		BoundingBox: BoundingBox{
			X1: request.GetBoundingBox().GetX1(),
			Y1: request.GetBoundingBox().GetY1(),
			X2: request.GetBoundingBox().GetX2(),
			Y2: request.GetBoundingBox().GetY2(),
		},
	}

	// Call service
	response, err := s.service.CreateFace(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create face: %v", err)
	}

	// Convert response
	return &immichv1.FaceResponse{
		Id:       response.ID,
		AssetId:  response.AssetID,
		PersonId: response.PersonID,
		BoundingBox: &immichv1.BoundingBox{
			X1: response.BoundingBox.X1,
			Y1: response.BoundingBox.Y1,
			X2: response.BoundingBox.X2,
			Y2: response.BoundingBox.Y2,
		},
		ImageWidth:  response.ImageWidth,
		ImageHeight: response.ImageHeight,
	}, nil
}

// DeleteFace removes a face detection record
func (s *Server) DeleteFace(ctx context.Context, request *immichv1.DeleteFaceRequest) (*emptypb.Empty, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Validate request
	if request.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// Call service
	err := s.service.DeleteFace(ctx, request.GetId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete face: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// ReassignFacesById reassigns faces to a different person
func (s *Server) ReassignFacesById(ctx context.Context, request *immichv1.ReassignFacesByIdRequest) (*immichv1.ReassignFacesByIdResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Validate request
	if request.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if request.GetPersonId() == "" {
		return nil, status.Error(codes.InvalidArgument, "person_id is required")
	}

	// Call service
	response, err := s.service.ReassignFacesById(ctx, request.GetId(), request.GetPersonId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to reassign faces: %v", err)
	}

	// Convert response
	updatedFaces := make([]*immichv1.FaceResponse, len(response.UpdatedFaces))
	for i, face := range response.UpdatedFaces {
		updatedFaces[i] = &immichv1.FaceResponse{
			Id:       face.ID,
			AssetId:  face.AssetID,
			PersonId: face.PersonID,
			BoundingBox: &immichv1.BoundingBox{
				X1: face.BoundingBox.X1,
				Y1: face.BoundingBox.Y1,
				X2: face.BoundingBox.X2,
				Y2: face.BoundingBox.Y2,
			},
			ImageWidth:  face.ImageWidth,
			ImageHeight: face.ImageHeight,
		}
	}

	return &immichv1.ReassignFacesByIdResponse{
		UpdatedFaces: updatedFaces,
	}, nil
}
