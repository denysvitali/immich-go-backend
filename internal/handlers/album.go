package handlers

import (
	"net/http"
	"strconv"

	"github.com/denysvitali/immich-go-backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AlbumHandler struct {
	services *services.Services
}

func NewAlbumHandler(services *services.Services) *AlbumHandler {
	return &AlbumHandler{services: services}
}

// GET /albums
func (h *AlbumHandler) GetAllAlbums(c *gin.Context) {
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

	var shared *bool
	if sharedParam := c.Query("shared"); sharedParam != "" {
		if sharedVal, err := strconv.ParseBool(sharedParam); err == nil {
			shared = &sharedVal
		}
	}

	albums, err := h.services.Album.GetAllAlbums(userID, shared)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, albums)
}

// GET /albums/{id}
func (h *AlbumHandler) GetAlbumInfo(c *gin.Context) {
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

	albumIDStr := c.Param("id")
	albumID, err := uuid.Parse(albumIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid album ID")
		return
	}

	album, err := h.services.Album.GetAlbumByID(albumID, userID)
	if err != nil {
		respondWithError(c, http.StatusNotFound, err.Error())
		return
	}

	respondWithData(c, album)
}

// POST /albums
func (h *AlbumHandler) CreateAlbum(c *gin.Context) {
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

	var req services.CreateAlbumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	album, err := h.services.Album.CreateAlbum(userID, req)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithData(c, album)
}

// PATCH /albums/{id}
func (h *AlbumHandler) UpdateAlbumInfo(c *gin.Context) {
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

	albumIDStr := c.Param("id")
	albumID, err := uuid.Parse(albumIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid album ID")
		return
	}

	var req services.UpdateAlbumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	album, err := h.services.Album.UpdateAlbum(albumID, userID, req)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithData(c, album)
}

// DELETE /albums/{id}
func (h *AlbumHandler) DeleteAlbum(c *gin.Context) {
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

	albumIDStr := c.Param("id")
	albumID, err := uuid.Parse(albumIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid album ID")
		return
	}

	if err := h.services.Album.DeleteAlbum(albumID, userID); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithSuccess(c, "Album deleted successfully")
}

// PUT /albums/{id}/assets
func (h *AlbumHandler) AddAssetsToAlbum(c *gin.Context) {
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

	albumIDStr := c.Param("id")
	albumID, err := uuid.Parse(albumIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid album ID")
		return
	}

	var req services.AddAssetsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	album, err := h.services.Album.AddAssets(albumID, userID, req)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithData(c, album)
}

// DELETE /albums/{id}/assets
func (h *AlbumHandler) RemoveAssetFromAlbum(c *gin.Context) {
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

	albumIDStr := c.Param("id")
	albumID, err := uuid.Parse(albumIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid album ID")
		return
	}

	var req services.RemoveAssetsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	album, err := h.services.Album.RemoveAssets(albumID, userID, req)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithData(c, album)
}

// GET /albums/count
func (h *AlbumHandler) GetAlbumCount(c *gin.Context) {
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

	albums, err := h.services.Album.GetAllAlbums(userID, nil)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, gin.H{
		"owned":  len(albums), // TODO: Separate owned vs shared count
		"shared": 0,
		"notShared": len(albums),
	})
}
