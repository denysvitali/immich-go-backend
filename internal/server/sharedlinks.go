package server

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/sharedlinks"
)

// Shared link type strings as stored in the database (upstream Immich values).
const (
	sharedLinkTypeAlbum      = "ALBUM"
	sharedLinkTypeIndividual = "INDIVIDUAL"
)

// GetAllSharedLinks returns all shared links owned by the authenticated user.
func (s *Server) GetAllSharedLinks(ctx context.Context, _ *immichv1.GetAllSharedLinksRequest) (*immichv1.GetAllSharedLinksResponse, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	links, err := s.sharedLinksService.GetSharedLinks(ctx, userID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get shared links", err)
	}

	protoLinks := make([]*immichv1.SharedLinkResponse, len(links))
	for i, link := range links {
		protoLinks[i] = convertSharedLinkToProto(link)
	}

	return &immichv1.GetAllSharedLinksResponse{SharedLinks: protoLinks}, nil
}

// GetMySharedLink resolves a shared link from its key (passed as the token
// query parameter), verifying the optional password. This mirrors upstream
// Immich's GET /shared-links/me endpoint.
func (s *Server) GetMySharedLink(ctx context.Context, request *immichv1.GetMySharedLinkRequest) (*immichv1.SharedLinkResponse, error) {
	key := request.GetToken()
	if key == "" {
		return nil, status.Error(codes.InvalidArgument, "shared link key (token) is required")
	}

	link, err := s.sharedLinksService.GetSharedLinkByKey(ctx, key, request.GetPassword())
	if err != nil {
		return nil, sharedLinkError(ctx, err)
	}

	response := convertSharedLinkToProto(link)

	// The caller proved access (key + password), so the asset list can be
	// included in the response.
	assets, err := s.sharedLinksService.GetSharedLinkAssets(ctx, key, request.GetPassword())
	if err != nil {
		return nil, sharedLinkError(ctx, err)
	}
	assetIDs := make([]string, len(assets))
	for i, asset := range assets {
		assetIDs[i] = uuid.UUID(asset.ID.Bytes).String()
	}
	response.AssetIds = assetIDs
	response.Assets = int32(len(assetIDs)) //nolint:gosec // asset count fits in int32

	return response, nil
}

// GetSharedLinkById returns a shared link owned by the authenticated user.
func (s *Server) GetSharedLinkById(ctx context.Context, request *immichv1.GetSharedLinkByIdRequest) (*immichv1.SharedLinkResponse, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	linkID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid shared link ID: %v", err)
	}

	link, err := s.sharedLinksService.GetSharedLink(ctx, userID, linkID)
	if err != nil {
		return nil, sharedLinkError(ctx, err)
	}

	return convertSharedLinkToProto(link), nil
}

// CreateSharedLink creates a new shared link for an album or a set of assets.
func (s *Server) CreateSharedLink(ctx context.Context, request *immichv1.CreateSharedLinkRequest) (*immichv1.SharedLinkResponse, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	var linkType string
	switch request.GetType() {
	case immichv1.SharedLinkType_SHARED_LINK_TYPE_ALBUM:
		linkType = sharedLinkTypeAlbum
		if request.GetAlbumId() == "" {
			return nil, status.Error(codes.InvalidArgument, "album_id is required for album shared links")
		}
	case immichv1.SharedLinkType_SHARED_LINK_TYPE_INDIVIDUAL:
		linkType = sharedLinkTypeIndividual
		if len(request.GetAssetIds()) == 0 {
			return nil, status.Error(codes.InvalidArgument, "asset_ids are required for individual shared links")
		}
	default:
		return nil, status.Error(codes.InvalidArgument, "shared link type must be ALBUM or INDIVIDUAL")
	}

	if request.AlbumId != nil {
		if _, err := uuid.Parse(request.GetAlbumId()); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid album ID: %v", err)
		}
	}

	var expiresAt *time.Time
	if request.ExpiresAt != nil {
		t := request.GetExpiresAt().AsTime()
		expiresAt = &t
	}

	// Upstream Immich defaults: allowUpload=false, allowDownload=true,
	// showMetadata=true when the fields are omitted.
	allowUpload := false
	if request.AllowUpload != nil {
		allowUpload = request.GetAllowUpload()
	}
	allowDownload := true
	if request.AllowDownload != nil {
		allowDownload = request.GetAllowDownload()
	}
	showMetadata := true
	if request.ShowMetadata != nil {
		showMetadata = request.GetShowMetadata()
	}

	link, err := s.sharedLinksService.CreateSharedLink(ctx, userID, &sharedlinks.CreateSharedLinkRequest{
		Type:          linkType,
		AssetIDs:      request.GetAssetIds(),
		AlbumID:       request.AlbumId,
		Description:   request.GetDescription(),
		Password:      request.GetPassword(),
		ExpiresAt:     expiresAt,
		AllowDownload: allowDownload,
		AllowUpload:   allowUpload,
		ShowExif:      showMetadata,
	})
	if err != nil {
		return nil, sharedLinkError(ctx, err)
	}

	response := convertSharedLinkToProto(link)
	response.AssetIds = request.GetAssetIds()
	return response, nil
}

// UpdateSharedLink updates an existing shared link owned by the authenticated user.
func (s *Server) UpdateSharedLink(ctx context.Context, request *immichv1.UpdateSharedLinkRequest) (*immichv1.SharedLinkResponse, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	linkID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid shared link ID: %v", err)
	}

	updateReq := &sharedlinks.UpdateSharedLinkRequest{
		Description:   request.Description,
		Password:      request.Password,
		AllowUpload:   request.AllowUpload,
		AllowDownload: request.AllowDownload,
		ShowExif:      request.ShowMetadata,
	}
	if request.ExpiresAt != nil {
		t := request.GetExpiresAt().AsTime()
		updateReq.ExpiresAt = &t
		updateReq.ChangeExpiryTime = true
	}

	link, err := s.sharedLinksService.UpdateSharedLink(ctx, userID, linkID, updateReq)
	if err != nil {
		return nil, sharedLinkError(ctx, err)
	}

	return convertSharedLinkToProto(link), nil
}

// RemoveSharedLink deletes a shared link owned by the authenticated user.
func (s *Server) RemoveSharedLink(ctx context.Context, request *immichv1.RemoveSharedLinkRequest) (*emptypb.Empty, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	linkID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid shared link ID: %v", err)
	}

	if err := s.sharedLinksService.DeleteSharedLink(ctx, userID, linkID); err != nil {
		return nil, sharedLinkError(ctx, err)
	}

	return &emptypb.Empty{}, nil
}

// AddSharedLinkAssets adds assets to a shared link owned by the authenticated user.
func (s *Server) AddSharedLinkAssets(ctx context.Context, request *immichv1.AddSharedLinkAssetsRequest) (*immichv1.SharedLinkResponse, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	linkID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid shared link ID: %v", err)
	}

	if len(request.GetAssetIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "asset_ids must not be empty")
	}

	if err := s.sharedLinksService.AddAssetsToSharedLink(ctx, userID, linkID, request.GetAssetIds()); err != nil {
		return nil, sharedLinkError(ctx, err)
	}

	link, err := s.sharedLinksService.GetSharedLink(ctx, userID, linkID)
	if err != nil {
		return nil, sharedLinkError(ctx, err)
	}

	return convertSharedLinkToProto(link), nil
}

// RemoveSharedLinkAssets removes assets from a shared link owned by the authenticated user.
func (s *Server) RemoveSharedLinkAssets(ctx context.Context, request *immichv1.RemoveSharedLinkAssetsRequest) (*immichv1.SharedLinkResponse, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	linkID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid shared link ID: %v", err)
	}

	if len(request.GetAssetIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "asset_ids must not be empty")
	}

	if err := s.sharedLinksService.RemoveAssetsFromSharedLink(ctx, userID, linkID, request.GetAssetIds()); err != nil {
		return nil, sharedLinkError(ctx, err)
	}

	link, err := s.sharedLinksService.GetSharedLink(ctx, userID, linkID)
	if err != nil {
		return nil, sharedLinkError(ctx, err)
	}

	return convertSharedLinkToProto(link), nil
}

// convertSharedLinkToProto converts a service-layer shared link to its proto DTO.
func convertSharedLinkToProto(link *sharedlinks.SharedLink) *immichv1.SharedLinkResponse {
	response := &immichv1.SharedLinkResponse{
		Id:            link.ID.String(),
		UserId:        link.UserID.String(),
		Key:           link.Key,
		Type:          sharedLinkTypeToProto(link.Type),
		CreatedAt:     timestamppb.New(link.CreatedAt),
		UpdatedAt:     timestamppb.New(link.UpdatedAt),
		AllowUpload:   link.AllowUpload,
		AllowDownload: link.AllowDownload,
		ShowMetadata:  link.ShowExif,
		Password:      link.Password != "",
		Assets:        int32(link.AssetCount), //nolint:gosec // asset count fits in int32
	}

	if link.Description != "" {
		response.Description = &link.Description
	}

	if link.ExpiresAt != nil {
		response.ExpiresAt = timestamppb.New(*link.ExpiresAt)
	}

	if link.AlbumID != nil {
		albumID := link.AlbumID.String()
		response.AlbumId = &albumID
	}

	return response
}

// sharedLinkTypeToProto maps the database type string to the proto enum.
func sharedLinkTypeToProto(linkType string) immichv1.SharedLinkType {
	switch linkType {
	case sharedLinkTypeAlbum:
		return immichv1.SharedLinkType_SHARED_LINK_TYPE_ALBUM
	case sharedLinkTypeIndividual:
		return immichv1.SharedLinkType_SHARED_LINK_TYPE_INDIVIDUAL
	default:
		return immichv1.SharedLinkType_SHARED_LINK_TYPE_UNSPECIFIED
	}
}

// sharedLinkError maps service-layer errors to gRPC status errors.
func sharedLinkError(ctx context.Context, err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not found"):
		return status.Error(codes.NotFound, "shared link not found")
	case strings.Contains(msg, "access denied"):
		return status.Error(codes.PermissionDenied, "access denied")
	case strings.Contains(msg, "expired"):
		return status.Error(codes.NotFound, "shared link has expired")
	case strings.Contains(msg, "password required"), strings.Contains(msg, "invalid password"):
		return status.Error(codes.Unauthenticated, "invalid password")
	case strings.Contains(msg, "invalid album ID"):
		return status.Error(codes.InvalidArgument, "invalid album ID")
	default:
		return SanitizedInternal(ctx, "shared link operation failed", err)
	}
}
