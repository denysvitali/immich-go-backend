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

func (h *TagHandler) UpsertTags(c *gin.Context) {
	respondWithSuccess(c, "Tags upserted")
}

func (h *TagHandler) BulkTagAssets(c *gin.Context) {
	respondWithSuccess(c, "Assets tagged")
}

func (h *TagHandler) GetTagById(c *gin.Context) {
	id := c.Param("id")
	respondWithData(c, gin.H{
		"id":    id,
		"name":  "Sample Tag",
		"value": "sample-tag",
	})
}

func (h *TagHandler) UpdateTag(c *gin.Context) {
	respondWithSuccess(c, "Tag updated")
}

func (h *TagHandler) DeleteTag(c *gin.Context) {
	respondWithSuccess(c, "Tag deleted")
}

func (h *TagHandler) TagAssets(c *gin.Context) {
	respondWithSuccess(c, "Assets tagged")
}

func (h *TagHandler) UntagAssets(c *gin.Context) {
	respondWithSuccess(c, "Assets untagged")
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

func (h *ActivityHandler) GetActivityStatistics(c *gin.Context) {
	respondWithData(c, gin.H{
		"comments": 0,
	})
}

func (h *ActivityHandler) DeleteActivity(c *gin.Context) {
	respondWithSuccess(c, "Activity deleted")
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

func (h *NotificationHandler) DeleteNotifications(c *gin.Context) {
	respondWithSuccess(c, "Notifications deleted")
}

func (h *NotificationHandler) UpdateNotifications(c *gin.Context) {
	respondWithSuccess(c, "Notifications updated")
}

func (h *NotificationHandler) GetNotification(c *gin.Context) {
	id := c.Param("id")
	respondWithData(c, gin.H{
		"id":      id,
		"message": "Sample notification",
		"type":    "info",
	})
}

func (h *NotificationHandler) UpdateNotification(c *gin.Context) {
	respondWithSuccess(c, "Notification updated")
}

func (h *NotificationHandler) DeleteNotification(c *gin.Context) {
	respondWithSuccess(c, "Notification deleted")
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

func (h *PartnerHandler) CreatePartner(c *gin.Context) {
	respondWithSuccess(c, "Partner created")
}

func (h *PartnerHandler) UpdatePartner(c *gin.Context) {
	respondWithSuccess(c, "Partner updated")
}

func (h *PartnerHandler) RemovePartner(c *gin.Context) {
	respondWithSuccess(c, "Partner removed")
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

func (h *PersonHandler) CreatePerson(c *gin.Context) {
	respondWithSuccess(c, "Person created")
}

func (h *PersonHandler) UpdatePeople(c *gin.Context) {
	respondWithSuccess(c, "People updated")
}

func (h *PersonHandler) ReassignFaces(c *gin.Context) {
	respondWithSuccess(c, "Faces reassigned")
}

func (h *PersonHandler) GetPerson(c *gin.Context) {
	id := c.Param("id")
	respondWithData(c, gin.H{
		"id":            id,
		"name":          "Sample Person",
		"thumbnailPath": "",
	})
}

func (h *PersonHandler) UpdatePerson(c *gin.Context) {
	respondWithSuccess(c, "Person updated")
}

func (h *PersonHandler) MergePerson(c *gin.Context) {
	respondWithSuccess(c, "Person merged")
}

func (h *PersonHandler) GetPersonStatistics(c *gin.Context) {
	respondWithData(c, gin.H{
		"assets": 0,
	})
}

func (h *PersonHandler) CreatePersonThumbnail(c *gin.Context) {
	respondWithSuccess(c, "Person thumbnail created")
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

func (h *SharedLinkHandler) CreateSharedLink(c *gin.Context) {
	respondWithSuccess(c, "Shared link created")
}

func (h *SharedLinkHandler) GetMySharedLink(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

func (h *SharedLinkHandler) GetSharedLinkById(c *gin.Context) {
	id := c.Param("id")
	respondWithData(c, gin.H{
		"id":        id,
		"type":      "ALBUM",
		"expiresAt": nil,
	})
}

func (h *SharedLinkHandler) UpdateSharedLink(c *gin.Context) {
	respondWithSuccess(c, "Shared link updated")
}

func (h *SharedLinkHandler) RemoveSharedLink(c *gin.Context) {
	respondWithSuccess(c, "Shared link removed")
}

func (h *SharedLinkHandler) AddSharedLinkAssets(c *gin.Context) {
	respondWithSuccess(c, "Assets added to shared link")
}

func (h *SharedLinkHandler) RemoveSharedLinkAssets(c *gin.Context) {
	respondWithSuccess(c, "Assets removed from shared link")
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

func (h *StackHandler) SearchStacks(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

func (h *StackHandler) CreateStack(c *gin.Context) {
	respondWithSuccess(c, "Stack created")
}

func (h *StackHandler) DeleteStacks(c *gin.Context) {
	respondWithSuccess(c, "Stacks deleted")
}

func (h *StackHandler) GetStack(c *gin.Context) {
	id := c.Param("id")
	respondWithData(c, gin.H{
		"id":             id,
		"primaryAssetId": id,
		"assetCount":     1,
	})
}

func (h *StackHandler) UpdateStack(c *gin.Context) {
	respondWithSuccess(c, "Stack updated")
}

func (h *StackHandler) DeleteStack(c *gin.Context) {
	respondWithSuccess(c, "Stack deleted")
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

func (h *JobHandler) CreateJob(c *gin.Context) {
	respondWithSuccess(c, "Job created")
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

func (h *SearchHandler) SearchAssets(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

func (h *SearchHandler) SearchPerson(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

func (h *SearchHandler) SearchPlaces(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

func (h *SearchHandler) SearchRandom(c *gin.Context) {
	respondWithData(c, []interface{}{})
}

func (h *SearchHandler) GetSearchSuggestions(c *gin.Context) {
	respondWithData(c, []string{})
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

func (h *DownloadHandler) DownloadArchive(c *gin.Context) {
	respondWithError(c, http.StatusNotImplemented, "Download archive not implemented")
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

func (h *ServerHandler) Ping(c *gin.Context) {
	respondWithData(c, gin.H{
		"response": "pong",
	})
}

func (h *ServerHandler) GetConfig(c *gin.Context) {
	respondWithData(c, gin.H{
		"loginPageMessage": "",
		"oauthButtonText":  "Login with OAuth",
		"passwordLogin": gin.H{
			"enabled": true,
		},
		"externalDomain": "",
		"isInitialized":  true,
		"isOnboarded":    true,
	})
}

func (h *ServerHandler) GetFeatures(c *gin.Context) {
	respondWithData(c, gin.H{
		"smartSearch":       true,
		"facialRecognition": true,
		"sidecar":           true,
		"search":            true,
		"trash":             true,
		"oauth":             false,
		"oauthAutoLaunch":   false,
		"passwordLogin":     true,
		"configFile":        false,
	})
}

func (h *ServerHandler) GetAbout(c *gin.Context) {
	respondWithData(c, gin.H{
		"version":       "1.0.0",
		"versionUrl":    "https://github.com/immich-app/immich",
		"repository":    "https://github.com/immich-app/immich",
		"repositoryUrl": "https://github.com/immich-app/immich",
		"sourceRef":     "main",
		"sourceCommit":  "abcd1234",
		"sourceUrl":     "https://github.com/immich-app/immich",
		"build":         "1",
		"buildUrl":      "",
		"buildImage":    "",
		"buildImageUrl": "",
		"documentation": "https://immich.app/docs",
		"supportUrl":    "https://github.com/immich-app/immich/discussions",
	})
}

func (h *ServerHandler) GetMediaTypes(c *gin.Context) {
	respondWithData(c, gin.H{
		"image":   []string{"jpg", "jpeg", "png", "gif", "bmp", "webp", "tiff", "svg", "heic", "heif", "avif"},
		"video":   []string{"mp4", "mov", "avi", "mkv", "webm", "3gp", "m4v", "mpg", "mpeg", "wmv", "flv"},
		"sidecar": []string{"xmp"},
	})
}

func (h *ServerHandler) GetStatistics(c *gin.Context) {
	respondWithData(c, gin.H{
		"photos":      0,
		"videos":      0,
		"usage":       0,
		"usageByUser": []interface{}{},
	})
}

func (h *ServerHandler) GetStorage(c *gin.Context) {
	respondWithData(c, gin.H{
		"diskSizeRaw":        1000000000000,
		"diskUseRaw":         500000000000,
		"diskAvailableRaw":   500000000000,
		"diskUsagePercentage": 50.0,
	})
}

func (h *ServerHandler) GetTheme(c *gin.Context) {
	respondWithData(c, gin.H{
		"customCss": "",
	})
}

func (h *ServerHandler) GetVersion(c *gin.Context) {
	respondWithData(c, gin.H{
		"major": 1,
		"minor": 0,
		"patch": 0,
	})
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

func (h *SystemConfigHandler) GetConfigDefaults(c *gin.Context) {
	respondWithData(c, gin.H{
		"ffmpeg": gin.H{
			"crf":                23,
			"threads":            0,
			"preset":             "ultrafast",
			"targetVideoCodec":   "h264",
			"acceptedVideoCodecs": []string{"h264"},
			"targetAudioCodec":   "aac",
			"acceptedAudioCodecs": []string{"aac", "mp3", "libopus"},
			"targetResolution":   "720",
			"maxBitrate":         "0",
			"bframes":            -1,
			"refs":               0,
			"gopSize":            0,
			"npl":                0,
			"temporalAQ":         false,
			"cqMode":             "auto",
			"twoPass":            false,
			"preferredHwDevice":  "auto",
			"transcode":          "required",
			"tonemap":            "hable",
			"accel":              "disabled",
		},
	})
}

func (h *SystemConfigHandler) GetStorageTemplateOptions(c *gin.Context) {
	respondWithData(c, gin.H{
		"dayOptions":    []string{"d", "dd"},
		"monthOptions":  []string{"M", "MM", "MMM", "MMMM"},
		"yearOptions":   []string{"y", "yy", "yyyy"},
		"hourOptions":   []string{"h", "hh", "H", "HH"},
		"minuteOptions": []string{"m", "mm"},
		"secondOptions": []string{"s", "ss"},
		"presetOptions": []string{
			"{{y}}/{{y}}-{{MM}}-{{dd}}/{{filename}}",
			"{{y}}/{{MM}}-{{dd}}/{{filename}}",
			"{{y}}/{{MMMM}}/{{filename}}",
			"{{y}}/{{MM}}/{{filename}}",
		},
	})
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

func (h *SystemMetadataHandler) GetAdminOnboarding(c *gin.Context) {
	respondWithData(c, gin.H{
		"isOnboarded": true,
	})
}

func (h *SystemMetadataHandler) UpdateAdminOnboarding(c *gin.Context) {
	respondWithSuccess(c, "Admin onboarding updated")
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

func (h *OAuthHandler) StartOAuth(c *gin.Context) {
	respondWithError(c, http.StatusNotImplemented, "OAuth not implemented")
}

func (h *OAuthHandler) FinishOAuth(c *gin.Context) {
	respondWithError(c, http.StatusNotImplemented, "OAuth not implemented")
}

func (h *OAuthHandler) RedirectOAuthToMobile(c *gin.Context) {
	respondWithError(c, http.StatusNotImplemented, "OAuth not implemented")
}

func (h *OAuthHandler) LinkOAuthAccount(c *gin.Context) {
	respondWithError(c, http.StatusNotImplemented, "OAuth not implemented")
}

func (h *OAuthHandler) UnlinkOAuthAccount(c *gin.Context) {
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
