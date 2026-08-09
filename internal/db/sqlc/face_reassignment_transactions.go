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

var (
	// ErrFaceReassignmentNotFound means no live face matched one of the
	// requested asset/person pairs.
	ErrFaceReassignmentNotFound = errors.New("face for reassignment not found")
	// ErrFaceReassignmentAccessDenied means a matching face belongs to an
	// asset owned by another tenant.
	ErrFaceReassignmentAccessDenied = errors.New("face reassignment access denied")
	// ErrFaceReassignmentConflict means the validated face set changed before
	// it could be updated.
	ErrFaceReassignmentConflict = errors.New("face reassignment conflict")
)

// FaceReassignment identifies the current person and asset for one or more
// faces that should move to the target person.
type FaceReassignment struct {
	AssetID        pgtype.UUID
	SourcePersonID pgtype.UUID
}

// ReassignFacesParams describes one tenant-scoped bulk face reassignment.
type ReassignFacesParams struct {
	TargetPersonID pgtype.UUID
	OwnerID        pgtype.UUID
	FaceUpdates    []FaceReassignment
}

type faceReassignmentQueries interface {
	GetFacesForReassignment(context.Context, GetFacesForReassignmentParams) ([]GetFacesForReassignmentRow, error)
	GetOwnedPersonForMerge(context.Context, GetOwnedPersonForMergeParams) (Person, error)
	ReassignFacesByIDs(context.Context, ReassignFacesByIDsParams) (int64, error)
	RepairFeatureFacesAfterReassignment(context.Context, []pgtype.UUID) error
}

// ReassignFaces validates, authorizes, and moves the complete face set in one
// transaction. No face changes until every request item has been checked.
func (q *Queries) ReassignFaces(ctx context.Context, params ReassignFacesParams) (Person, error) {
	if !params.TargetPersonID.Valid || !params.OwnerID.Valid {
		return Person{}, errors.New("reassign faces requires valid target and owner IDs")
	}

	updates := make([]FaceReassignment, 0, len(params.FaceUpdates))
	seenUpdates := make(map[[32]byte]struct{}, len(params.FaceUpdates))
	for _, update := range params.FaceUpdates {
		if !update.AssetID.Valid || !update.SourcePersonID.Valid {
			return Person{}, errors.New("reassign faces requires valid asset and source person IDs")
		}
		var key [32]byte
		copy(key[:16], update.AssetID.Bytes[:])
		copy(key[16:], update.SourcePersonID.Bytes[:])
		if _, duplicate := seenUpdates[key]; duplicate {
			continue
		}
		seenUpdates[key] = struct{}{}
		updates = append(updates, update)
	}

	// A deterministic order keeps concurrent requests involving the same
	// people/faces from acquiring locks in opposite orders.
	sort.Slice(updates, func(i, j int) bool {
		if comparison := bytes.Compare(updates[i].AssetID.Bytes[:], updates[j].AssetID.Bytes[:]); comparison != 0 {
			return comparison < 0
		}
		return bytes.Compare(updates[i].SourcePersonID.Bytes[:], updates[j].SourcePersonID.Bytes[:]) < 0
	})

	beginner, ok := q.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return Person{}, errors.New("reassign faces requires a transaction-capable database")
	}

	var target Person
	err := pgx.BeginFunc(ctx, beginner, func(tx pgx.Tx) error {
		var err error
		target, err = reassignFacesInTransaction(ctx, q.WithTx(tx), ReassignFacesParams{
			TargetPersonID: params.TargetPersonID,
			OwnerID:        params.OwnerID,
			FaceUpdates:    updates,
		})
		return err
	})
	if err != nil {
		return Person{}, err
	}
	return target, nil
}

func reassignFacesInTransaction(ctx context.Context, queries faceReassignmentQueries, params ReassignFacesParams) (Person, error) {
	participantIDs := []pgtype.UUID{params.TargetPersonID}
	seenPeople := map[[16]byte]struct{}{params.TargetPersonID.Bytes: {}}
	for _, update := range params.FaceUpdates {
		if _, duplicate := seenPeople[update.SourcePersonID.Bytes]; duplicate {
			continue
		}
		seenPeople[update.SourcePersonID.Bytes] = struct{}{}
		participantIDs = append(participantIDs, update.SourcePersonID)
	}
	sort.Slice(participantIDs, func(i, j int) bool {
		return bytes.Compare(participantIDs[i].Bytes[:], participantIDs[j].Bytes[:]) < 0
	})

	var target Person
	for _, personID := range participantIDs {
		person, err := queries.GetOwnedPersonForMerge(ctx, GetOwnedPersonForMergeParams{
			PersonID: personID,
			OwnerID:  params.OwnerID,
		})
		if err != nil {
			return Person{}, fmt.Errorf("lock face reassignment participant: %w", err)
		}
		if personID.Bytes == params.TargetPersonID.Bytes {
			target = person
		}
	}

	faceIDs := make([]pgtype.UUID, 0, len(params.FaceUpdates))
	seenFaces := make(map[[16]byte]struct{}, len(params.FaceUpdates))
	for _, update := range params.FaceUpdates {
		faces, err := queries.GetFacesForReassignment(ctx, GetFacesForReassignmentParams(update))
		if err != nil {
			return Person{}, fmt.Errorf("lock faces for reassignment: %w", err)
		}
		if len(faces) == 0 {
			return Person{}, ErrFaceReassignmentNotFound
		}
		for _, face := range faces {
			if face.AssetOwnerID != params.OwnerID {
				return Person{}, ErrFaceReassignmentAccessDenied
			}
			if _, duplicate := seenFaces[face.ID.Bytes]; duplicate {
				continue
			}
			seenFaces[face.ID.Bytes] = struct{}{}
			faceIDs = append(faceIDs, face.ID)
		}
	}

	if len(faceIDs) == 0 {
		return target, nil
	}
	updated, err := queries.ReassignFacesByIDs(ctx, ReassignFacesByIDsParams{
		TargetPersonID: params.TargetPersonID,
		FaceIds:        faceIDs,
	})
	if err != nil {
		return Person{}, fmt.Errorf("update faces for reassignment: %w", err)
	}
	if updated != int64(len(faceIDs)) {
		return Person{}, fmt.Errorf("%w: updated %d of %d faces", ErrFaceReassignmentConflict, updated, len(faceIDs))
	}
	if err := queries.RepairFeatureFacesAfterReassignment(ctx, participantIDs); err != nil {
		return Person{}, fmt.Errorf("repair feature faces after reassignment: %w", err)
	}

	return target, nil
}
