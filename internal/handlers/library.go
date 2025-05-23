package handlers

import (
	"net/http"

	"github.com/denysvitali/immich-go-backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type LibraryHandler struct {
	services *services.Services
}

func NewLibraryHandler(services *services.Services) *LibraryHandler {
	return &LibraryHandler{services: services}
}

// GET /libraries
func (h *LibraryHandler) GetAllLibraries(c *gin.Context) {
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

	libraries, err := h.services.Library.GetAllLibraries(userID)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, libraries)
}

// GET /libraries/{id}
func (h *LibraryHandler) GetLibrary(c *gin.Context) {
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

	libraryIDStr := c.Param("id")
	libraryID, err := uuid.Parse(libraryIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid library ID")
		return
	}

	library, err := h.services.Library.GetLibraryByID(libraryID, userID)
	if err != nil {
		respondWithError(c, http.StatusNotFound, err.Error())
		return
	}

	respondWithData(c, library)
}

// POST /libraries
func (h *LibraryHandler) CreateLibrary(c *gin.Context) {
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

	var req services.CreateLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	library, err := h.services.Library.CreateLibrary(userID, req)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithData(c, library)
}

// PUT /libraries/{id}
func (h *LibraryHandler) UpdateLibrary(c *gin.Context) {
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

	libraryIDStr := c.Param("id")
	libraryID, err := uuid.Parse(libraryIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid library ID")
		return
	}

	var req services.UpdateLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	library, err := h.services.Library.UpdateLibrary(libraryID, userID, req)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithData(c, library)
}

// DELETE /libraries/{id}
func (h *LibraryHandler) DeleteLibrary(c *gin.Context) {
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

	libraryIDStr := c.Param("id")
	libraryID, err := uuid.Parse(libraryIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid library ID")
		return
	}

	if err := h.services.Library.DeleteLibrary(libraryID, userID); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithSuccess(c, "Library deleted successfully")
}

// GET /libraries/{id}/statistics
func (h *LibraryHandler) GetLibraryStatistics(c *gin.Context) {
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

	libraryIDStr := c.Param("id")
	libraryID, err := uuid.Parse(libraryIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid library ID")
		return
	}

	stats, err := h.services.Library.GetLibraryStatistics(libraryID, userID)
	if err != nil {
		respondWithError(c, http.StatusNotFound, err.Error())
		return
	}

	respondWithData(c, stats)
}

// POST /libraries/{id}/scan
func (h *LibraryHandler) ScanLibrary(c *gin.Context) {
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

	libraryIDStr := c.Param("id")
	libraryID, err := uuid.Parse(libraryIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid library ID")
		return
	}

	if err := h.services.Library.ScanLibrary(libraryID, userID); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithSuccess(c, "Library scan initiated")
}
