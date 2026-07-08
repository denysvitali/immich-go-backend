//go:build integration
// +build integration

package search

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchPerson_ShortQueries(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries, nil, nil)
	server := NewServer(service)

	// Create two users to verify ownership isolation.
	ownerID := uuid.New()
	_, err := tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:          pgtype.UUID{Bytes: ownerID, Valid: true},
		Email:       "owner@example.com",
		Name:        "Owner",
		Password:    "secret",
		IsAdmin:     false,
		IsOnboarded: true,
	})
	require.NoError(t, err)

	otherID := uuid.New()
	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:          pgtype.UUID{Bytes: otherID, Valid: true},
		Email:       "other@example.com",
		Name:        "Other",
		Password:    "secret",
		IsAdmin:     false,
		IsOnboarded: true,
	})
	require.NoError(t, err)

	ownerCtx := auth.WithClaims(ctx, &auth.Claims{UserID: ownerID.String()})

	// Create people for the owner.
	alice, err := tdb.Queries.CreatePerson(ctx, sqlc.CreatePersonParams{
		OwnerId:       pgtype.UUID{Bytes: ownerID, Valid: true},
		Name:          "Alice",
		ThumbnailPath: "thumbs/alice.jpg",
		IsHidden:      false,
	})
	require.NoError(t, err)

	_, err = tdb.Queries.CreatePerson(ctx, sqlc.CreatePersonParams{
		OwnerId:       pgtype.UUID{Bytes: ownerID, Valid: true},
		Name:          "Bob",
		ThumbnailPath: "thumbs/bob.jpg",
		IsHidden:      true,
	})
	require.NoError(t, err)

	// Person owned by another user should not appear in search results.
	_, err = tdb.Queries.CreatePerson(ctx, sqlc.CreatePersonParams{
		OwnerId:       pgtype.UUID{Bytes: otherID, Valid: true},
		Name:          "Ally",
		ThumbnailPath: "thumbs/ally.jpg",
		IsHidden:      false,
	})
	require.NoError(t, err)

	t.Run("single character query", func(t *testing.T) {
		resp, err := server.SearchPerson(ownerCtx, &immichv1.SearchPersonRequest{Name: "A"})
		require.NoError(t, err)
		require.Len(t, resp.People, 1)
		assert.Equal(t, uuid.UUID(alice.ID.Bytes).String(), resp.People[0].Id)
		assert.Equal(t, "Alice", resp.People[0].Name)
		assert.Equal(t, "thumbs/alice.jpg", resp.People[0].ThumbnailPath)
		assert.False(t, resp.People[0].IsHidden)
	})

	t.Run("two character query", func(t *testing.T) {
		resp, err := server.SearchPerson(ownerCtx, &immichv1.SearchPersonRequest{Name: "li"})
		require.NoError(t, err)
		require.Len(t, resp.People, 1)
		assert.Equal(t, "Alice", resp.People[0].Name)
	})

	t.Run("case insensitive match", func(t *testing.T) {
		resp, err := server.SearchPerson(ownerCtx, &immichv1.SearchPersonRequest{Name: "aLi"})
		require.NoError(t, err)
		require.Len(t, resp.People, 1)
		assert.Equal(t, "Alice", resp.People[0].Name)
	})

	t.Run("hidden people excluded by default", func(t *testing.T) {
		resp, err := server.SearchPerson(ownerCtx, &immichv1.SearchPersonRequest{Name: "Bob"})
		require.NoError(t, err)
		assert.Empty(t, resp.People)
	})

	t.Run("hidden people included when requested", func(t *testing.T) {
		withHidden := true
		resp, err := server.SearchPerson(ownerCtx, &immichv1.SearchPersonRequest{Name: "Bob", WithHidden: &withHidden})
		require.NoError(t, err)
		require.Len(t, resp.People, 1)
		assert.Equal(t, "Bob", resp.People[0].Name)
		assert.True(t, resp.People[0].IsHidden)
	})
}
