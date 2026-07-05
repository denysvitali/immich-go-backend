package tags

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func TestCurrentUserIDFromContext(t *testing.T) {
	userID := uuid.New()

	got, err := currentUserIDFromContext(auth.WithClaims(context.Background(), &auth.Claims{UserID: userID.String()}))
	require.NoError(t, err)
	assert.Equal(t, userID, got)

	_, err = currentUserIDFromContext(context.Background())
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = currentUserIDFromContext(auth.WithClaims(context.Background(), &auth.Claims{UserID: "not-a-uuid"}))
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestTagResponse(t *testing.T) {
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	tagID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	createdAt := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 7, 5, 10, 30, 0, 0, time.UTC)

	resp := tagResponse(sqlc.Tag{
		ID:        pgUUID(tagID),
		UserId:    pgUUID(userID),
		Value:     "travel",
		CreatedAt: pgtype.Timestamptz{Time: createdAt, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: updatedAt, Valid: true},
		Color:     pgtype.Text{String: "#123456", Valid: true},
	})

	require.NotNil(t, resp)
	assert.Equal(t, tagID.String(), resp.GetId())
	assert.Equal(t, userID.String(), resp.GetUserId())
	assert.Equal(t, "travel", resp.GetName())
	require.NotNil(t, resp.Color)
	assert.Equal(t, "#123456", resp.GetColor())
	assert.Equal(t, createdAt, resp.GetCreatedAt().AsTime())
	assert.Equal(t, updatedAt, resp.GetUpdatedAt().AsTime())
}

func TestGetOwnedTag(t *testing.T) {
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	otherUserID := uuid.MustParse("22222222-3333-4444-5555-666666666666")
	tagID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	fake := newFakeTagQueries(tagFixture(tagID, userID, "owned"))
	server := &Server{queries: fake}

	got, err := server.getOwnedTag(context.Background(), pgUUID(tagID), userID)
	require.NoError(t, err)
	assert.Equal(t, "owned", got.Value)

	_, err = server.getOwnedTag(context.Background(), pgUUID(tagID), otherUserID)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = server.getOwnedTag(context.Background(), pgUUID(uuid.New()), userID)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestUpsertTagsReusesExistingTagsAndCachesCreatedTags(t *testing.T) {
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	existingID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	createdID := uuid.MustParse("bbbbbbbb-cccc-dddd-eeee-ffffffffffff")
	fake := newFakeTagQueries(tagFixture(existingID, userID, "existing"))
	fake.nextCreateIDs = []uuid.UUID{createdID}
	server := &Server{queries: fake}

	resp, err := server.UpsertTags(auth.WithClaims(context.Background(), &auth.Claims{UserID: userID.String()}), &immichv1.UpsertTagsRequest{
		Tags: []*immichv1.TagUpsert{
			{Name: "existing"},
			{Name: "new"},
			{Name: "new"},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.GetTags(), 3)
	assert.Equal(t, existingID.String(), resp.GetTags()[0].GetId())
	assert.Equal(t, createdID.String(), resp.GetTags()[1].GetId())
	assert.Equal(t, createdID.String(), resp.GetTags()[2].GetId())
	assert.Equal(t, 1, fake.getTagsCalls)
	require.Len(t, fake.createTagCalls, 1)
	assert.Equal(t, "new", fake.createTagCalls[0].Value)
}

func TestBulkTagAssetsCountsSuccessfulWritesForOwnedTags(t *testing.T) {
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	otherUserID := uuid.MustParse("22222222-3333-4444-5555-666666666666")
	ownedTagID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	otherTagID := uuid.MustParse("bbbbbbbb-cccc-dddd-eeee-ffffffffffff")
	assetID1 := uuid.MustParse("cccccccc-dddd-eeee-ffff-000000000000")
	assetID2 := uuid.MustParse("dddddddd-eeee-ffff-0000-111111111111")
	fake := newFakeTagQueries(
		tagFixture(ownedTagID, userID, "owned"),
		tagFixture(otherTagID, otherUserID, "other"),
	)
	server := &Server{queries: fake}

	resp, err := server.BulkTagAssets(auth.WithClaims(context.Background(), &auth.Claims{UserID: userID.String()}), &immichv1.BulkTagAssetsRequest{
		TagIds:   []string{"not-a-uuid", ownedTagID.String(), otherTagID.String()},
		AssetIds: []string{assetID1.String(), "not-a-uuid", assetID2.String()},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.GetCount())
	require.Len(t, fake.addTagToAssetCalls, 2)
	assert.Equal(t, pgUUID(ownedTagID), fake.addTagToAssetCalls[0].TagsId)
	assert.Equal(t, pgUUID(assetID1), fake.addTagToAssetCalls[0].AssetsId)
	assert.Equal(t, pgUUID(ownedTagID), fake.addTagToAssetCalls[1].TagsId)
	assert.Equal(t, pgUUID(assetID2), fake.addTagToAssetCalls[1].AssetsId)
}

type fakeTagQueries struct {
	tagsByID              map[uuid.UUID]sqlc.Tag
	tags                  []sqlc.Tag
	getTagsCalls          int
	createTagCalls        []sqlc.CreateTagParams
	updateTagCalls        []sqlc.UpdateTagParams
	deleteTagCalls        []pgtype.UUID
	addTagToAssetCalls    []sqlc.AddTagToAssetParams
	removeTagToAssetCalls []sqlc.RemoveTagFromAssetParams
	nextCreateIDs         []uuid.UUID
	now                   time.Time
}

func newFakeTagQueries(tags ...sqlc.Tag) *fakeTagQueries {
	fake := &fakeTagQueries{
		tagsByID: make(map[uuid.UUID]sqlc.Tag, len(tags)),
		tags:     tags,
		now:      time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
	}
	for _, tag := range tags {
		fake.tagsByID[uuid.UUID(tag.ID.Bytes)] = tag
	}

	return fake
}

func (f *fakeTagQueries) GetTags(ctx context.Context, userid pgtype.UUID) ([]sqlc.Tag, error) {
	f.getTagsCalls++
	tags := make([]sqlc.Tag, 0, len(f.tags))
	for _, tag := range f.tags {
		if tag.UserId.Bytes == userid.Bytes {
			tags = append(tags, tag)
		}
	}

	return tags, nil
}

func (f *fakeTagQueries) CreateTag(ctx context.Context, arg sqlc.CreateTagParams) (sqlc.Tag, error) {
	f.createTagCalls = append(f.createTagCalls, arg)
	tagID := uuid.New()
	if len(f.nextCreateIDs) > 0 {
		tagID = f.nextCreateIDs[0]
		f.nextCreateIDs = f.nextCreateIDs[1:]
	}
	tag := sqlc.Tag{
		ID:        pgUUID(tagID),
		UserId:    arg.UserId,
		Value:     arg.Value,
		CreatedAt: pgtype.Timestamptz{Time: f.now, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: f.now, Valid: true},
		Color:     arg.Color,
	}
	f.tags = append(f.tags, tag)
	f.tagsByID[tagID] = tag

	return tag, nil
}

func (f *fakeTagQueries) GetTag(ctx context.Context, id pgtype.UUID) (sqlc.Tag, error) {
	tag, ok := f.tagsByID[uuid.UUID(id.Bytes)]
	if !ok {
		return sqlc.Tag{}, errors.New("not found")
	}

	return tag, nil
}

func (f *fakeTagQueries) DeleteTag(ctx context.Context, id pgtype.UUID) error {
	f.deleteTagCalls = append(f.deleteTagCalls, id)
	return nil
}

func (f *fakeTagQueries) UpdateTag(ctx context.Context, arg sqlc.UpdateTagParams) (sqlc.Tag, error) {
	f.updateTagCalls = append(f.updateTagCalls, arg)
	tag, ok := f.tagsByID[uuid.UUID(arg.ID.Bytes)]
	if !ok {
		return sqlc.Tag{}, errors.New("not found")
	}
	tag.Value = arg.Value.String
	tag.Color = arg.Color
	tag.UpdatedAt = pgtype.Timestamptz{Time: f.now, Valid: true}
	f.tagsByID[uuid.UUID(arg.ID.Bytes)] = tag

	return tag, nil
}

func (f *fakeTagQueries) AddTagToAsset(ctx context.Context, arg sqlc.AddTagToAssetParams) error {
	f.addTagToAssetCalls = append(f.addTagToAssetCalls, arg)
	return nil
}

func (f *fakeTagQueries) RemoveTagFromAsset(ctx context.Context, arg sqlc.RemoveTagFromAssetParams) error {
	f.removeTagToAssetCalls = append(f.removeTagToAssetCalls, arg)
	return nil
}

func tagFixture(tagID uuid.UUID, userID uuid.UUID, name string) sqlc.Tag {
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	return sqlc.Tag{
		ID:        pgUUID(tagID),
		UserId:    pgUUID(userID),
		Value:     name,
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}
}
