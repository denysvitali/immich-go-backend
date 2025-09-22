package sync

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
)

// Service handles synchronization logic for mobile clients
type Service struct {
	queries *sqlc.Queries
	logger  *logrus.Logger

	// In-memory storage for sync acknowledgments (per user)
	// In production, this should be stored in Redis or database
	syncAckMutex sync.RWMutex
	syncAcks     map[string]map[string]bool // userID -> assetID -> acknowledged

	// Track last sync timestamps per user
	lastSyncMutex sync.RWMutex
	lastSync      map[string]time.Time

	// Event broadcasting for real-time sync
	eventMutex sync.RWMutex
	eventSubscribers map[string][]chan *SyncEvent // userID -> list of subscriber channels
}

// NewService creates a new sync service
func NewService(queries *sqlc.Queries, logger *logrus.Logger) *Service {
	return &Service{
		queries:          queries,
		logger:           logger,
		syncAcks:         make(map[string]map[string]bool),
		lastSync:         make(map[string]time.Time),
		eventSubscribers: make(map[string][]chan *SyncEvent),
	}
}

// SyncState represents the synchronization state for a user
type SyncState struct {
	UserID             string
	LastSyncTime       time.Time
	PendingAssets      []string
	AcknowledgedAssets []string
}

// SyncEvent represents a real-time sync event
type SyncEvent struct {
	Type      string    // "asset", "album", "partner"
	Action    string    // "upsert", "delete"
	UserID    string    // User who owns the resource
	ResourceID string   // ID of the asset/album/partner
	Timestamp time.Time
	Data      interface{} // Optional additional data
}

// DeltaSyncResult contains changes since last sync
type DeltaSyncResult struct {
	NeedsFullSync  bool
	UpsertedAssets []string
	DeletedAssets  []string
}

// GetDeltaSync returns changes since the specified timestamp
func (s *Service) GetDeltaSync(ctx context.Context, userID string, updatedAfter time.Time) (*DeltaSyncResult, error) {
	// Check if we need full sync (e.g., first sync or too long since last sync)
	s.lastSyncMutex.RLock()
	lastSyncTime, exists := s.lastSync[userID]
	s.lastSyncMutex.RUnlock()

	// If no previous sync or more than 7 days since last sync, require full sync
	if !exists || time.Since(lastSyncTime) > 7*24*time.Hour {
		return &DeltaSyncResult{
			NeedsFullSync: true,
		}, nil
	}

	// Get assets modified after the specified time
	userUUID := pgtype.UUID{}
	if err := userUUID.Scan(userID); err != nil {
		return nil, err
	}

	// Get recently modified assets - for now, just get all user's assets
	// In production, this would filter by updatedAt timestamp
	assets, err := s.queries.GetUserAssets(ctx, sqlc.GetUserAssetsParams{
		OwnerId: userUUID,
		Status:  sqlc.NullAssetsStatusEnum{},
		Offset:  pgtype.Int4{Int32: 0, Valid: true},
		Limit:   pgtype.Int4{Int32: 100, Valid: true},
	})
	if err != nil {
		// If query fails, fall back to full sync
		s.logger.WithError(err).Warn("Failed to get modified assets, suggesting full sync")
		return &DeltaSyncResult{
			NeedsFullSync: true,
		}, nil
	}

	// Extract asset IDs from assets modified after the specified time
	upserted := []string{}
	for _, asset := range assets {
		if asset.UpdatedAt.Valid && asset.UpdatedAt.Time.After(updatedAfter) {
			upserted = append(upserted, asset.ID.String())
		}
	}

	// Get deleted assets (would need a separate deleted_assets table in production)
	// For now, return empty deleted list
	deleted := []string{}

	// Update last sync time
	s.lastSyncMutex.Lock()
	s.lastSync[userID] = time.Now()
	s.lastSyncMutex.Unlock()

	return &DeltaSyncResult{
		NeedsFullSync:  false,
		UpsertedAssets: upserted,
		DeletedAssets:  deleted,
	}, nil
}

// GetFullSync returns all asset IDs for a user with pagination
func (s *Service) GetFullSync(ctx context.Context, userID string, limit int, updatedUntil *time.Time) ([]string, bool, *time.Time, error) {
	userUUID := pgtype.UUID{}
	if err := userUUID.Scan(userID); err != nil {
		return nil, false, nil, err
	}

	// Default limit if not specified
	if limit <= 0 {
		limit = 1000
	}

	// Get assets for user
	params := sqlc.GetUserAssetsParams{
		OwnerId: userUUID,
		Status:  sqlc.NullAssetsStatusEnum{},
		Offset:  pgtype.Int4{Int32: 0, Valid: true},
		Limit:   pgtype.Int4{Int32: int32(limit + 1), Valid: true}, // Get one extra to check if there are more
	}

	assets, err := s.queries.GetUserAssets(ctx, params)
	if err != nil {
		return nil, false, nil, err
	}

	hasMore := len(assets) > limit
	if hasMore {
		assets = assets[:limit]
	}

	// Extract asset IDs
	assetIDs := make([]string, len(assets))
	var lastUpdated *time.Time

	for i, asset := range assets {
		assetIDs[i] = asset.ID.String()
		if asset.UpdatedAt.Valid {
			lastUpdated = &asset.UpdatedAt.Time
		}
	}

	// Update last sync time
	s.lastSyncMutex.Lock()
	s.lastSync[userID] = time.Now()
	s.lastSyncMutex.Unlock()

	return assetIDs, hasMore, lastUpdated, nil
}

// AcknowledgeSync marks assets as acknowledged by the client
func (s *Service) AcknowledgeSync(ctx context.Context, userID string, assetIDs []string) error {
	s.syncAckMutex.Lock()
	defer s.syncAckMutex.Unlock()

	if s.syncAcks[userID] == nil {
		s.syncAcks[userID] = make(map[string]bool)
	}

	for _, assetID := range assetIDs {
		s.syncAcks[userID][assetID] = true
	}

	s.logger.WithFields(logrus.Fields{
		"user_id": userID,
		"count":   len(assetIDs),
	}).Debug("Acknowledged sync for assets")

	return nil
}

// GetAcknowledgedAssets returns the list of acknowledged asset IDs for a user
func (s *Service) GetAcknowledgedAssets(ctx context.Context, userID string) ([]string, error) {
	s.syncAckMutex.RLock()
	defer s.syncAckMutex.RUnlock()

	userAcks := s.syncAcks[userID]
	if userAcks == nil {
		return []string{}, nil
	}

	assetIDs := make([]string, 0, len(userAcks))
	for assetID, acked := range userAcks {
		if acked {
			assetIDs = append(assetIDs, assetID)
		}
	}

	return assetIDs, nil
}

// DeleteAcknowledgment removes acknowledgment for specified assets
func (s *Service) DeleteAcknowledgment(ctx context.Context, userID string, assetIDs []string) error {
	s.syncAckMutex.Lock()
	defer s.syncAckMutex.Unlock()

	userAcks := s.syncAcks[userID]
	if userAcks == nil {
		return nil
	}

	for _, assetID := range assetIDs {
		delete(userAcks, assetID)
	}

	s.logger.WithFields(logrus.Fields{
		"user_id": userID,
		"count":   len(assetIDs),
	}).Debug("Deleted acknowledgments for assets")

	return nil
}

// GetSyncState returns the current sync state for a user
func (s *Service) GetSyncState(ctx context.Context, userID string) (*SyncState, error) {
	s.lastSyncMutex.RLock()
	lastSync := s.lastSync[userID]
	s.lastSyncMutex.RUnlock()

	acknowledged, _ := s.GetAcknowledgedAssets(ctx, userID)

	return &SyncState{
		UserID:             userID,
		LastSyncTime:       lastSync,
		AcknowledgedAssets: acknowledged,
	}, nil
}

// ClearUserSyncState clears all sync state for a user
func (s *Service) ClearUserSyncState(ctx context.Context, userID string) error {
	s.syncAckMutex.Lock()
	delete(s.syncAcks, userID)
	s.syncAckMutex.Unlock()

	s.lastSyncMutex.Lock()
	delete(s.lastSync, userID)
	s.lastSyncMutex.Unlock()

	s.logger.WithField("user_id", userID).Info("Cleared sync state for user")

	return nil
}

// SubscribeToEvents creates a new event channel for a user to receive real-time events
func (s *Service) SubscribeToEvents(userID string) chan *SyncEvent {
	s.eventMutex.Lock()
	defer s.eventMutex.Unlock()

	// Create a buffered channel to avoid blocking
	eventChan := make(chan *SyncEvent, 100)

	// Add to subscribers list
	if s.eventSubscribers[userID] == nil {
		s.eventSubscribers[userID] = []chan *SyncEvent{}
	}
	s.eventSubscribers[userID] = append(s.eventSubscribers[userID], eventChan)

	s.logger.WithField("userID", userID).Debug("User subscribed to sync events")
	return eventChan
}

// UnsubscribeFromEvents removes an event channel from the subscribers list
func (s *Service) UnsubscribeFromEvents(userID string, eventChan chan *SyncEvent) {
	s.eventMutex.Lock()
	defer s.eventMutex.Unlock()

	if subscribers, exists := s.eventSubscribers[userID]; exists {
		for i, ch := range subscribers {
			if ch == eventChan {
				// Remove from slice
				s.eventSubscribers[userID] = append(subscribers[:i], subscribers[i+1:]...)
				close(eventChan)
				s.logger.WithField("userID", userID).Debug("User unsubscribed from sync events")
				break
			}
		}

		// Clean up empty subscriber lists
		if len(s.eventSubscribers[userID]) == 0 {
			delete(s.eventSubscribers, userID)
		}
	}
}

// BroadcastAssetEvent broadcasts an asset change event to all subscribers
func (s *Service) BroadcastAssetEvent(ownerID string, assetID string, action string) {
	event := &SyncEvent{
		Type:       "asset",
		Action:     action,
		UserID:     ownerID,
		ResourceID: assetID,
		Timestamp:  time.Now(),
	}
	s.broadcastEvent(ownerID, event)
}

// BroadcastAlbumEvent broadcasts an album change event to all subscribers
func (s *Service) BroadcastAlbumEvent(ownerID string, albumID string, action string) {
	event := &SyncEvent{
		Type:       "album",
		Action:     action,
		UserID:     ownerID,
		ResourceID: albumID,
		Timestamp:  time.Now(),
	}
	s.broadcastEvent(ownerID, event)
}

// BroadcastPartnerEvent broadcasts a partner change event to all subscribers
func (s *Service) BroadcastPartnerEvent(userID string, partnerID string, action string) {
	event := &SyncEvent{
		Type:       "partner",
		Action:     action,
		UserID:     userID,
		ResourceID: partnerID,
		Timestamp:  time.Now(),
	}
	s.broadcastEvent(userID, event)
}

// broadcastEvent sends an event to all subscribers for a user
func (s *Service) broadcastEvent(userID string, event *SyncEvent) {
	s.eventMutex.RLock()
	subscribers := s.eventSubscribers[userID]
	s.eventMutex.RUnlock()

	if len(subscribers) == 0 {
		return
	}

	s.logger.WithFields(logrus.Fields{
		"userID": userID,
		"type":   event.Type,
		"action": event.Action,
		"resourceID": event.ResourceID,
		"subscribers": len(subscribers),
	}).Debug("Broadcasting sync event")

	// Send to all subscribers (non-blocking)
	for _, ch := range subscribers {
		select {
		case ch <- event:
			// Sent successfully
		default:
			// Channel is full, skip this event
			s.logger.WithField("userID", userID).Warn("Event channel full, dropping event")
		}
	}
}
