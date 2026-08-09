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

// MergePeopleParams identifies one owned target and the owned source people
// whose faces should move to it.
type MergePeopleParams struct {
	TargetID  pgtype.UUID
	OwnerID   pgtype.UUID
	SourceIds []pgtype.UUID
}

type mergePeopleQueries interface {
	DeletePeopleForMerge(context.Context, DeletePeopleForMergeParams) (int64, error)
	GetOwnedPersonForMerge(context.Context, GetOwnedPersonForMergeParams) (Person, error)
	ReassignPersonFaces(context.Context, ReassignPersonFacesParams) error
}

// MergePeople reassigns all source faces and deletes the source people in one
// transaction. Locking every participant before the first write makes the
// server's ownership validation race-safe as well as atomic.
func (q *Queries) MergePeople(ctx context.Context, params MergePeopleParams) (Person, error) {
	if !params.TargetID.Valid || !params.OwnerID.Valid {
		return Person{}, errors.New("merge people requires valid target and owner IDs")
	}

	sourceIDs := make([]pgtype.UUID, 0, len(params.SourceIds))
	seen := make(map[[16]byte]struct{}, len(params.SourceIds))
	for _, sourceID := range params.SourceIds {
		if !sourceID.Valid {
			return Person{}, errors.New("merge people requires valid source IDs")
		}
		if sourceID.Bytes == params.TargetID.Bytes {
			return Person{}, errors.New("merge people target cannot also be a source")
		}
		if _, duplicate := seen[sourceID.Bytes]; duplicate {
			continue
		}
		seen[sourceID.Bytes] = struct{}{}
		sourceIDs = append(sourceIDs, sourceID)
	}

	beginner, ok := q.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return Person{}, errors.New("merge people requires a transaction-capable database")
	}

	var target Person
	err := pgx.BeginFunc(ctx, beginner, func(tx pgx.Tx) error {
		var err error
		target, err = mergePeopleInTransaction(ctx, q.WithTx(tx), MergePeopleParams{
			TargetID:  params.TargetID,
			OwnerID:   params.OwnerID,
			SourceIds: sourceIDs,
		})
		return err
	})
	if err != nil {
		return Person{}, err
	}
	return target, nil
}

func mergePeopleInTransaction(ctx context.Context, queries mergePeopleQueries, params MergePeopleParams) (Person, error) {
	// Lock in a deterministic order so inverse concurrent merges do not
	// deadlock merely because their target/source order differs.
	participantIDs := append([]pgtype.UUID{params.TargetID}, params.SourceIds...)
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
			return Person{}, fmt.Errorf("lock merge participant: %w", err)
		}
		if personID.Bytes == params.TargetID.Bytes {
			target = person
		}
	}

	if len(params.SourceIds) == 0 {
		return target, nil
	}
	if err := queries.ReassignPersonFaces(ctx, ReassignPersonFacesParams{
		TargetID:  params.TargetID,
		SourceIds: params.SourceIds,
	}); err != nil {
		return Person{}, fmt.Errorf("reassign source faces: %w", err)
	}

	deleted, err := queries.DeletePeopleForMerge(ctx, DeletePeopleForMergeParams{
		SourceIds: params.SourceIds,
		OwnerID:   params.OwnerID,
	})
	if err != nil {
		return Person{}, fmt.Errorf("delete merged people: %w", err)
	}
	if deleted != int64(len(params.SourceIds)) {
		return Person{}, fmt.Errorf("delete merged people: deleted %d of %d sources", deleted, len(params.SourceIds))
	}
	return target, nil
}
