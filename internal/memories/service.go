package memories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrMemoryAssetAccessDenied means the memory is not owned by the caller or
// at least one requested asset is not currently accessible to that owner.
var ErrMemoryAssetAccessDenied = errors.New("memory asset access denied")

type Service struct {
	queries *sqlc.Queries
}

func NewService(queries *sqlc.Queries) *Service {
	return &Service{
		queries: queries,
	}
}

type Memory struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Date        time.Time `json:"date"`
	AssetIDs    []string  `json:"assetIds"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Type        string    `json:"type"` // "on_this_day", "year_ago", "custom"
}

func (s *Service) GetMemories(ctx context.Context, userID string) ([]*Memory, error) {
	// Parse user ID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}

	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Get memories from database
	dbMemories, err := s.queries.GetMemories(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	// Convert to Memory structs
	memories := make([]*Memory, 0, len(dbMemories))
	for _, dbMem := range dbMemories {
		memory, err := s.hydrateMemory(ctx, dbMem, userUUID, userID)
		if err != nil {
			return nil, err
		}
		memories = append(memories, memory)
	}

	return memories, nil
}

// hydrateMemory converts a sqlc.Memory row into the service Memory type,
// populating AssetIDs from the memories_assets_assets join table.
func (s *Service) hydrateMemory(
	ctx context.Context,
	dbMem sqlc.Memory,
	ownerID pgtype.UUID,
	userID string,
) (*Memory, error) {
	// Parse JSON data to get type and other metadata
	var data map[string]interface{}
	if err := json.Unmarshal(dbMem.Data, &data); err != nil {
		data = make(map[string]interface{})
	}

	memoryType := dbMem.Type
	if memoryType == "" {
		memoryType = "on_this_day"
	}

	// Extract title from data or use default
	title, _ := data["title"].(string)
	if title == "" {
		title = "Memory"
	}

	// Extract description
	description, _ := data["description"].(string)

	// Look up the asset IDs linked to this memory so callers can fetch
	// the full asset rows. Failures here are non-fatal: a memory without
	// linked assets is still valid.
	memUUID := pgtype.UUID{Bytes: dbMem.ID.Bytes, Valid: true}
	assetUUIDs, err := s.queries.GetAccessibleMemoryAssetIDs(ctx, sqlc.GetAccessibleMemoryAssetIDsParams{
		MemoryID: memUUID,
		OwnerID:  ownerID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load memory assets: %w", err)
	}

	assetIDs := make([]string, 0, len(assetUUIDs))
	for _, a := range assetUUIDs {
		assetIDs = append(assetIDs, uuid.UUID(a.Bytes).String())
	}

	return &Memory{
		ID:          uuid.UUID(dbMem.ID.Bytes).String(),
		UserID:      userID,
		Title:       title,
		Description: description,
		Date:        dbMem.MemoryAt.Time,
		Type:        memoryType,
		AssetIDs:    assetIDs,
		CreatedAt:   dbMem.CreatedAt.Time,
		UpdatedAt:   dbMem.UpdatedAt.Time,
	}, nil
}

func (s *Service) GetMemory(ctx context.Context, userID string, memoryID string) (*Memory, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Parse memory ID
	memUUID, err := uuid.Parse(memoryID)
	if err != nil {
		return nil, err
	}

	memoryUUID := pgtype.UUID{Bytes: memUUID, Valid: true}

	// Get memory from database
	dbMemory, err := s.queries.GetOwnedMemory(ctx, sqlc.GetOwnedMemoryParams{
		MemoryID: memoryUUID,
		OwnerID:  userUUID,
	})
	if err != nil {
		return nil, err
	}

	return s.hydrateMemory(ctx, dbMemory, userUUID, userID)
}

func (s *Service) CreateMemory(ctx context.Context, memory *Memory) (*Memory, error) {
	// Parse user ID
	userUUID, err := uuid.Parse(memory.UserID)
	if err != nil {
		return nil, err
	}

	pgUserUUID := pgtype.UUID{Bytes: userUUID, Valid: true}

	// Prepare JSON data
	data := map[string]interface{}{
		"title":       memory.Title,
		"description": memory.Description,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// Create memory in database
	dbMemory, err := s.queries.CreateMemory(ctx, sqlc.CreateMemoryParams{
		OwnerId: pgUserUUID,
		Type:    memory.Type,
		Data:    jsonData,
	})
	if err != nil {
		return nil, err
	}

	memory.ID = uuid.UUID(dbMemory.ID.Bytes).String()
	memory.CreatedAt = dbMemory.CreatedAt.Time
	memory.UpdatedAt = dbMemory.UpdatedAt.Time

	return memory, nil
}

func (s *Service) UpdateMemory(ctx context.Context, userID string, memoryID string, updates map[string]interface{}) (*Memory, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Parse memory ID
	memUUID, err := uuid.Parse(memoryID)
	if err != nil {
		return nil, err
	}

	memoryUUID := pgtype.UUID{Bytes: memUUID, Valid: true}

	// Get existing memory first to verify ownership and merge data
	existing, err := s.queries.GetOwnedMemory(ctx, sqlc.GetOwnedMemoryParams{
		MemoryID: memoryUUID,
		OwnerID:  userUUID,
	})
	if err != nil {
		return nil, err
	}

	// Prepare update params
	updateParams := sqlc.UpdateMemoryParams{
		ID: memoryUUID,
	}

	// Handle different update fields
	if memType, ok := updates["type"].(string); ok {
		updateParams.Type = pgtype.Text{String: memType, Valid: true}
	}

	// Parse existing JSON data
	var data map[string]interface{}
	if err := json.Unmarshal(existing.Data, &data); err != nil {
		data = make(map[string]interface{})
	}

	// Handle is_saved in JSON data
	if isSaved, ok := updates["is_saved"].(bool); ok {
		data["is_saved"] = isSaved
	}

	// Update JSON data if title or description provided
	if title, ok := updates["title"].(string); ok {
		data["title"] = title
	}
	if description, ok := updates["description"].(string); ok {
		data["description"] = description
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	updateParams.Data = jsonData

	// Update memory in database
	dbMemory, err := s.queries.UpdateMemory(ctx, updateParams)
	if err != nil {
		return nil, err
	}

	title, _ := data["title"].(string)
	description, _ := data["description"].(string)

	return &Memory{
		ID:          memoryID,
		UserID:      userID,
		Title:       title,
		Description: description,
		Date:        dbMemory.MemoryAt.Time,
		Type:        dbMemory.Type,
		AssetIDs:    []string{},
		CreatedAt:   dbMemory.CreatedAt.Time,
		UpdatedAt:   dbMemory.UpdatedAt.Time,
	}, nil
}

func (s *Service) DeleteMemory(ctx context.Context, userID string, memoryID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Parse memory ID
	memUUID, err := uuid.Parse(memoryID)
	if err != nil {
		return err
	}

	memoryUUID := pgtype.UUID{Bytes: memUUID, Valid: true}

	// Get memory first to verify ownership
	_, err = s.queries.GetOwnedMemory(ctx, sqlc.GetOwnedMemoryParams{
		MemoryID: memoryUUID,
		OwnerID:  userUUID,
	})
	if err != nil {
		return err
	}

	// Delete memory from database (soft delete)
	return s.queries.DeleteMemory(ctx, memoryUUID)
}

func (s *Service) AddAssetsToMemory(ctx context.Context, userID string, memoryID string, assetIDs []string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Parse memory ID
	memUUID, err := uuid.Parse(memoryID)
	if err != nil {
		return fmt.Errorf("invalid memory ID: %w", err)
	}

	memoryUUID := pgtype.UUID{Bytes: memUUID, Valid: true}

	// Parse every ID before touching the database. Invalid IDs reject the whole
	// request instead of silently shrinking it into a partial mutation.
	assetUUIDs := make([]pgtype.UUID, 0, len(assetIDs))
	seen := make(map[uuid.UUID]struct{}, len(assetIDs))
	for _, assetIDStr := range assetIDs {
		assetID, err := uuid.Parse(assetIDStr)
		if err != nil {
			return fmt.Errorf("invalid asset ID: %w", err)
		}
		if _, ok := seen[assetID]; ok {
			continue
		}
		seen[assetID] = struct{}{}
		assetUUIDs = append(assetUUIDs, pgtype.UUID{Bytes: assetID, Valid: true})
	}

	// The guarded statement validates the owned memory and the complete set of
	// owned/shared assets before inserting any relation.
	authorized, err := s.queries.AddAccessibleAssetsToMemory(ctx, sqlc.AddAccessibleAssetsToMemoryParams{
		AssetIds: assetUUIDs,
		MemoryID: memoryUUID,
		OwnerID:  userUUID,
	})
	if err != nil {
		return fmt.Errorf("add assets to memory: %w", err)
	}
	if !authorized {
		return ErrMemoryAssetAccessDenied
	}
	return nil
}

func (s *Service) RemoveAssetsFromMemory(ctx context.Context, userID string, memoryID string, assetIDs []string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Parse memory ID
	memUUID, err := uuid.Parse(memoryID)
	if err != nil {
		return fmt.Errorf("invalid memory ID: %w", err)
	}

	memoryUUID := pgtype.UUID{Bytes: memUUID, Valid: true}

	// Verify memory ownership
	_, err = s.queries.GetOwnedMemory(ctx, sqlc.GetOwnedMemoryParams{
		MemoryID: memoryUUID,
		OwnerID:  userUUID,
	})
	if err != nil {
		return fmt.Errorf("memory not found: %w", err)
	}

	// Convert asset IDs to UUIDs
	var assetUUIDs []pgtype.UUID
	for _, assetIDStr := range assetIDs {
		assetID, err := uuid.Parse(assetIDStr)
		if err != nil {
			continue // Skip invalid IDs
		}
		assetUUIDs = append(assetUUIDs, pgtype.UUID{Bytes: assetID, Valid: true})
	}

	if len(assetUUIDs) == 0 {
		return nil
	}

	// Remove assets from memory
	return s.queries.RemoveAssetsFromMemory(ctx, sqlc.RemoveAssetsFromMemoryParams{
		MemoriesId: memoryUUID,
		Column2:    assetUUIDs,
	})
}

func (s *Service) GetMemoryAssets(ctx context.Context, userID string, memoryID string) ([]string, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Parse memory ID
	memUUID, err := uuid.Parse(memoryID)
	if err != nil {
		return nil, fmt.Errorf("invalid memory ID: %w", err)
	}

	memoryUUID := pgtype.UUID{Bytes: memUUID, Valid: true}

	// Verify memory ownership
	_, err = s.queries.GetOwnedMemory(ctx, sqlc.GetOwnedMemoryParams{
		MemoryID: memoryUUID,
		OwnerID:  userUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("access denied: memory does not belong to user")
		}
		return nil, fmt.Errorf("memory not found: %w", err)
	}

	// Get assets for this memory
	assetUUIDs, err := s.queries.GetAccessibleMemoryAssetIDs(ctx, sqlc.GetAccessibleMemoryAssetIDsParams{
		MemoryID: memoryUUID,
		OwnerID:  userUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get memory assets: %w", err)
	}

	// Convert to string IDs
	assetIDs := make([]string, len(assetUUIDs))
	for i, assetUUID := range assetUUIDs {
		assetIDs[i] = uuid.UUID(assetUUID.Bytes).String()
	}

	return assetIDs, nil
}

func (s *Service) GenerateMemories(ctx context.Context, userID string) error {
	// Generate memories based on date patterns
	// This would typically run as a background job analyzing user's assets
	// and creating memories for special dates, trips, etc.
	// Implementation requires job queue system (not yet available)
	return fmt.Errorf("memory generation requires job queue system implementation")
}

// CountMemories returns the total number of (non-deleted) memories owned by
// the given user.
func (s *Service) CountMemories(ctx context.Context, userID string) (int64, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return 0, err
	}

	return s.queries.CountMemories(ctx, pgtype.UUID{Bytes: uid, Valid: true})
}
