package memories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

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

		memories = append(memories, &Memory{
			ID:          uuid.UUID(dbMem.ID.Bytes).String(),
			UserID:      userID,
			Title:       title,
			Description: description,
			Date:        dbMem.MemoryAt.Time,
			Type:        memoryType,
			AssetIDs:    []string{}, // Would need to query memory_assets table
			CreatedAt:   dbMem.CreatedAt.Time,
			UpdatedAt:   dbMem.UpdatedAt.Time,
		})
	}

	return memories, nil
}

func (s *Service) GetMemory(ctx context.Context, userID string, memoryID string) (*Memory, error) {
	// Parse memory ID
	memUUID, err := uuid.Parse(memoryID)
	if err != nil {
		return nil, err
	}

	memoryUUID := pgtype.UUID{Bytes: memUUID, Valid: true}

	// Get memory from database
	dbMemory, err := s.queries.GetMemory(ctx, memoryUUID)
	if err != nil {
		return nil, err
	}

	// Verify ownership
	if uuid.UUID(dbMemory.OwnerId.Bytes).String() != userID {
		return nil, err // Should return permission denied error
	}

	// Parse JSON data
	var data map[string]interface{}
	if err := json.Unmarshal(dbMemory.Data, &data); err != nil {
		data = make(map[string]interface{})
	}

	title, _ := data["title"].(string)
	if title == "" {
		title = "Memory"
	}

	description, _ := data["description"].(string)

	return &Memory{
		ID:          memoryID,
		UserID:      userID,
		Title:       title,
		Description: description,
		Date:        dbMemory.MemoryAt.Time,
		Type:        dbMemory.Type,
		AssetIDs:    []string{}, // Would need to query memory_assets table
		CreatedAt:   dbMemory.CreatedAt.Time,
		UpdatedAt:   dbMemory.UpdatedAt.Time,
	}, nil
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
	// Parse memory ID
	memUUID, err := uuid.Parse(memoryID)
	if err != nil {
		return nil, err
	}

	memoryUUID := pgtype.UUID{Bytes: memUUID, Valid: true}

	// Get existing memory first to verify ownership and merge data
	existing, err := s.queries.GetMemory(ctx, memoryUUID)
	if err != nil {
		return nil, err
	}

	// Verify ownership
	if uuid.UUID(existing.OwnerId.Bytes).String() != userID {
		return nil, err // Should return permission denied error
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
	// Parse memory ID
	memUUID, err := uuid.Parse(memoryID)
	if err != nil {
		return err
	}

	memoryUUID := pgtype.UUID{Bytes: memUUID, Valid: true}

	// Get memory first to verify ownership
	dbMemory, err := s.queries.GetMemory(ctx, memoryUUID)
	if err != nil {
		return err
	}

	// Verify ownership
	if uuid.UUID(dbMemory.OwnerId.Bytes).String() != userID {
		return err // Should return permission denied error
	}

	// Delete memory from database (soft delete)
	return s.queries.DeleteMemory(ctx, memoryUUID)
}

func (s *Service) AddAssetsToMemory(ctx context.Context, userID string, memoryID string, assetIDs []string) error {
	// Parse memory ID
	memUUID, err := uuid.Parse(memoryID)
	if err != nil {
		return fmt.Errorf("invalid memory ID: %w", err)
	}

	memoryUUID := pgtype.UUID{Bytes: memUUID, Valid: true}

	// Verify memory ownership
	dbMemory, err := s.queries.GetMemory(ctx, memoryUUID)
	if err != nil {
		return fmt.Errorf("memory not found: %w", err)
	}

	if uuid.UUID(dbMemory.OwnerId.Bytes).String() != userID {
		return fmt.Errorf("access denied: memory does not belong to user")
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

	// Add assets to memory
	return s.queries.AddAssetsToMemory(ctx, sqlc.AddAssetsToMemoryParams{
		MemoriesId: memoryUUID,
		Column2:    assetUUIDs,
	})
}

func (s *Service) RemoveAssetsFromMemory(ctx context.Context, userID string, memoryID string, assetIDs []string) error {
	// Parse memory ID
	memUUID, err := uuid.Parse(memoryID)
	if err != nil {
		return fmt.Errorf("invalid memory ID: %w", err)
	}

	memoryUUID := pgtype.UUID{Bytes: memUUID, Valid: true}

	// Verify memory ownership
	dbMemory, err := s.queries.GetMemory(ctx, memoryUUID)
	if err != nil {
		return fmt.Errorf("memory not found: %w", err)
	}

	if uuid.UUID(dbMemory.OwnerId.Bytes).String() != userID {
		return fmt.Errorf("access denied: memory does not belong to user")
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
	// Parse memory ID
	memUUID, err := uuid.Parse(memoryID)
	if err != nil {
		return nil, fmt.Errorf("invalid memory ID: %w", err)
	}

	memoryUUID := pgtype.UUID{Bytes: memUUID, Valid: true}

	// Verify memory ownership
	dbMemory, err := s.queries.GetMemory(ctx, memoryUUID)
	if err != nil {
		return nil, fmt.Errorf("memory not found: %w", err)
	}

	if uuid.UUID(dbMemory.OwnerId.Bytes).String() != userID {
		return nil, fmt.Errorf("access denied: memory does not belong to user")
	}

	// Get assets for this memory
	assetUUIDs, err := s.queries.GetMemoryAssets(ctx, memoryUUID)
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