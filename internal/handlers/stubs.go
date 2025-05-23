package handlers

import (
	"net/http"

	"github.com/denysvitali/immich-go-backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Stub handlers for completeness - these can be expanded later

type TagHandler struct {
	services *services.Services
}

func NewTagHandler(services *services.Services) *TagHandler {
	return &TagHandler{services: services}
}

func (h *TagHandler) GetAllTags(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

func (h *TagHandler) CreateTag(c *gin.Context) {
	respondWithSuccess(c, "Tag created")
}

type ActivityHandler struct {
	services *services.Services
}

func NewActivityHandler(services *services.Services) *ActivityHandler {
	return &ActivityHandler{services: services}
}

func (h *ActivityHandler) GetActivities(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

func (h *ActivityHandler) CreateActivity(c *gin.Context) {
	respondWithSuccess(c, "Activity created")
}

type NotificationHandler struct {
	services *services.Services
}

func NewNotificationHandler(services *services.Services) *NotificationHandler {
	return &NotificationHandler{services: services}
}

func (h *NotificationHandler) GetNotifications(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

type PartnerHandler struct {
	services *services.Services
}

func NewPartnerHandler(services *services.Services) *PartnerHandler {
	return &PartnerHandler{services: services}
}

func (h *PartnerHandler) GetPartners(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

type PersonHandler struct {
	services *services.Services
}

func NewPersonHandler(services *services.Services) *PersonHandler {
	return &PersonHandler{services: services}
}

func (h *PersonHandler) GetAllPeople(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

type SharedLinkHandler struct {
	services *services.Services
}

func NewSharedLinkHandler(services *services.Services) *SharedLinkHandler {
	return &SharedLinkHandler{services: services}
}

func (h *SharedLinkHandler) GetAllSharedLinks(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

type StackHandler struct {
	services *services.Services
}

func NewStackHandler(services *services.Services) *StackHandler {
	return &StackHandler{services: services}
}

func (h *StackHandler) GetStacks(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

type JobHandler struct {
	services *services.Services
}

func NewJobHandler(services *services.Services) *JobHandler {
	return &JobHandler{services: services}
}

func (h *JobHandler) GetAllJobsStatus(c *gin.Context) {
	respondWithData(c, gin.H{
		"isRunning": false,
		"queueStatus": gin.H{
			"isActive": false,
			"isPaused": false,
		},
		"jobCounts": gin.H{
			"active":    0,
			"completed": 0,
			"failed":    0,
			"delayed":   0,
			"waiting":   0,
			"paused":    0,
		},
	})
}

func (h *JobHandler) SendJobCommand(c *gin.Context) {
	respondWithSuccess(c, "Job command sent")
}

type SearchHandler struct {
	services *services.Services
}

func NewSearchHandler(services *services.Services) *SearchHandler {
	return &SearchHandler{services: services}
}

func (h *SearchHandler) Search(c *gin.Context) {
	respondWithData(c, gin.H{
		"albums": gin.H{
			"total": 0,
			"count": 0,
			"items": []interface{}{},
		},
		"assets": gin.H{
			"total": 0,
			"count": 0,
			"items": []interface{}{},
		},
	})
}

func (h *SearchHandler) GetExploreData(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

func (h *SearchHandler) SearchMetadata(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

func (h *SearchHandler) SearchSmart(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

type DownloadHandler struct {
	services *services.Services
}

func NewDownloadHandler(services *services.Services) *DownloadHandler {
	return &DownloadHandler{services: services}
}

func (h *DownloadHandler) DownloadFiles(c *gin.Context) {
	respondWithError(c, http.StatusNotImplemented, "Download not implemented")
}

func (h *DownloadHandler) GetDownloadInfo(c *gin.Context) {
	respondWithError(c, http.StatusNotImplemented, "Download info not implemented")
}

type ServerHandler struct {
	services *services.Services
}

func NewServerHandler(services *services.Services) *ServerHandler {
	return &ServerHandler{services: services}
}

func (h *ServerHandler) GetServerInfo(c *gin.Context) {
	respondWithData(c, h.services.Server.GetServerInfo())
}

func (h *ServerHandler) GetServerVersion(c *gin.Context) {
	respondWithData(c, h.services.Server.GetServerVersion())
}

func (h *ServerHandler) GetServerFeatures(c *gin.Context) {
	respondWithData(c, h.services.Server.GetServerFeatures())
}

func (h *ServerHandler) GetServerStatistics(c *gin.Context) {
	respondWithData(c, h.services.Server.GetServerStats())
}

func (h *ServerHandler) GetServerConfig(c *gin.Context) {
	respondWithData(c, h.services.Server.GetServerConfig())
}

func (h *ServerHandler) PingServer(c *gin.Context) {
	respondWithData(c, h.services.Server.PingServer())
}

type SystemConfigHandler struct {
	services *services.Services
}

func NewSystemConfigHandler(services *services.Services) *SystemConfigHandler {
	return &SystemConfigHandler{services: services}
}

func (h *SystemConfigHandler) GetConfig(c *gin.Context) {
	respondWithData(c, gin.H{
		"passwordLogin": gin.H{
			"enabled": true,
		},
		"oauth": gin.H{
			"enabled": false,
		},
	})
}

func (h *SystemConfigHandler) UpdateConfig(c *gin.Context) {
	respondWithSuccess(c, "Config updated")
}

type SystemMetadataHandler struct {
	services *services.Services
}

func NewSystemMetadataHandler(services *services.Services) *SystemMetadataHandler {
	return &SystemMetadataHandler{services: services}
}

func (h *SystemMetadataHandler) GetReverseGeocodingState(c *gin.Context) {
	respondWithData(c, gin.H{
		"lastUpdate": nil,
		"lastImport": nil,
	})
}

type OAuthHandler struct {
	services *services.Services
}

func NewOAuthHandler(services *services.Services) *OAuthHandler {
	return &OAuthHandler{services: services}
}

func (h *OAuthHandler) GenerateConfig(c *gin.Context) {
	respondWithError(c, http.StatusNotImplemented, "OAuth not implemented")
}

func (h *OAuthHandler) Callback(c *gin.Context) {
	respondWithError(c, http.StatusNotImplemented, "OAuth not implemented")
}

func (h *OAuthHandler) Link(c *gin.Context) {
	respondWithError(c, http.StatusNotImplemented, "OAuth not implemented")
}

func (h *OAuthHandler) Unlink(c *gin.Context) {
	respondWithError(c, http.StatusNotImplemented, "OAuth not implemented")
}

// Admin handlers
type AdminNotificationHandler struct {
	services *services.Services
}

func NewAdminNotificationHandler(services *services.Services) *AdminNotificationHandler {
	return &AdminNotificationHandler{services: services}
}

func (h *AdminNotificationHandler) CreateNotification(c *gin.Context) {
	respondWithSuccess(c, "Admin notification created")
}

func (h *AdminNotificationHandler) GetNotificationTemplate(c *gin.Context) {
	templateName := c.Param("name")
	respondWithData(c, gin.H{
		"template": templateName,
		"subject":  "Test notification",
		"body":     "This is a test notification template",
	})
}

func (h *AdminNotificationHandler) SendTestEmail(c *gin.Context) {
	respondWithSuccess(c, "Test email sent")
}

type AdminUserHandler struct {
	services *services.Services
}

func NewAdminUserHandler(services *services.Services) *AdminUserHandler {
	return &AdminUserHandler{services: services}
}

func (h *AdminUserHandler) SearchUsersAdmin(c *gin.Context) {
	includeDeleted := c.Query("withDeleted") == "true"
	users, err := h.services.User.GetAllUsers(includeDeleted)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithData(c, users)
}

func (h *AdminUserHandler) CreateUserAdmin(c *gin.Context) {
	var createReq services.CreateUserRequest

	if err := c.ShouldBindJSON(&createReq); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.services.User.CreateUser(createReq)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, user)
}

func (h *AdminUserHandler) GetUserAdmin(c *gin.Context) {
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

func (h *AdminUserHandler) UpdateUserAdmin(c *gin.Context) {
	userID := c.Param("id")
	id, err := uuid.Parse(userID)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var updateReq services.UpdateUserRequest

	if err := c.ShouldBindJSON(&updateReq); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.services.User.UpdateUser(id, updateReq)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, user)
}

func (h *AdminUserHandler) DeleteUserAdmin(c *gin.Context) {
	userID := c.Param("id")
	id, err := uuid.Parse(userID)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	err = h.services.User.DeleteUser(id)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithSuccess(c, "User deleted successfully")
}

func (h *AdminUserHandler) RestoreUserAdmin(c *gin.Context) {
	userID := c.Param("id")
	respondWithSuccess(c, "User "+userID+" restored successfully")
}

func (h *AdminUserHandler) GetUserPreferencesAdmin(c *gin.Context) {
	userID := c.Param("id")
	id, err := uuid.Parse(userID)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	prefs, err := h.services.User.GetUserPreferences(id)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithData(c, prefs)
}

func (h *AdminUserHandler) UpdateUserPreferencesAdmin(c *gin.Context) {
	userID := c.Param("id")
	id, err := uuid.Parse(userID)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var prefs map[string]interface{}
	if err := c.ShouldBindJSON(&prefs); err != nil {
		respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	_, err = h.services.User.UpdateUserPreferences(id, prefs)
	if err != nil {
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithSuccess(c, "User preferences updated")
}
