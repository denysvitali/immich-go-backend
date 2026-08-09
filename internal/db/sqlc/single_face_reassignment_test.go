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

type fakeSingleFaceReassignmentQueries struct {
	initialFace AssetFace
	updatedFace AssetFace
	lockedFace  GetFaceForReassignmentByIDRow
	people      map[[16]byte]Person
	initialErr  error
	lockedErr   error
	personErr   error
	updateErr   error
	repairErr   error
	updateCount int64
	getCalls    int
	personCalls []GetOwnedPersonForMergeParams
	updateCalls []ReassignFacesByIDsParams
	repairCalls [][]pgtype.UUID
}

func (f *fakeSingleFaceReassignmentQueries) GetAssetFaceByID(_ context.Context, _ pgtype.UUID) (AssetFace, error) {
	f.getCalls++
	if f.getCalls == 1 {
		return f.initialFace, f.initialErr
	}
	return f.updatedFace, nil
}

func (f *fakeSingleFaceReassignmentQueries) GetFaceForReassignmentByID(_ context.Context, _ pgtype.UUID) (GetFaceForReassignmentByIDRow, error) {
	return f.lockedFace, f.lockedErr
}

func (f *fakeSingleFaceReassignmentQueries) GetOwnedPersonForMerge(_ context.Context, params GetOwnedPersonForMergeParams) (Person, error) {
	f.personCalls = append(f.personCalls, params)
	if f.personErr != nil {
		return Person{}, f.personErr
	}
	person, ok := f.people[params.PersonID.Bytes]
	if !ok || person.OwnerId != params.OwnerID {
		return Person{}, pgx.ErrNoRows
	}
	return person, nil
}

func (f *fakeSingleFaceReassignmentQueries) ReassignFacesByIDs(_ context.Context, params ReassignFacesByIDsParams) (int64, error) {
	f.updateCalls = append(f.updateCalls, params)
	return f.updateCount, f.updateErr
}

func (f *fakeSingleFaceReassignmentQueries) RepairFeatureFacesAfterReassignment(_ context.Context, personIDs []pgtype.UUID) error {
	f.repairCalls = append(f.repairCalls, personIDs)
	return f.repairErr
}

func singleFaceUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func singleFaceLockedRow(face AssetFace, ownerID pgtype.UUID) GetFaceForReassignmentByIDRow {
	return GetFaceForReassignmentByIDRow{
		AssetId:       face.AssetId,
		PersonId:      face.PersonId,
		ImageWidth:    face.ImageWidth,
		ImageHeight:   face.ImageHeight,
		BoundingBoxX1: face.BoundingBoxX1,
		BoundingBoxY1: face.BoundingBoxY1,
		BoundingBoxX2: face.BoundingBoxX2,
		BoundingBoxY2: face.BoundingBoxY2,
		ID:            face.ID,
		SourceType:    face.SourceType,
		DeletedAt:     face.DeletedAt,
		AssetOwnerID:  ownerID,
	}
}

func TestReassignSingleFaceInTransactionRepairsSourceAndTarget(t *testing.T) {
	ownerID := singleFaceUUID(uuid.New())
	targetID := singleFaceUUID(uuid.New())
	sourceID := singleFaceUUID(uuid.New())
	faceID := singleFaceUUID(uuid.New())
	assetID := singleFaceUUID(uuid.New())
	initial := AssetFace{ID: faceID, AssetId: assetID, PersonId: sourceID}
	updated := initial
	updated.PersonId = targetID
	store := &fakeSingleFaceReassignmentQueries{
		initialFace: initial,
		updatedFace: updated,
		lockedFace:  singleFaceLockedRow(initial, ownerID),
		people: map[[16]byte]Person{
			targetID.Bytes: {ID: targetID, OwnerId: ownerID},
			sourceID.Bytes: {ID: sourceID, OwnerId: ownerID},
		},
		updateCount: 1,
	}

	got, err := reassignSingleFaceInTransaction(context.Background(), store, ReassignSingleFaceParams{
		FaceID:         faceID,
		TargetPersonID: targetID,
		OwnerID:        ownerID,
	})
	require.NoError(t, err)
	assert.Equal(t, updated, got)
	require.Len(t, store.personCalls, 2)
	assert.ElementsMatch(t, []pgtype.UUID{sourceID, targetID}, []pgtype.UUID{
		store.personCalls[0].PersonID,
		store.personCalls[1].PersonID,
	})
	require.Len(t, store.updateCalls, 1)
	assert.Equal(t, targetID, store.updateCalls[0].TargetPersonID)
	assert.Equal(t, []pgtype.UUID{faceID}, store.updateCalls[0].FaceIds)
	require.Len(t, store.repairCalls, 1)
	assert.ElementsMatch(t, []pgtype.UUID{sourceID, targetID}, store.repairCalls[0])
}

func TestReassignSingleFaceInTransactionPreservesTenantAuthorization(t *testing.T) {
	ownerID := singleFaceUUID(uuid.New())
	foreignOwnerID := singleFaceUUID(uuid.New())
	targetID := singleFaceUUID(uuid.New())
	sourceID := singleFaceUUID(uuid.New())
	changedSourceID := singleFaceUUID(uuid.New())
	faceID := singleFaceUUID(uuid.New())
	initial := AssetFace{ID: faceID, AssetId: singleFaceUUID(uuid.New()), PersonId: sourceID}
	ownedPeople := map[[16]byte]Person{
		targetID.Bytes: {ID: targetID, OwnerId: ownerID},
		sourceID.Bytes: {ID: sourceID, OwnerId: ownerID},
	}

	tests := []struct {
		name       string
		people     map[[16]byte]Person
		lockedFace GetFaceForReassignmentByIDRow
		wantErr    error
	}{
		{name: "foreign target", people: map[[16]byte]Person{sourceID.Bytes: ownedPeople[sourceID.Bytes]}, lockedFace: singleFaceLockedRow(initial, ownerID), wantErr: ErrFaceReassignmentAccessDenied},
		{name: "foreign source", people: map[[16]byte]Person{targetID.Bytes: ownedPeople[targetID.Bytes]}, lockedFace: singleFaceLockedRow(initial, ownerID), wantErr: ErrFaceReassignmentAccessDenied},
		{name: "foreign asset", people: ownedPeople, lockedFace: singleFaceLockedRow(initial, foreignOwnerID), wantErr: ErrFaceReassignmentAccessDenied},
		{name: "concurrent source change", people: ownedPeople, lockedFace: func() GetFaceForReassignmentByIDRow {
			row := singleFaceLockedRow(initial, ownerID)
			row.PersonId = changedSourceID
			return row
		}(), wantErr: ErrFaceReassignmentConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeSingleFaceReassignmentQueries{
				initialFace: initial,
				lockedFace:  tt.lockedFace,
				people:      tt.people,
				updateCount: 1,
			}
			_, err := reassignSingleFaceInTransaction(context.Background(), store, ReassignSingleFaceParams{
				FaceID:         faceID,
				TargetPersonID: targetID,
				OwnerID:        ownerID,
			})
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Empty(t, store.updateCalls)
			assert.Empty(t, store.repairCalls)
		})
	}
}

func TestReassignSingleFaceInTransactionRollsBackOnRepairFailure(t *testing.T) {
	ownerID := singleFaceUUID(uuid.New())
	targetID := singleFaceUUID(uuid.New())
	sourceID := singleFaceUUID(uuid.New())
	faceID := singleFaceUUID(uuid.New())
	initial := AssetFace{ID: faceID, AssetId: singleFaceUUID(uuid.New()), PersonId: sourceID}
	repairErr := errors.New("repair failed")
	store := &fakeSingleFaceReassignmentQueries{
		initialFace: initial,
		lockedFace:  singleFaceLockedRow(initial, ownerID),
		people: map[[16]byte]Person{
			targetID.Bytes: {ID: targetID, OwnerId: ownerID},
			sourceID.Bytes: {ID: sourceID, OwnerId: ownerID},
		},
		updateCount: 1,
		repairErr:   repairErr,
	}

	_, err := reassignSingleFaceInTransaction(context.Background(), store, ReassignSingleFaceParams{
		FaceID:         faceID,
		TargetPersonID: targetID,
		OwnerID:        ownerID,
	})
	assert.ErrorIs(t, err, repairErr)
	require.Len(t, store.updateCalls, 1)
	require.Len(t, store.repairCalls, 1)
}
