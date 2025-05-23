package handlers

import (
	"net/http"

	"github.com/denysvitali/immich-go-backend/internal/services"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	services *services.Services
}

func NewAuthHandler(services *services.Services) *AuthHandler {
	return &AuthHandler{services: services}
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string                  `json:"accessToken"`
	RefreshToken string                  `json:"refreshToken"`
	UserEmail    string                  `json:"userEmail"`
	UserID       string                  `json:"userId"`
	UserInfo     services.UserResponse   `json:"userInfo"`
}

type SignUpRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"firstName" binding:"required"`
	LastName  string `json:"lastName" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=8"`
}

type LogoutRequest struct {
	AccessToken string `json:"accessToken" binding:"required"`
}

// POST /auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.services.Auth.Login(req.Email, req.Password)
	if err != nil {
		respondWithError(c, http.StatusUnauthorized, err.Error())
		return
	}

	// Get user info
	userInfo, err := h.services.User.GetUserByID(result.UserID)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, "Failed to get user info")
		return
	}

	response := LoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		UserEmail:    userInfo.Email,
		UserID:       result.UserID.String(),
		UserInfo:     *userInfo,
	}

	respondWithData(c, response)
}

// POST /auth/admin-sign-up
func (h *AuthHandler) AdminSignUp(c *gin.Context) {
	var req SignUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.services.Auth.AdminSignUp(req.Email, req.Password, req.FirstName, req.LastName)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Get user info
	userInfo, err := h.services.User.GetUserByID(result.UserID)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, "Failed to get user info")
		return
	}

	response := LoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		UserEmail:    userInfo.Email,
		UserID:       result.UserID.String(),
		UserInfo:     *userInfo,
	}

	respondWithData(c, response)
}

// POST /auth/change-password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	err := h.services.Auth.ChangePassword(userID.(string), req.CurrentPassword, req.NewPassword)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithSuccess(c, "Password changed successfully")
}

// POST /auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	err := h.services.Auth.Logout(req.AccessToken)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondWithSuccess(c, "Logged out successfully")
}

// POST /auth/validate-access-token
func (h *AuthHandler) ValidateAccessToken(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	isValid, err := h.services.Auth.ValidateToken(userID.(string))
	if err != nil || !isValid {
		respondWithError(c, http.StatusUnauthorized, "Invalid token")
		return
	}

	respondWithData(c, gin.H{
		"authStatus": true,
	})
}

// GET /auth/devices
func (h *AuthHandler) GetAuthDevices(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// TODO: Implement device management
	// For now, return empty array
	respondWithData(c, []interface{}{})
}

// DELETE /auth/devices
func (h *AuthHandler) LogoutAuthDevices(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// TODO: Implement device logout
	// For now, just return success
	respondWithSuccess(c, "All devices logged out successfully")
}

// DELETE /auth/devices/{id}
func (h *AuthHandler) LogoutAuthDevice(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		respondWithError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	deviceID := c.Param("id")
	if deviceID == "" {
		respondWithError(c, http.StatusBadRequest, "Device ID is required")
		return
	}

	// TODO: Implement specific device logout
	// For now, just return success
	respondWithSuccess(c, "Device logged out successfully")
}
