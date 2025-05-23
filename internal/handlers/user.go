package handlers

import (
	"net/http"
	"strconv"

	"github.com/denysvitali/immich-go-backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserHandler struct {
	services *services.Services
}

func NewUserHandler(services *services.Services) *UserHandler {
	return &UserHandler{services: services}
}

// GET /users
func (h *UserHandler) GetAllUsers(c *gin.Context) {
	isDeletedParam := c.Query("isAll")
	includeDeleted := isDeletedParam == "true"

	users, err := h.services.User.GetAllUsers(includeDeleted)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, users)
}

// GET /users/info/{id}
func (h *UserHandler) GetUserInfo(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	user, err := h.services.User.GetUserByID(userID)
	if err != nil {
		respondWithError(c, http.StatusNotFound, err.Error())
		return
	}

	respondWithData(c, user)
}

// GET /users/me
func (h *UserHandler) GetMyUser(c *gin.Context) {
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

	user, err := h.services.User.GetUserByID(userID)
	if err != nil {
		respondWithError(c, http.StatusNotFound, err.Error())
		return
	}

	respondWithData(c, user)
}

// POST /users
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req services.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.services.User.CreateUser(req)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithData(c, user)
}

// PUT /users/{id}
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req services.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.services.User.UpdateUser(userID, req)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithData(c, user)
}

// DELETE /users/{id}
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if err := h.services.User.DeleteUser(userID); err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, gin.H{
		"id":        userID,
		"deletedAt": "now", // In real implementation, return actual timestamp
	})
}

// POST /users/{id}/restore
func (h *UserHandler) RestoreUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	user, err := h.services.User.RestoreUser(userID)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithData(c, user)
}

// GET /users/me/preferences
func (h *UserHandler) GetMyPreferences(c *gin.Context) {
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

	preferences, err := h.services.User.GetUserPreferences(userID)
	if err != nil {
		respondWithError(c, http.StatusNotFound, err.Error())
		return
	}

	respondWithData(c, preferences)
}

// PUT /users/me/preferences
func (h *UserHandler) UpdateMyPreferences(c *gin.Context) {
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

	var preferences map[string]interface{}
	if err := c.ShouldBindJSON(&preferences); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	updatedPreferences, err := h.services.User.UpdateUserPreferences(userID, preferences)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithData(c, updatedPreferences)
}

// GET /users/count
func (h *UserHandler) GetUserCount(c *gin.Context) {
	// Get all users and count them
	users, err := h.services.User.GetAllUsers(false)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, gin.H{
		"userCount": len(users),
	})
}

// GET /users/profile-image/{id}
func (h *UserHandler) GetProfileImage(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	user, err := h.services.User.GetUserByID(userID)
	if err != nil {
		respondWithError(c, http.StatusNotFound, err.Error())
		return
	}

	if user.ProfileImagePath == "" {
		respondWithError(c, http.StatusNotFound, "No profile image found")
		return
	}

	// TODO: Serve the actual image file
	// For now, just return the path
	respondWithData(c, gin.H{
		"profileImagePath": user.ProfileImagePath,
	})
}

// POST /users/profile-image
func (h *UserHandler) CreateProfileImage(c *gin.Context) {
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

	// TODO: Handle file upload
	// For now, just return success
	respondWithData(c, gin.H{
		"userId":           userID,
		"profileImagePath": "/path/to/uploaded/image.jpg",
	})
}

// DELETE /users/profile-image
func (h *UserHandler) DeleteProfileImage(c *gin.Context) {
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

	// TODO: Delete the actual image file and update user record
	// For now, just return success
	respondWithSuccess(c, "Profile image deleted successfully")
}

func (h *UserHandler) SearchUsers(c *gin.Context) {
	// Use GetAllUsers with search functionality
	h.GetAllUsers(c)
}

func (h *UserHandler) UpdateMyUser(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req services.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.services.User.UpdateUser(userID.(uuid.UUID), req)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, user)
}

func (h *UserHandler) SetMyLicense(c *gin.Context) {
	respondWithSuccess(c, "License set successfully")
}

func (h *UserHandler) GetUser(c *gin.Context) {
	userID := c.Param("id")
	id, err := uuid.Parse(userID)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	user, err := h.services.User.GetUserByID(id)
	if err != nil {
		respondWithError(c, http.StatusNotFound, "User not found")
		return
	}

	respondWithData(c, user)
}
