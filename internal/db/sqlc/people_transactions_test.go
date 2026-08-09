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

type fakeMergePeopleQueries struct {
	people        map[[16]byte]Person
	getErrors     map[[16]byte]error
	reassignErr   error
	deleteErr     error
	deleteCount   int64
	reassignCalls []ReassignPersonFacesParams
	deleteCalls   []DeletePeopleForMergeParams
}

func (f *fakeMergePeopleQueries) GetOwnedPersonForMerge(_ context.Context, params GetOwnedPersonForMergeParams) (Person, error) {
	if err := f.getErrors[params.PersonID.Bytes]; err != nil {
		return Person{}, err
	}
	person, ok := f.people[params.PersonID.Bytes]
	if !ok || person.OwnerId != params.OwnerID {
		return Person{}, pgx.ErrNoRows
	}
	return person, nil
}

func (f *fakeMergePeopleQueries) ReassignPersonFaces(_ context.Context, params ReassignPersonFacesParams) error {
	f.reassignCalls = append(f.reassignCalls, params)
	return f.reassignErr
}

func (f *fakeMergePeopleQueries) DeletePeopleForMerge(_ context.Context, params DeletePeopleForMergeParams) (int64, error) {
	f.deleteCalls = append(f.deleteCalls, params)
	return f.deleteCount, f.deleteErr
}

func mergeTestUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func TestMergePeopleInTransactionHandlesEveryWriteFailure(t *testing.T) {
	ownerID := mergeTestUUID(uuid.New())
	targetID := mergeTestUUID(uuid.New())
	sourceID := mergeTestUUID(uuid.New())
	people := map[[16]byte]Person{
		targetID.Bytes: {ID: targetID, OwnerId: ownerID},
		sourceID.Bytes: {ID: sourceID, OwnerId: ownerID},
	}
	reassignFailure := errors.New("reassign failed")
	deleteFailure := errors.New("delete failed")

	tests := []struct {
		name            string
		store           *fakeMergePeopleQueries
		wantError       error
		wantDeleteCalls int
	}{
		{
			name: "reassign failure stops before delete",
			store: &fakeMergePeopleQueries{
				people:      people,
				reassignErr: reassignFailure,
			},
			wantError:       reassignFailure,
			wantDeleteCalls: 0,
		},
		{
			name: "delete failure is returned",
			store: &fakeMergePeopleQueries{
				people:      people,
				deleteErr:   deleteFailure,
				deleteCount: 1,
			},
			wantError:       deleteFailure,
			wantDeleteCalls: 1,
		},
		{
			name: "short delete is returned",
			store: &fakeMergePeopleQueries{
				people:      people,
				deleteCount: 0,
			},
			wantDeleteCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mergePeopleInTransaction(context.Background(), tt.store, MergePeopleParams{
				TargetID:  targetID,
				OwnerID:   ownerID,
				SourceIds: []pgtype.UUID{sourceID},
			})
			require.Error(t, err)
			if tt.wantError != nil {
				assert.ErrorIs(t, err, tt.wantError)
			}
			assert.Len(t, tt.store.deleteCalls, tt.wantDeleteCalls)
		})
	}
}

func TestMergePeopleInTransactionRejectsParticipantLostAfterValidation(t *testing.T) {
	ownerID := mergeTestUUID(uuid.New())
	targetID := mergeTestUUID(uuid.New())
	sourceID := mergeTestUUID(uuid.New())
	store := &fakeMergePeopleQueries{
		people: map[[16]byte]Person{
			targetID.Bytes: {ID: targetID, OwnerId: ownerID},
		},
	}

	_, err := mergePeopleInTransaction(context.Background(), store, MergePeopleParams{
		TargetID:  targetID,
		OwnerID:   ownerID,
		SourceIds: []pgtype.UUID{sourceID},
	})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.Empty(t, store.reassignCalls)
	assert.Empty(t, store.deleteCalls)
}

func TestMergePeopleInTransactionMovesFacesBeforeDeletingSources(t *testing.T) {
	ownerID := mergeTestUUID(uuid.New())
	targetID := mergeTestUUID(uuid.New())
	sourceID := mergeTestUUID(uuid.New())
	target := Person{ID: targetID, OwnerId: ownerID, Name: "target"}
	store := &fakeMergePeopleQueries{
		people: map[[16]byte]Person{
			targetID.Bytes: target,
			sourceID.Bytes: {ID: sourceID, OwnerId: ownerID},
		},
		deleteCount: 1,
	}

	got, err := mergePeopleInTransaction(context.Background(), store, MergePeopleParams{
		TargetID:  targetID,
		OwnerID:   ownerID,
		SourceIds: []pgtype.UUID{sourceID},
	})
	require.NoError(t, err)
	assert.Equal(t, target, got)
	require.Len(t, store.reassignCalls, 1)
	require.Len(t, store.deleteCalls, 1)
	assert.Equal(t, targetID, store.reassignCalls[0].TargetID)
	assert.Equal(t, []pgtype.UUID{sourceID}, store.reassignCalls[0].SourceIds)
}
