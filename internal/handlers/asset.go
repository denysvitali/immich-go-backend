package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AssetHandler struct {
	services *services.Services
}

func NewAssetHandler(services *services.Services) *AssetHandler {
	return &AssetHandler{services: services}
}

// GET /assets
func (h *AssetHandler) GetAllAssets(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Parse query parameters
	options := services.AssetSearchOptions{
		UserID: userID,
		Page:   0,
		Size:   250, // Default page size
	}

	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil {
			options.Page = page
		}
	}

	if sizeStr := c.Query("size"); sizeStr != "" {
		if size, err := strconv.Atoi(sizeStr); err == nil {
			options.Size = size
		}
	}

	if assetType := c.Query("type"); assetType != "" {
		options.Type = &assetType
	}

	if favoriteStr := c.Query("isFavorite"); favoriteStr != "" {
		if favorite, err := strconv.ParseBool(favoriteStr); err == nil {
			options.IsFavorite = &favorite
		}
	}

	if archivedStr := c.Query("isArchived"); archivedStr != "" {
		if archived, err := strconv.ParseBool(archivedStr); err == nil {
			options.IsArchived = &archived
		}
	}

	if trashedStr := c.Query("isTrashed"); trashedStr != "" {
		if trashed, err := strconv.ParseBool(trashedStr); err == nil {
			options.IsTrashed = &trashed
		}
	}

	if takenAfterStr := c.Query("takenAfter"); takenAfterStr != "" {
		if takenAfter, err := time.Parse(time.RFC3339, takenAfterStr); err == nil {
			options.TakenAfter = &takenAfter
		}
	}

	if takenBeforeStr := c.Query("takenBefore"); takenBeforeStr != "" {
		if takenBefore, err := time.Parse(time.RFC3339, takenBeforeStr); err == nil {
			options.TakenBefore = &takenBefore
		}
	}

	assets, err := h.services.Asset.GetAllAssets(userID, options)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, assets)
}

// GET /assets/{id}
func (h *AssetHandler) GetAssetInfo(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	assetIDStr := c.Param("id")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid asset ID")
		return
	}

	asset, err := h.services.Asset.GetAssetByID(assetID, userID)
	if err != nil {
		respondWithError(c, http.StatusNotFound, err.Error())
		return
	}

	respondWithData(c, asset)
}

// POST /assets
func (h *AssetHandler) UploadAsset(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req services.CreateAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	asset, err := h.services.Asset.CreateAsset(userID, req)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithData(c, asset)
}

// PUT /assets/{id}
func (h *AssetHandler) UpdateAsset(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	assetIDStr := c.Param("id")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid asset ID")
		return
	}

	var req services.UpdateAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	asset, err := h.services.Asset.UpdateAsset(assetID, userID, req)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithData(c, asset)
}

// DELETE /assets
func (h *AssetHandler) DeleteAssets(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		IDs   []uuid.UUID `json:"ids" binding:"required"`
		Force *bool       `json:"force,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	force := req.Force != nil && *req.Force

	if force {
		// Permanently delete assets
		for _, assetID := range req.IDs {
			if err := h.services.Asset.DeleteAsset(assetID, userID); err != nil {
				respondWithError(c, http.StatusInternalServerError, err.Error())
				return
			}
		}
	} else {
		// Move to trash
		if err := h.services.Asset.TrashAssets(userID, req.IDs); err != nil {
			respondWithError(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	respondWithSuccess(c, "Assets deleted successfully")
}

// POST /assets/restore
func (h *AssetHandler) RestoreAssets(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		IDs []uuid.UUID `json:"ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.services.Asset.RestoreAssets(userID, req.IDs); err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithSuccess(c, "Assets restored successfully")
}

// GET /assets/statistics
func (h *AssetHandler) GetAssetStatistics(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	stats, err := h.services.Asset.GetAssetStatistics(userID)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, stats)
}

// GET /assets/memory-lane
func (h *AssetHandler) GetMemoryLane(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get day and month from query parameters (default to today)
	now := time.Now()
	day := now.Day()
	month := int(now.Month())

	if dayStr := c.Query("day"); dayStr != "" {
		if d, err := strconv.Atoi(dayStr); err == nil {
			day = d
		}
	}

	if monthStr := c.Query("month"); monthStr != "" {
		if m, err := strconv.Atoi(monthStr); err == nil {
			month = m
		}
	}

	assets, err := h.services.Asset.GetMemoryLane(userID, day, month)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, assets)
}

// GET /assets/{id}/thumbnail
func (h *AssetHandler) GetAssetThumbnail(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	assetIDStr := c.Param("id")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid asset ID")
		return
	}

	format := c.Query("format")
	if format == "" {
		format = "WEBP"
	}

	thumbnailPath, err := h.services.Asset.GetAssetThumbnail(assetID, userID, format)
	if err != nil {
		respondWithError(c, http.StatusNotFound, err.Error())
		return
	}

	// TODO: Serve the actual file
	// For now, just return the path
	respondWithData(c, gin.H{
		"thumbnailPath": thumbnailPath,
	})
}

// POST /assets/exist
func (h *AssetHandler) CheckExistingAssets(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		DeviceAssetIds []string `json:"deviceAssetIds" binding:"required"`
		DeviceId       string   `json:"deviceId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := h.services.Asset.CheckExistingAssets(userID, req.DeviceAssetIds, req.DeviceId)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, existing)
}

// POST /assets/bulk-upload-check
func (h *AssetHandler) BulkUploadCheck(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		Assets []services.CreateAssetRequest `json:"assets" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	results, err := h.services.Asset.BulkUploadCheck(userID, req.Assets)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, gin.H{
		"results": results,
	})
}

// GET /assets/map-marker
func (h *AssetHandler) GetMapMarkers(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// TODO: Implement map markers based on asset GPS coordinates
	// For now, return empty array
	respondWithData(c, []interface{}{})
}

// GET /assets/random
func (h *AssetHandler) GetRandomAssets(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	count := 1
	if countStr := c.Query("count"); countStr != "" {
		if c, err := strconv.Atoi(countStr); err == nil {
			count = c
		}
	}

	// Get random assets by getting all and limiting
	options := services.AssetSearchOptions{
		UserID: userID,
		Size:   count,
	}

	assets, err := h.services.Asset.GetAllAssets(userID, options)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, assets)
}
