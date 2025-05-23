package handlers

import (
	"github.com/denysvitali/immich-go-backend/internal/services"
	"github.com/gin-gonic/gin"
)

type Handlers struct {
	services *services.Services
	Auth     *AuthHandler
	User     *UserHandler
	Album    *AlbumHandler
	Asset    *AssetHandler
	Library  *LibraryHandler
	APIKey   *APIKeyHandler
	Tag      *TagHandler
	Activity *ActivityHandler
	Notification *NotificationHandler
	Partner  *PartnerHandler
	Person   *PersonHandler
	SharedLink *SharedLinkHandler
	Stack    *StackHandler
	Job      *JobHandler
	Search   *SearchHandler
	Download *DownloadHandler
	Server   *ServerHandler
	SystemConfig *SystemConfigHandler
	SystemMetadata *SystemMetadataHandler
	OAuth    *OAuthHandler
}

func NewHandlers(services *services.Services) *Handlers {
	return &Handlers{
		services:       services,
		Auth:           NewAuthHandler(services),
		User:           NewUserHandler(services),
		Album:          NewAlbumHandler(services),
		Asset:          NewAssetHandler(services),
		Library:        NewLibraryHandler(services),
		APIKey:         NewAPIKeyHandler(services),
		Tag:            NewTagHandler(services),
		Activity:       NewActivityHandler(services),
		Notification:   NewNotificationHandler(services),
		Partner:        NewPartnerHandler(services),
		Person:         NewPersonHandler(services),
		SharedLink:     NewSharedLinkHandler(services),
		Stack:          NewStackHandler(services),
		Job:            NewJobHandler(services),
		Search:         NewSearchHandler(services),
		Download:       NewDownloadHandler(services),
		Server:         NewServerHandler(services),
		SystemConfig:   NewSystemConfigHandler(services),
		SystemMetadata: NewSystemMetadataHandler(services),
		OAuth:          NewOAuthHandler(services),
	}
}

// Common response helpers
func respondWithError(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{"error": message})
}

func respondWithData(c *gin.Context, data interface{}) {
	c.JSON(200, data)
}

func respondWithSuccess(c *gin.Context, message string) {
	c.JSON(200, gin.H{"message": message})
}
