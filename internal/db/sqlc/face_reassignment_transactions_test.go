package sqlc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeFaceReassignmentQueries struct {
	people      map[[16]byte]Person
	faces       map[[32]byte][]GetFacesForReassignmentRow
	personErr   error
	facesErr    error
	updateErr   error
	repairErr   error
	updateCount int64
	getCalls    []GetFacesForReassignmentParams
	updateCalls []ReassignFacesByIDsParams
	repairCalls [][]pgtype.UUID
}

func (f *fakeFaceReassignmentQueries) GetOwnedPersonForMerge(_ context.Context, params GetOwnedPersonForMergeParams) (Person, error) {
	if f.personErr != nil {
		return Person{}, f.personErr
	}
	person, ok := f.people[params.PersonID.Bytes]
	if !ok || person.OwnerId != params.OwnerID {
		return Person{}, pgx.ErrNoRows
	}
	return person, nil
}

func (f *fakeFaceReassignmentQueries) GetFacesForReassignment(_ context.Context, params GetFacesForReassignmentParams) ([]GetFacesForReassignmentRow, error) {
	f.getCalls = append(f.getCalls, params)
	if f.facesErr != nil {
		return nil, f.facesErr
	}
	return f.faces[faceReassignmentKey(params.AssetID, params.SourcePersonID)], nil
}

func (f *fakeFaceReassignmentQueries) ReassignFacesByIDs(_ context.Context, params ReassignFacesByIDsParams) (int64, error) {
	f.updateCalls = append(f.updateCalls, params)
	return f.updateCount, f.updateErr
}

func (f *fakeFaceReassignmentQueries) RepairFeatureFacesAfterReassignment(_ context.Context, personIDs []pgtype.UUID) error {
	f.repairCalls = append(f.repairCalls, personIDs)
	return f.repairErr
}

func faceReassignmentUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func faceReassignmentKey(assetID, personID pgtype.UUID) [32]byte {
	var key [32]byte
	copy(key[:16], assetID.Bytes[:])
	copy(key[16:], personID.Bytes[:])
	return key
}

func TestReassignFacesInTransactionAuthorizesBeforeBatchUpdate(t *testing.T) {
	ownerID := faceReassignmentUUID(uuid.New())
	targetID := faceReassignmentUUID(uuid.New())
	sourceOneID := faceReassignmentUUID(uuid.New())
	sourceTwoID := faceReassignmentUUID(uuid.New())
	assetOneID := faceReassignmentUUID(uuid.New())
	assetTwoID := faceReassignmentUUID(uuid.New())
	faceOneID := faceReassignmentUUID(uuid.New())
	faceTwoID := faceReassignmentUUID(uuid.New())
	faceThreeID := faceReassignmentUUID(uuid.New())
	target := Person{ID: targetID, OwnerId: ownerID, Name: "target"}
	store := &fakeFaceReassignmentQueries{
		people: map[[16]byte]Person{
			targetID.Bytes:    target,
			sourceOneID.Bytes: {ID: sourceOneID, OwnerId: ownerID},
			sourceTwoID.Bytes: {ID: sourceTwoID, OwnerId: ownerID},
		},
		faces: map[[32]byte][]GetFacesForReassignmentRow{
			faceReassignmentKey(assetOneID, sourceOneID): {
				{ID: faceOneID, AssetOwnerID: ownerID},
				{ID: faceTwoID, AssetOwnerID: ownerID},
			},
			faceReassignmentKey(assetTwoID, sourceTwoID): {
				{ID: faceThreeID, AssetOwnerID: ownerID},
			},
		},
		updateCount: 3,
	}

	got, err := reassignFacesInTransaction(context.Background(), store, ReassignFacesParams{
		TargetPersonID: targetID,
		OwnerID:        ownerID,
		FaceUpdates: []FaceReassignment{
			{AssetID: assetOneID, SourcePersonID: sourceOneID},
			{AssetID: assetTwoID, SourcePersonID: sourceTwoID},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, target, got)
	require.Len(t, store.getCalls, 2)
	require.Len(t, store.updateCalls, 1)
	assert.Equal(t, targetID, store.updateCalls[0].TargetPersonID)
	assert.ElementsMatch(t, []pgtype.UUID{faceOneID, faceTwoID, faceThreeID}, store.updateCalls[0].FaceIds)
	require.Len(t, store.repairCalls, 1)
	assert.ElementsMatch(t, []pgtype.UUID{targetID, sourceOneID, sourceTwoID}, store.repairCalls[0])
}

func TestReassignFacesInTransactionRejectsUnauthorizedOrMissingFaceBeforeWrite(t *testing.T) {
	ownerID := faceReassignmentUUID(uuid.New())
	foreignOwnerID := faceReassignmentUUID(uuid.New())
	targetID := faceReassignmentUUID(uuid.New())
	sourceID := faceReassignmentUUID(uuid.New())
	assetID := faceReassignmentUUID(uuid.New())
	faceID := faceReassignmentUUID(uuid.New())
	people := map[[16]byte]Person{
		targetID.Bytes: {ID: targetID, OwnerId: ownerID},
		sourceID.Bytes: {ID: sourceID, OwnerId: ownerID},
	}

	tests := []struct {
		name    string
		faces   []GetFacesForReassignmentRow
		wantErr error
	}{
		{name: "missing", wantErr: ErrFaceReassignmentNotFound},
		{name: "foreign asset", faces: []GetFacesForReassignmentRow{{ID: faceID, AssetOwnerID: foreignOwnerID}}, wantErr: ErrFaceReassignmentAccessDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeFaceReassignmentQueries{
				people: people,
				faces: map[[32]byte][]GetFacesForReassignmentRow{
					faceReassignmentKey(assetID, sourceID): tt.faces,
				},
			}
			_, err := reassignFacesInTransaction(context.Background(), store, ReassignFacesParams{
				TargetPersonID: targetID,
				OwnerID:        ownerID,
				FaceUpdates: []FaceReassignment{
					{AssetID: assetID, SourcePersonID: sourceID},
				},
			})
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Empty(t, store.updateCalls)
			assert.Empty(t, store.repairCalls)
		})
	}
}

func TestReassignFacesInTransactionPropagatesEveryDatabaseFailure(t *testing.T) {
	ownerID := faceReassignmentUUID(uuid.New())
	targetID := faceReassignmentUUID(uuid.New())
	sourceID := faceReassignmentUUID(uuid.New())
	assetID := faceReassignmentUUID(uuid.New())
	faceID := faceReassignmentUUID(uuid.New())
	people := map[[16]byte]Person{
		targetID.Bytes: {ID: targetID, OwnerId: ownerID},
		sourceID.Bytes: {ID: sourceID, OwnerId: ownerID},
	}
	databaseErr := errors.New("database unavailable")

	tests := []struct {
		name        string
		personErr   error
		facesErr    error
		updateErr   error
		repairErr   error
		updateCount int64
		wantErr     error
	}{
		{name: "person lock", personErr: databaseErr, wantErr: databaseErr},
		{name: "face lock", facesErr: databaseErr, wantErr: databaseErr},
		{name: "batch update", updateErr: databaseErr, updateCount: 1, wantErr: databaseErr},
		{name: "short update", updateCount: 0, wantErr: ErrFaceReassignmentConflict},
		{name: "feature repair", repairErr: databaseErr, updateCount: 1, wantErr: databaseErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeFaceReassignmentQueries{
				people: people,
				faces: map[[32]byte][]GetFacesForReassignmentRow{
					faceReassignmentKey(assetID, sourceID): {{ID: faceID, AssetOwnerID: ownerID}},
				},
				personErr:   tt.personErr,
				facesErr:    tt.facesErr,
				updateErr:   tt.updateErr,
				repairErr:   tt.repairErr,
				updateCount: tt.updateCount,
			}
			_, err := reassignFacesInTransaction(context.Background(), store, ReassignFacesParams{
				TargetPersonID: targetID,
				OwnerID:        ownerID,
				FaceUpdates: []FaceReassignment{
					{AssetID: assetID, SourcePersonID: sourceID},
				},
			})
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
