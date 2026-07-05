package stacks

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCurrentUserIDFromContext(t *testing.T) {
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")

	got, err := currentUserIDFromContext(auth.WithClaims(context.Background(), &auth.Claims{UserID: userID.String()}))
	require.NoError(t, err)
	assert.Equal(t, userID.String(), got)

	_, err = currentUserIDFromContext(context.Background())
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = currentUserIDFromContext(auth.WithClaims(context.Background(), &auth.Claims{UserID: "not-a-uuid"}))
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestStackResponse(t *testing.T) {
	createdAt := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 7, 5, 10, 30, 0, 0, time.UTC)

	resp := stackResponse(&StackResponse{
		ID:             "stack-id",
		PrimaryAssetID: "asset-1",
		AssetIDs:       []string{"asset-1", "asset-2"},
		AssetCount:     2,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	})

	require.NotNil(t, resp)
	assert.Equal(t, "stack-id", resp.GetId())
	assert.Equal(t, "asset-1", resp.GetPrimaryAssetId())
	assert.Equal(t, []string{"asset-1", "asset-2"}, resp.GetAssetIds())
	assert.Equal(t, int32(2), resp.GetAssetCount())
	assert.Equal(t, createdAt, resp.GetCreatedAt().AsTime())
	assert.Equal(t, updatedAt, resp.GetUpdatedAt().AsTime())
}

func TestCreateStackUsesAuthenticatedUser(t *testing.T) {
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	fake := &fakeStackService{}
	server := NewServer(fake)

	_, err := server.CreateStack(
		auth.WithClaims(context.Background(), &auth.Claims{UserID: userID.String()}),
		&immichv1.CreateStackRequest{AssetIds: []string{"asset-id"}},
	)
	require.NoError(t, err)
	assert.Equal(t, userID.String(), fake.createUserID)
}

func TestStackLookupAndMutationsUseAuthenticatedUser(t *testing.T) {
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: userID.String()})
	fake := &fakeStackService{}
	server := NewServer(fake)

	_, err := server.GetStack(ctx, &immichv1.GetStackRequest{Id: "stack-id"})
	require.NoError(t, err)
	assert.Equal(t, userID.String(), fake.getUserID)

	primaryAssetID := "asset-2"
	_, err = server.UpdateStack(ctx, &immichv1.UpdateStackRequest{
		Id:             "stack-id",
		PrimaryAssetId: &primaryAssetID,
	})
	require.NoError(t, err)
	assert.Equal(t, userID.String(), fake.updateUserID)

	_, err = server.DeleteStack(ctx, &immichv1.DeleteStackRequest{Id: "stack-id"})
	require.NoError(t, err)
	assert.Equal(t, userID.String(), fake.deleteUserID)

	_, err = server.DeleteStacks(ctx, &immichv1.DeleteStacksRequest{Ids: []string{"stack-id"}})
	require.NoError(t, err)
	assert.Equal(t, userID.String(), fake.deleteManyUserID)
}

func TestSearchStacksUsesAuthenticatedUser(t *testing.T) {
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	requestedUserID := "22222222-3333-4444-5555-666666666666"
	primaryAssetID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	fake := &fakeStackService{
		searchStacksResponse: &SearchStacksResponse{
			Stacks: []*StackResponse{stackFixture("stack-id", primaryAssetID)},
		},
	}
	server := NewServer(fake)

	resp, err := server.SearchStacks(
		auth.WithClaims(context.Background(), &auth.Claims{UserID: userID.String()}),
		&immichv1.SearchStacksRequest{
			UserId:         stringPtr(requestedUserID),
			PrimaryAssetId: stringPtr(primaryAssetID),
		},
	)
	require.NoError(t, err)
	require.Len(t, resp.GetStacks(), 1)
	assert.Equal(t, "stack-id", resp.GetStacks()[0].GetId())
	require.Equal(t, 1, fake.searchStacksCalls)
	require.NotNil(t, fake.searchStacksRequest.UserID)
	assert.Equal(t, userID.String(), *fake.searchStacksRequest.UserID)
	require.NotNil(t, fake.searchStacksRequest.PrimaryAssetID)
	assert.Equal(t, primaryAssetID, *fake.searchStacksRequest.PrimaryAssetID)
}

func TestSearchStacksRequiresAuth(t *testing.T) {
	fake := &fakeStackService{}
	server := NewServer(fake)

	_, err := server.SearchStacks(context.Background(), &immichv1.SearchStacksRequest{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Zero(t, fake.searchStacksCalls)
}

func TestRemoveAssetFromStackUsesAuthenticatedUser(t *testing.T) {
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	fake := &fakeStackService{}
	server := NewServer(fake)

	_, err := server.RemoveAssetFromStack(
		auth.WithClaims(context.Background(), &auth.Claims{UserID: userID.String()}),
		&immichv1.RemoveAssetFromStackRequest{Id: "stack-id", AssetId: "asset-id"},
	)
	require.NoError(t, err)
	assert.Equal(t, userID.String(), fake.removeUserID)
	assert.Equal(t, "stack-id", fake.removeStackID)
	assert.Equal(t, "asset-id", fake.removeAssetID)
}

func TestStackStatusErrorMapsCommonServiceErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{
			name: "invalid argument",
			err:  errors.New("invalid stack ID: invalid UUID"),
			code: codes.InvalidArgument,
		},
		{
			name: "required argument",
			err:  errors.New("user ID is required for search"),
			code: codes.InvalidArgument,
		},
		{
			name: "pgx not found",
			err:  fmt.Errorf("failed to get stack: %w", pgx.ErrNoRows),
			code: codes.NotFound,
		},
		{
			name: "string not found",
			err:  errors.New("asset not found: asset is not part of this stack"),
			code: codes.NotFound,
		},
		{
			name: "permission denied",
			err:  errors.New("access denied: stack is not owned by the user"),
			code: codes.PermissionDenied,
		},
		{
			name: "passes through status errors",
			err:  status.Error(codes.AlreadyExists, "stack exists"),
			code: codes.AlreadyExists,
		},
		{
			name: "internal fallback",
			err:  errors.New("database unavailable"),
			code: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := stackStatusError(tt.err, "fallback")
			assert.Equal(t, tt.code, status.Code(err))
		})
	}
}

type fakeStackService struct {
	createStackCalls int
	createUserID     string

	getUserID string

	updateUserID string

	deleteUserID     string
	deleteManyUserID string

	searchStacksCalls    int
	searchStacksRequest  SearchStacksRequest
	searchStacksResponse *SearchStacksResponse
	searchStacksErr      error

	removeUserID  string
	removeStackID string
	removeAssetID string
	removeErr     error
}

func (f *fakeStackService) CreateStack(ctx context.Context, userID string, req CreateStackRequest) (*StackResponse, error) {
	f.createStackCalls++
	f.createUserID = userID
	return stackFixture("stack-id", firstString(req.AssetIDs)), nil
}

func (f *fakeStackService) GetStack(ctx context.Context, userID, stackID string) (*StackResponse, error) {
	f.getUserID = userID
	return stackFixture(stackID, "asset-id"), nil
}

func (f *fakeStackService) UpdateStack(ctx context.Context, userID, stackID string, req UpdateStackRequest) (*StackResponse, error) {
	f.updateUserID = userID
	primaryAssetID := "asset-id"
	if req.PrimaryAssetID != nil {
		primaryAssetID = *req.PrimaryAssetID
	}

	return stackFixture(stackID, primaryAssetID), nil
}

func (f *fakeStackService) DeleteStack(ctx context.Context, userID, stackID string) error {
	f.deleteUserID = userID
	return nil
}

func (f *fakeStackService) DeleteStacks(ctx context.Context, userID string, stackIDs []string) error {
	f.deleteManyUserID = userID
	return nil
}

func (f *fakeStackService) SearchStacks(ctx context.Context, req SearchStacksRequest) (*SearchStacksResponse, error) {
	f.searchStacksCalls++
	f.searchStacksRequest = req
	if f.searchStacksErr != nil {
		return nil, f.searchStacksErr
	}
	if f.searchStacksResponse != nil {
		return f.searchStacksResponse, nil
	}

	return &SearchStacksResponse{}, nil
}

func (f *fakeStackService) RemoveAssetFromStack(ctx context.Context, userID, stackID, assetID string) error {
	f.removeUserID = userID
	f.removeStackID = stackID
	f.removeAssetID = assetID
	return f.removeErr
}

func stackFixture(stackID, primaryAssetID string) *StackResponse {
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	return &StackResponse{
		ID:             stackID,
		PrimaryAssetID: primaryAssetID,
		AssetIDs:       []string{primaryAssetID},
		AssetCount:     1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func stringPtr(value string) *string {
	return &value
}
