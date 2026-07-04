package server

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

// ownedAssetUUID parses the asset ID and verifies the asset belongs to the
// authenticated user, returning both UUIDs.
func (s *Server) ownedAssetUUID(ctx context.Context, assetID string) (pgtype.UUID, pgtype.UUID, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, err
	}
	userUUID, err := pgutil.StringToUUID(claims.UserID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}
	assetUUID, err := pgutil.StringToUUID(assetID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, status.Errorf(codes.InvalidArgument, "invalid asset ID: %v", err)
	}

	if _, err := s.db.GetAssetByIDAndUser(ctx, sqlc.GetAssetByIDAndUserParams{
		ID:      assetUUID,
		OwnerId: userUUID,
	}); err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, status.Error(codes.NotFound, "asset not found")
	}

	return assetUUID, userUUID, nil
}

// jsonToStruct converts stored JSONB bytes to a protobuf Struct.
func jsonToStruct(data []byte) (*structpb.Struct, error) {
	if len(data) == 0 {
		return structpb.NewStruct(nil)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return structpb.NewStruct(m)
}

// structToJSON converts a protobuf Struct to JSONB bytes.
func structToJSON(value *structpb.Struct) ([]byte, error) {
	if value == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(value.AsMap())
}

// assetMetadataToProto converts a metadata row to its proto DTO.
func assetMetadataToProto(row sqlc.AssetMetadatum) (*immichv1.AssetMetadataResponse, error) {
	value, err := jsonToStruct(row.Value)
	if err != nil {
		return nil, err
	}
	return &immichv1.AssetMetadataResponse{
		Key:       row.Key,
		Value:     value,
		UpdatedAt: timestamppb.New(row.UpdatedAt.Time),
	}, nil
}

// CopyAsset copies associations (albums, shared links, stack) and metadata
// (favorite flag, sidecar path, key/value metadata) from a source asset to a
// target asset owned by the same user.
func (s *Server) CopyAsset(ctx context.Context, request *immichv1.CopyAssetRequest) (*emptypb.Empty, error) {
	sourceUUID, _, err := s.ownedAssetUUID(ctx, request.GetSourceId())
	if err != nil {
		return nil, err
	}
	targetUUID, _, err := s.ownedAssetUUID(ctx, request.GetTargetId())
	if err != nil {
		return nil, err
	}
	if sourceUUID == targetUUID {
		return nil, status.Error(codes.InvalidArgument, "source and target asset must differ")
	}

	// All copy options default to true when omitted (per upstream spec).
	enabled := func(v *bool) bool { return v == nil || *v }

	if enabled(request.Albums) {
		if err := s.db.CopyAssetAlbums(ctx, sqlc.CopyAssetAlbumsParams{
			AssetsId:   sourceUUID,
			AssetsId_2: targetUUID,
		}); err != nil {
			return nil, SanitizedInternal(ctx, "failed to copy album associations", err)
		}
	}

	if enabled(request.Favorite) {
		if err := s.db.CopyAssetFavorite(ctx, sqlc.CopyAssetFavoriteParams{
			ID:   sourceUUID,
			ID_2: targetUUID,
		}); err != nil {
			return nil, SanitizedInternal(ctx, "failed to copy favorite status", err)
		}
	}

	if enabled(request.SharedLinks) {
		if err := s.db.CopyAssetSharedLinks(ctx, sqlc.CopyAssetSharedLinksParams{
			AssetsId:   sourceUUID,
			AssetsId_2: targetUUID,
		}); err != nil {
			return nil, SanitizedInternal(ctx, "failed to copy shared links", err)
		}
	}

	if enabled(request.Sidecar) {
		if err := s.db.CopyAssetSidecar(ctx, sqlc.CopyAssetSidecarParams{
			ID:   sourceUUID,
			ID_2: targetUUID,
		}); err != nil {
			return nil, SanitizedInternal(ctx, "failed to copy sidecar", err)
		}
	}

	if enabled(request.Stack) {
		if err := s.db.CopyAssetStack(ctx, sqlc.CopyAssetStackParams{
			ID:   sourceUUID,
			ID_2: targetUUID,
		}); err != nil {
			return nil, SanitizedInternal(ctx, "failed to copy stack association", err)
		}
	}

	if err := s.db.CopyAssetMetadata(ctx, sqlc.CopyAssetMetadataParams{
		AssetId:   sourceUUID,
		AssetId_2: targetUUID,
	}); err != nil {
		return nil, SanitizedInternal(ctx, "failed to copy asset metadata", err)
	}

	return &emptypb.Empty{}, nil
}

// GetAssetMetadata returns all metadata key/value pairs for an asset.
func (s *Server) GetAssetMetadata(ctx context.Context, request *immichv1.GetAssetMetadataRequest) (*immichv1.GetAssetMetadataResponse, error) {
	assetUUID, _, err := s.ownedAssetUUID(ctx, request.GetId())
	if err != nil {
		return nil, err
	}

	rows, err := s.db.ListAssetMetadata(ctx, assetUUID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to list asset metadata", err)
	}

	items := make([]*immichv1.AssetMetadataResponse, 0, len(rows))
	for _, row := range rows {
		item, err := assetMetadataToProto(row)
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to decode asset metadata", err)
		}
		items = append(items, item)
	}

	return &immichv1.GetAssetMetadataResponse{Items: items}, nil
}

// UpdateAssetMetadata upserts metadata key/value pairs for an asset.
func (s *Server) UpdateAssetMetadata(ctx context.Context, request *immichv1.UpdateAssetMetadataRequest) (*immichv1.GetAssetMetadataResponse, error) {
	assetUUID, _, err := s.ownedAssetUUID(ctx, request.GetId())
	if err != nil {
		return nil, err
	}

	items := make([]*immichv1.AssetMetadataResponse, 0, len(request.GetItems()))
	for _, upsert := range request.GetItems() {
		if upsert.GetKey() == "" {
			return nil, status.Error(codes.InvalidArgument, "metadata key must not be empty")
		}
		value, err := structToJSON(upsert.GetValue())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid metadata value: %v", err)
		}

		row, err := s.db.UpsertAssetMetadata(ctx, sqlc.UpsertAssetMetadataParams{
			AssetId: assetUUID,
			Key:     upsert.GetKey(),
			Value:   value,
		})
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to upsert asset metadata", err)
		}

		item, err := assetMetadataToProto(row)
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to decode asset metadata", err)
		}
		items = append(items, item)
	}

	return &immichv1.GetAssetMetadataResponse{Items: items}, nil
}

// GetAssetMetadataByKey returns a single metadata item by key.
func (s *Server) GetAssetMetadataByKey(ctx context.Context, request *immichv1.GetAssetMetadataByKeyRequest) (*immichv1.AssetMetadataResponse, error) {
	assetUUID, _, err := s.ownedAssetUUID(ctx, request.GetId())
	if err != nil {
		return nil, err
	}

	row, err := s.db.GetAssetMetadataByKey(ctx, sqlc.GetAssetMetadataByKeyParams{
		AssetId: assetUUID,
		Key:     request.GetKey(),
	})
	if err != nil {
		return nil, status.Error(codes.NotFound, "metadata key not found")
	}

	item, err := assetMetadataToProto(row)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to decode asset metadata", err)
	}
	return item, nil
}

// DeleteAssetMetadata deletes a single metadata item by key.
func (s *Server) DeleteAssetMetadata(ctx context.Context, request *immichv1.DeleteAssetMetadataRequest) (*emptypb.Empty, error) {
	assetUUID, _, err := s.ownedAssetUUID(ctx, request.GetId())
	if err != nil {
		return nil, err
	}

	if err := s.db.DeleteAssetMetadata(ctx, sqlc.DeleteAssetMetadataParams{
		AssetId: assetUUID,
		Key:     request.GetKey(),
	}); err != nil {
		return nil, SanitizedInternal(ctx, "failed to delete asset metadata", err)
	}

	return &emptypb.Empty{}, nil
}

// UpdateBulkAssetMetadata upserts metadata for multiple assets in one call.
func (s *Server) UpdateBulkAssetMetadata(ctx context.Context, request *immichv1.UpdateBulkAssetMetadataRequest) (*immichv1.UpdateBulkAssetMetadataResponse, error) {
	items := make([]*immichv1.AssetMetadataBulkResponse, 0, len(request.GetItems()))
	for _, upsert := range request.GetItems() {
		assetUUID, _, err := s.ownedAssetUUID(ctx, upsert.GetAssetId())
		if err != nil {
			return nil, err
		}
		if upsert.GetKey() == "" {
			return nil, status.Error(codes.InvalidArgument, "metadata key must not be empty")
		}
		value, err := structToJSON(upsert.GetValue())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid metadata value: %v", err)
		}

		row, err := s.db.UpsertAssetMetadata(ctx, sqlc.UpsertAssetMetadataParams{
			AssetId: assetUUID,
			Key:     upsert.GetKey(),
			Value:   value,
		})
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to upsert asset metadata", err)
		}

		protoValue, err := jsonToStruct(row.Value)
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to decode asset metadata", err)
		}
		items = append(items, &immichv1.AssetMetadataBulkResponse{
			AssetId:   upsert.GetAssetId(),
			Key:       row.Key,
			Value:     protoValue,
			UpdatedAt: timestamppb.New(row.UpdatedAt.Time),
		})
	}

	return &immichv1.UpdateBulkAssetMetadataResponse{Items: items}, nil
}

// DeleteBulkAssetMetadata deletes metadata items for multiple assets.
func (s *Server) DeleteBulkAssetMetadata(ctx context.Context, request *immichv1.DeleteBulkAssetMetadataRequest) (*emptypb.Empty, error) {
	for _, item := range request.GetItems() {
		assetUUID, _, err := s.ownedAssetUUID(ctx, item.GetAssetId())
		if err != nil {
			return nil, err
		}

		if err := s.db.DeleteAssetMetadata(ctx, sqlc.DeleteAssetMetadataParams{
			AssetId: assetUUID,
			Key:     item.GetKey(),
		}); err != nil {
			return nil, SanitizedInternal(ctx, "failed to delete asset metadata", err)
		}
	}

	return &emptypb.Empty{}, nil
}

// assetEditsResponse loads and converts the edits for an asset.
func (s *Server) assetEditsResponse(ctx context.Context, assetID string, assetUUID pgtype.UUID) (*immichv1.AssetEditsResponse, error) {
	rows, err := s.db.GetAssetEdits(ctx, assetUUID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to list asset edits", err)
	}

	edits := make([]*immichv1.AssetEditActionItemResponse, 0, len(rows))
	for _, row := range rows {
		parameters, err := jsonToStruct(row.Parameters)
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to decode asset edit parameters", err)
		}
		edits = append(edits, &immichv1.AssetEditActionItemResponse{
			Id:         pgutil.UUIDToString(row.ID),
			Action:     row.Action,
			Parameters: parameters,
		})
	}

	return &immichv1.AssetEditsResponse{AssetId: assetID, Edits: edits}, nil
}

// GetAssetEdits returns the stored edit actions for an asset.
func (s *Server) GetAssetEdits(ctx context.Context, request *immichv1.GetAssetEditsRequest) (*immichv1.AssetEditsResponse, error) {
	assetUUID, _, err := s.ownedAssetUUID(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return s.assetEditsResponse(ctx, request.GetId(), assetUUID)
}

// EditAsset replaces the stored edit actions for an asset.
func (s *Server) EditAsset(ctx context.Context, request *immichv1.EditAssetRequest) (*immichv1.AssetEditsResponse, error) {
	assetUUID, _, err := s.ownedAssetUUID(ctx, request.GetId())
	if err != nil {
		return nil, err
	}

	// Replace the whole edit stack, mirroring upstream semantics.
	if err := s.db.DeleteAssetEdits(ctx, assetUUID); err != nil {
		return nil, SanitizedInternal(ctx, "failed to clear asset edits", err)
	}

	for i, edit := range request.GetEdits() {
		if edit.GetAction() == "" {
			return nil, status.Error(codes.InvalidArgument, "edit action must not be empty")
		}
		parameters, err := structToJSON(edit.GetParameters())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid edit parameters: %v", err)
		}

		if _, err := s.db.CreateAssetEdit(ctx, sqlc.CreateAssetEditParams{
			AssetId:    assetUUID,
			Action:     edit.GetAction(),
			Parameters: parameters,
			Position:   int32(i), //nolint:gosec // edit count fits in int32
		}); err != nil {
			return nil, SanitizedInternal(ctx, "failed to store asset edit", err)
		}
	}

	return s.assetEditsResponse(ctx, request.GetId(), assetUUID)
}

// RemoveAssetEdits removes all stored edit actions for an asset.
func (s *Server) RemoveAssetEdits(ctx context.Context, request *immichv1.RemoveAssetEditsRequest) (*emptypb.Empty, error) {
	assetUUID, _, err := s.ownedAssetUUID(ctx, request.GetId())
	if err != nil {
		return nil, err
	}

	if err := s.db.DeleteAssetEdits(ctx, assetUUID); err != nil {
		return nil, SanitizedInternal(ctx, "failed to remove asset edits", err)
	}

	return &emptypb.Empty{}, nil
}

// GetAssetOcr returns the OCR rows detected for an asset.
func (s *Server) GetAssetOcr(ctx context.Context, request *immichv1.GetAssetOcrRequest) (*immichv1.GetAssetOcrResponse, error) {
	assetUUID, _, err := s.ownedAssetUUID(ctx, request.GetId())
	if err != nil {
		return nil, err
	}

	rows, err := s.db.GetAssetOcr(ctx, assetUUID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to list asset OCR entries", err)
	}

	items := make([]*immichv1.AssetOcrResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, &immichv1.AssetOcrResponse{
			Id:        pgutil.UUIDToString(row.ID),
			AssetId:   pgutil.UUIDToString(row.AssetId),
			Text:      row.Text,
			TextScore: row.TextScore,
			BoxScore:  row.BoxScore,
			X1:        row.X1,
			Y1:        row.Y1,
			X2:        row.X2,
			Y2:        row.Y2,
			X3:        row.X3,
			Y3:        row.Y3,
			X4:        row.X4,
			Y4:        row.Y4,
		})
	}

	return &immichv1.GetAssetOcrResponse{Items: items}, nil
}
