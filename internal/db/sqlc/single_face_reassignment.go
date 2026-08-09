package sqlc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ReassignSingleFaceParams describes one tenant-scoped face move.
type ReassignSingleFaceParams struct {
	FaceID         pgtype.UUID
	TargetPersonID pgtype.UUID
	OwnerID        pgtype.UUID
}

type singleFaceReassignmentQueries interface {
	GetAssetFaceByID(context.Context, pgtype.UUID) (AssetFace, error)
	GetFaceForReassignmentByID(context.Context, pgtype.UUID) (GetFaceForReassignmentByIDRow, error)
	GetOwnedPersonForMerge(context.Context, GetOwnedPersonForMergeParams) (Person, error)
	ReassignFacesByIDs(context.Context, ReassignFacesByIDsParams) (int64, error)
	RepairFeatureFacesAfterReassignment(context.Context, []pgtype.UUID) error
}

// ReassignSingleFace authorizes and moves exactly one face, then repairs the
// source and target feature-face pointers before committing.
func (q *Queries) ReassignSingleFace(ctx context.Context, params ReassignSingleFaceParams) (AssetFace, error) {
	if !params.FaceID.Valid || !params.TargetPersonID.Valid || !params.OwnerID.Valid {
		return AssetFace{}, errors.New("single face reassignment requires valid face, target, and owner IDs")
	}

	beginner, ok := q.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return AssetFace{}, errors.New("single face reassignment requires a transaction-capable database")
	}

	var updated AssetFace
	err := pgx.BeginFunc(ctx, beginner, func(tx pgx.Tx) error {
		var err error
		updated, err = reassignSingleFaceInTransaction(ctx, q.WithTx(tx), params)
		return err
	})
	if err != nil {
		return AssetFace{}, err
	}
	return updated, nil
}

func reassignSingleFaceInTransaction(ctx context.Context, queries singleFaceReassignmentQueries, params ReassignSingleFaceParams) (AssetFace, error) {
	// Read the source before taking locks so people are locked before faces,
	// matching the bulk-reassignment lock order. The locked face is compared
	// below to reject a concurrent reassignment.
	initialFace, err := queries.GetAssetFaceByID(ctx, params.FaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return AssetFace{}, ErrFaceReassignmentNotFound
	}
	if err != nil {
		return AssetFace{}, fmt.Errorf("read face for reassignment: %w", err)
	}

	participantIDs := []pgtype.UUID{params.TargetPersonID}
	if initialFace.PersonId.Valid && initialFace.PersonId != params.TargetPersonID {
		participantIDs = append(participantIDs, initialFace.PersonId)
	}
	sort.Slice(participantIDs, func(i, j int) bool {
		return bytes.Compare(participantIDs[i].Bytes[:], participantIDs[j].Bytes[:]) < 0
	})
	for _, personID := range participantIDs {
		_, err := queries.GetOwnedPersonForMerge(ctx, GetOwnedPersonForMergeParams{
			PersonID: personID,
			OwnerID:  params.OwnerID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return AssetFace{}, ErrFaceReassignmentAccessDenied
		}
		if err != nil {
			return AssetFace{}, fmt.Errorf("lock single face reassignment participant: %w", err)
		}
	}

	lockedFace, err := queries.GetFaceForReassignmentByID(ctx, params.FaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return AssetFace{}, ErrFaceReassignmentNotFound
	}
	if err != nil {
		return AssetFace{}, fmt.Errorf("lock face for reassignment: %w", err)
	}
	if lockedFace.AssetOwnerID != params.OwnerID {
		return AssetFace{}, ErrFaceReassignmentAccessDenied
	}
	if lockedFace.PersonId != initialFace.PersonId {
		return AssetFace{}, ErrFaceReassignmentConflict
	}

	updatedCount, err := queries.ReassignFacesByIDs(ctx, ReassignFacesByIDsParams{
		TargetPersonID: params.TargetPersonID,
		FaceIds:        []pgtype.UUID{params.FaceID},
	})
	if err != nil {
		return AssetFace{}, fmt.Errorf("update single face reassignment: %w", err)
	}
	if updatedCount != 1 {
		return AssetFace{}, fmt.Errorf("%w: updated %d of 1 face", ErrFaceReassignmentConflict, updatedCount)
	}
	if err := queries.RepairFeatureFacesAfterReassignment(ctx, participantIDs); err != nil {
		return AssetFace{}, fmt.Errorf("repair feature faces after single reassignment: %w", err)
	}

	updated, err := queries.GetAssetFaceByID(ctx, params.FaceID)
	if err != nil {
		return AssetFace{}, fmt.Errorf("read reassigned face: %w", err)
	}
	return updated, nil
}
