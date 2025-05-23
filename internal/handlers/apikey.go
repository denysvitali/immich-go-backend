package handlers

import (
	"net/http"

	"github.com/denysvitali/immich-go-backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type APIKeyHandler struct {
	services *services.Services
}

func NewAPIKeyHandler(services *services.Services) *APIKeyHandler {
	return &APIKeyHandler{services: services}
}

// GET /api-keys
func (h *APIKeyHandler) GetAPIKeys(c *gin.Context) {
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

	apiKeys, err := h.services.APIKey.GetAllAPIKeys(userID)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, apiKeys)
}

// GET /api-keys/{id}
func (h *APIKeyHandler) GetAPIKey(c *gin.Context) {
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

	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid API key ID")
		return
	}

	apiKey, err := h.services.APIKey.GetAPIKeyByID(keyID, userID)
	if err != nil {
		respondWithError(c, http.StatusNotFound, err.Error())
		return
	}

	respondWithData(c, apiKey)
}

// POST /api-keys
func (h *APIKeyHandler) CreateAPIKey(c *gin.Context) {
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

	var req services.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	apiKey, err := h.services.APIKey.CreateAPIKey(userID, req)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithData(c, apiKey)
}

// PUT /api-keys/{id}
func (h *APIKeyHandler) UpdateAPIKey(c *gin.Context) {
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

	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid API key ID")
		return
	}

	var req services.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	apiKey, err := h.services.APIKey.UpdateAPIKey(keyID, userID, req)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithData(c, apiKey)
}

// DELETE /api-keys/{id}
func (h *APIKeyHandler) DeleteAPIKey(c *gin.Context) {
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

	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid API key ID")
		return
	}

	if err := h.services.APIKey.DeleteAPIKey(keyID, userID); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithSuccess(c, "API key deleted successfully")
}
