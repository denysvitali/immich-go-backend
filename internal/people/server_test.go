package people

import (
	"context"
	"errors"
	"testing"
	"time"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
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

func TestBuildPersonResponse(t *testing.T) {
	personID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	updatedAt := time.Date(2026, 7, 5, 12, 30, 0, 0, time.UTC)
	person := sqlc.Person{
		ID:            pgUUID(personID),
		UpdatedAt:     pgtype.Timestamptz{Time: updatedAt, Valid: true},
		Name:          "Ada",
		ThumbnailPath: "thumbs/ada.webp",
		IsHidden:      true,
		BirthDate:     pgtype.Date{Time: time.Date(1815, 12, 10, 0, 0, 0, 0, time.UTC), Valid: true},
	}

	resp := buildPersonResponse(person, 7)
	require.NotNil(t, resp)
	assert.Equal(t, personID.String(), resp.GetId())
	assert.Equal(t, "Ada", resp.GetName())
	assert.Equal(t, "1815-12-10", resp.GetBirthDate())
	assert.Equal(t, "thumbs/ada.webp", resp.GetThumbnailPath())
	assert.Equal(t, int32(7), resp.GetFaces())
	assert.True(t, resp.GetIsHidden())
	assert.Equal(t, updatedAt, resp.GetUpdatedAt().AsTime())
}

type fakePeopleQueries struct {
	people         map[uuid.UUID]sqlc.Person
	getErrors      map[uuid.UUID]error
	faceCounts     map[uuid.UUID]int64
	faceCountErrs  map[uuid.UUID]error
	mergeResult    sqlc.Person
	mergeErr       error
	mergeCalls     []sqlc.MergePeopleParams
	reassignResult sqlc.Person
	reassignErr    error
	reassignCalls  []sqlc.ReassignFacesParams
	deletedPeople  []pgtype.UUID
}

func (f *fakePeopleQueries) CountPersonAssets(_ context.Context, personID pgtype.UUID) (int64, error) {
	id := uuid.UUID(personID.Bytes)
	return f.faceCounts[id], f.faceCountErrs[id]
}

func (f *fakePeopleQueries) CreatePerson(context.Context, sqlc.CreatePersonParams) (sqlc.Person, error) {
	return sqlc.Person{}, errors.New("unexpected CreatePerson call")
}

func (f *fakePeopleQueries) DeletePerson(_ context.Context, id pgtype.UUID) error {
	f.deletedPeople = append(f.deletedPeople, id)
	return nil
}

func (f *fakePeopleQueries) GetPeople(context.Context, pgtype.UUID) ([]sqlc.Person, error) {
	return nil, errors.New("unexpected GetPeople call")
}

func (f *fakePeopleQueries) GetPerson(_ context.Context, id pgtype.UUID) (sqlc.Person, error) {
	personID := uuid.UUID(id.Bytes)
	if err := f.getErrors[personID]; err != nil {
		return sqlc.Person{}, err
	}
	person, ok := f.people[personID]
	if !ok {
		return sqlc.Person{}, pgx.ErrNoRows
	}
	return person, nil
}

func (f *fakePeopleQueries) MergePeople(_ context.Context, params sqlc.MergePeopleParams) (sqlc.Person, error) {
	f.mergeCalls = append(f.mergeCalls, params)
	if f.mergeErr != nil {
		return sqlc.Person{}, f.mergeErr
	}
	return f.mergeResult, nil
}

func (f *fakePeopleQueries) ReassignFaces(_ context.Context, params sqlc.ReassignFacesParams) (sqlc.Person, error) {
	f.reassignCalls = append(f.reassignCalls, params)
	if f.reassignErr != nil {
		return sqlc.Person{}, f.reassignErr
	}
	return f.reassignResult, nil
}

func (f *fakePeopleQueries) UpdatePerson(context.Context, sqlc.UpdatePersonParams) (sqlc.Person, error) {
	return sqlc.Person{}, errors.New("unexpected UpdatePerson call")
}

func testPerson(id, ownerID uuid.UUID, name string) sqlc.Person {
	return sqlc.Person{
		ID:        pgUUID(id),
		OwnerId:   pgUUID(ownerID),
		Name:      name,
		UpdatedAt: pgtype.Timestamptz{Time: time.Unix(1, 0).UTC(), Valid: true},
	}
}

func userContext(userID uuid.UUID) context.Context {
	return auth.WithClaims(context.Background(), &auth.Claims{UserID: userID.String()})
}

func TestMergePersonReassignsAllOwnedSourcesAtomically(t *testing.T) {
	ownerID := uuid.New()
	targetID := uuid.New()
	sourceOneID := uuid.New()
	sourceTwoID := uuid.New()
	target := testPerson(targetID, ownerID, "target")
	store := &fakePeopleQueries{
		people: map[uuid.UUID]sqlc.Person{
			targetID:    target,
			sourceOneID: testPerson(sourceOneID, ownerID, "source one"),
			sourceTwoID: testPerson(sourceTwoID, ownerID, "source two"),
		},
		faceCounts:  map[uuid.UUID]int64{targetID: 4},
		mergeResult: target,
	}
	server := &Server{queries: store}

	response, err := server.MergePerson(userContext(ownerID), &immichv1.MergePersonRequest{
		Id:  targetID.String(),
		Ids: []string{sourceOneID.String(), sourceTwoID.String(), sourceOneID.String()},
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, targetID.String(), response.GetPerson().GetId())
	assert.Equal(t, int32(4), response.GetPerson().GetFaces())
	require.Len(t, store.mergeCalls, 1)
	assert.Equal(t, pgUUID(targetID), store.mergeCalls[0].TargetID)
	assert.Equal(t, pgUUID(ownerID), store.mergeCalls[0].OwnerID)
	assert.Equal(t, []pgtype.UUID{pgUUID(sourceOneID), pgUUID(sourceTwoID)}, store.mergeCalls[0].SourceIds)
	assert.Empty(t, store.deletedPeople, "MergePerson must not perform non-transactional deletes")
}

func TestMergePersonRejectsCrossUserSourceBeforeMutation(t *testing.T) {
	ownerID := uuid.New()
	otherOwnerID := uuid.New()
	targetID := uuid.New()
	sourceID := uuid.New()
	store := &fakePeopleQueries{
		people: map[uuid.UUID]sqlc.Person{
			targetID: testPerson(targetID, ownerID, "target"),
			sourceID: testPerson(sourceID, otherOwnerID, "other user's source"),
		},
	}
	server := &Server{queries: store}

	response, err := server.MergePerson(userContext(ownerID), &immichv1.MergePersonRequest{
		Id:  targetID.String(),
		Ids: []string{sourceID.String()},
	})
	assert.Nil(t, response)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, store.mergeCalls)
	assert.Empty(t, store.deletedPeople)
}

func TestMergePersonRejectsInvalidSourceSetBeforeMutation(t *testing.T) {
	ownerID := uuid.New()
	targetID := uuid.New()
	missingID := uuid.New()
	target := testPerson(targetID, ownerID, "target")

	tests := []struct {
		name     string
		sourceID string
		wantCode codes.Code
	}{
		{name: "malformed", sourceID: "not-a-uuid", wantCode: codes.InvalidArgument},
		{name: "missing", sourceID: missingID.String(), wantCode: codes.NotFound},
		{name: "target itself", sourceID: targetID.String(), wantCode: codes.InvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakePeopleQueries{people: map[uuid.UUID]sqlc.Person{targetID: target}}
			server := &Server{queries: store}
			response, err := server.MergePerson(userContext(ownerID), &immichv1.MergePersonRequest{
				Id:  targetID.String(),
				Ids: []string{tt.sourceID},
			})
			assert.Nil(t, response)
			assert.Equal(t, tt.wantCode, status.Code(err))
			assert.Empty(t, store.mergeCalls)
			assert.Empty(t, store.deletedPeople)
		})
	}
}

func TestMergePersonReportsReadAndWriteFailures(t *testing.T) {
	ownerID := uuid.New()
	targetID := uuid.New()
	sourceID := uuid.New()
	target := testPerson(targetID, ownerID, "target")
	source := testPerson(sourceID, ownerID, "source")

	tests := []struct {
		name       string
		getErrors  map[uuid.UUID]error
		mergeError error
		wantCode   codes.Code
	}{
		{name: "source read", getErrors: map[uuid.UUID]error{sourceID: errors.New("database unavailable")}, wantCode: codes.Internal},
		{name: "merge query", mergeError: errors.New("write failed"), wantCode: codes.Internal},
		{name: "concurrent source change", mergeError: pgx.ErrNoRows, wantCode: codes.Aborted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakePeopleQueries{
				people:      map[uuid.UUID]sqlc.Person{targetID: target, sourceID: source},
				getErrors:   tt.getErrors,
				mergeResult: target,
				mergeErr:    tt.mergeError,
			}
			server := &Server{queries: store}
			response, err := server.MergePerson(userContext(ownerID), &immichv1.MergePersonRequest{
				Id:  targetID.String(),
				Ids: []string{sourceID.String()},
			})
			assert.Nil(t, response)
			assert.Equal(t, tt.wantCode, status.Code(err))
			assert.Empty(t, store.deletedPeople)
		})
	}
}

func TestReassignFacesMovesEveryOwnedFaceAtomically(t *testing.T) {
	ownerID := uuid.New()
	targetID := uuid.New()
	sourceOneID := uuid.New()
	sourceTwoID := uuid.New()
	assetOneID := uuid.New()
	assetTwoID := uuid.New()
	target := testPerson(targetID, ownerID, "target")
	store := &fakePeopleQueries{
		people: map[uuid.UUID]sqlc.Person{
			targetID:    target,
			sourceOneID: testPerson(sourceOneID, ownerID, "source one"),
			sourceTwoID: testPerson(sourceTwoID, ownerID, "source two"),
		},
		faceCounts:     map[uuid.UUID]int64{targetID: 2},
		reassignResult: target,
	}
	server := &Server{queries: store}

	response, err := server.ReassignFaces(userContext(ownerID), &immichv1.ReassignFacesRequest{
		Id: targetID.String(),
		FaceUpdates: []*immichv1.FaceUpdate{
			{AssetId: assetOneID.String(), PersonId: sourceOneID.String()},
			{AssetId: assetTwoID.String(), PersonId: sourceTwoID.String()},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Len(t, response.GetPeople(), 2)
	for _, person := range response.GetPeople() {
		assert.Equal(t, targetID.String(), person.GetId())
		assert.Equal(t, int32(2), person.GetFaces())
	}
	require.Len(t, store.reassignCalls, 1)
	assert.Equal(t, pgUUID(targetID), store.reassignCalls[0].TargetPersonID)
	assert.Equal(t, pgUUID(ownerID), store.reassignCalls[0].OwnerID)
	assert.Equal(t, []sqlc.FaceReassignment{
		{AssetID: pgUUID(assetOneID), SourcePersonID: pgUUID(sourceOneID)},
		{AssetID: pgUUID(assetTwoID), SourcePersonID: pgUUID(sourceTwoID)},
	}, store.reassignCalls[0].FaceUpdates)
}

func TestReassignFacesRejectsCrossUserPeopleBeforeMutation(t *testing.T) {
	ownerID := uuid.New()
	otherOwnerID := uuid.New()
	ownedTargetID := uuid.New()
	foreignTargetID := uuid.New()
	ownedSourceID := uuid.New()
	foreignSourceID := uuid.New()
	assetID := uuid.New()
	people := map[uuid.UUID]sqlc.Person{
		ownedTargetID:   testPerson(ownedTargetID, ownerID, "owned target"),
		foreignTargetID: testPerson(foreignTargetID, otherOwnerID, "foreign target"),
		ownedSourceID:   testPerson(ownedSourceID, ownerID, "owned source"),
		foreignSourceID: testPerson(foreignSourceID, otherOwnerID, "foreign source"),
	}

	tests := []struct {
		name     string
		targetID uuid.UUID
		sourceID uuid.UUID
	}{
		{name: "foreign target", targetID: foreignTargetID, sourceID: ownedSourceID},
		{name: "foreign source", targetID: ownedTargetID, sourceID: foreignSourceID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakePeopleQueries{people: people}
			server := &Server{queries: store}
			response, err := server.ReassignFaces(userContext(ownerID), &immichv1.ReassignFacesRequest{
				Id: tt.targetID.String(),
				FaceUpdates: []*immichv1.FaceUpdate{
					{AssetId: assetID.String(), PersonId: tt.sourceID.String()},
				},
			})
			assert.Nil(t, response)
			assert.Equal(t, codes.PermissionDenied, status.Code(err))
			assert.Empty(t, store.reassignCalls)
		})
	}
}

func TestReassignFacesRejectsForeignAssetFromTransaction(t *testing.T) {
	ownerID := uuid.New()
	targetID := uuid.New()
	sourceID := uuid.New()
	store := &fakePeopleQueries{
		people: map[uuid.UUID]sqlc.Person{
			targetID: testPerson(targetID, ownerID, "target"),
			sourceID: testPerson(sourceID, ownerID, "source"),
		},
		reassignErr: sqlc.ErrFaceReassignmentAccessDenied,
	}
	server := &Server{queries: store}

	response, err := server.ReassignFaces(userContext(ownerID), &immichv1.ReassignFacesRequest{
		Id: targetID.String(),
		FaceUpdates: []*immichv1.FaceUpdate{
			{AssetId: uuid.NewString(), PersonId: sourceID.String()},
		},
	})
	assert.Nil(t, response)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Len(t, store.reassignCalls, 1)
}

func TestReassignFacesValidatesCompleteRequestBeforeMutation(t *testing.T) {
	ownerID := uuid.New()
	targetID := uuid.New()
	sourceID := uuid.New()
	missingSourceID := uuid.New()
	assetID := uuid.New()
	people := map[uuid.UUID]sqlc.Person{
		targetID: testPerson(targetID, ownerID, "target"),
		sourceID: testPerson(sourceID, ownerID, "source"),
	}

	tests := []struct {
		name     string
		targetID string
		updates  []*immichv1.FaceUpdate
		wantCode codes.Code
	}{
		{name: "invalid target", targetID: "bad-target", wantCode: codes.InvalidArgument},
		{name: "invalid asset", targetID: targetID.String(), updates: []*immichv1.FaceUpdate{{AssetId: "bad-asset", PersonId: sourceID.String()}}, wantCode: codes.InvalidArgument},
		{name: "invalid source", targetID: targetID.String(), updates: []*immichv1.FaceUpdate{{AssetId: assetID.String(), PersonId: "bad-source"}}, wantCode: codes.InvalidArgument},
		{name: "missing source", targetID: targetID.String(), updates: []*immichv1.FaceUpdate{{AssetId: assetID.String(), PersonId: missingSourceID.String()}}, wantCode: codes.NotFound},
		{name: "nil update", targetID: targetID.String(), updates: []*immichv1.FaceUpdate{nil}, wantCode: codes.InvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakePeopleQueries{people: people}
			server := &Server{queries: store}
			response, err := server.ReassignFaces(userContext(ownerID), &immichv1.ReassignFacesRequest{
				Id:          tt.targetID,
				FaceUpdates: tt.updates,
			})
			assert.Nil(t, response)
			assert.Equal(t, tt.wantCode, status.Code(err))
			assert.Empty(t, store.reassignCalls)
		})
	}
}

func TestReassignFacesPropagatesTransactionAndCountFailures(t *testing.T) {
	ownerID := uuid.New()
	targetID := uuid.New()
	sourceID := uuid.New()
	assetID := uuid.New()
	target := testPerson(targetID, ownerID, "target")
	people := map[uuid.UUID]sqlc.Person{
		targetID: target,
		sourceID: testPerson(sourceID, ownerID, "source"),
	}

	tests := []struct {
		name         string
		reassignErr  error
		faceCountErr error
		wantCode     codes.Code
	}{
		{name: "face missing", reassignErr: sqlc.ErrFaceReassignmentNotFound, wantCode: codes.NotFound},
		{name: "concurrent update", reassignErr: sqlc.ErrFaceReassignmentConflict, wantCode: codes.Aborted},
		{name: "participant changed", reassignErr: pgx.ErrNoRows, wantCode: codes.Aborted},
		{name: "database write", reassignErr: errors.New("database unavailable"), wantCode: codes.Internal},
		{name: "count", faceCountErr: errors.New("count unavailable"), wantCode: codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakePeopleQueries{
				people:         people,
				faceCountErrs:  map[uuid.UUID]error{targetID: tt.faceCountErr},
				reassignResult: target,
				reassignErr:    tt.reassignErr,
			}
			server := &Server{queries: store}
			response, err := server.ReassignFaces(userContext(ownerID), &immichv1.ReassignFacesRequest{
				Id: targetID.String(),
				FaceUpdates: []*immichv1.FaceUpdate{
					{AssetId: assetID.String(), PersonId: sourceID.String()},
				},
			})
			assert.Nil(t, response)
			assert.Equal(t, tt.wantCode, status.Code(err))
			require.Len(t, store.reassignCalls, 1)
		})
	}
}

func TestReassignFacesEmptyRequestReturnsNoPeople(t *testing.T) {
	ownerID := uuid.New()
	targetID := uuid.New()
	store := &fakePeopleQueries{people: map[uuid.UUID]sqlc.Person{
		targetID: testPerson(targetID, ownerID, "target"),
	}}
	server := &Server{queries: store}

	response, err := server.ReassignFaces(userContext(ownerID), &immichv1.ReassignFacesRequest{Id: targetID.String()})
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Empty(t, response.GetPeople())
	assert.Empty(t, store.reassignCalls)
}
