package services

import (
	"github.com/denysvitali/immich-go-backend/internal/auth"
	"gorm.io/gorm"
)

type Services struct {
	Auth            *AuthService
	User            *UserService
	Album           *AlbumService
	Asset           *AssetService
	Library         *LibraryService
	APIKey          *APIKeyService
	Tag             *TagService
	Activity        *ActivityService
	Notification    *NotificationService
	Partner         *PartnerService
	Person          *PersonService
	SharedLink      *SharedLinkService
	Stack           *StackService
	Job             *JobService
	Search          *SearchService
	Download        *DownloadService
	Server          *ServerService
	SystemConfig    *SystemConfigService
	SystemMetadata  *SystemMetadataService
	OAuth           *OAuthService
}

func NewServices(db *gorm.DB, authService *auth.Service) *Services {
	return &Services{
		Auth:            NewAuthService(db, authService),
		User:            NewUserService(db),
		Album:           NewAlbumService(db),
		Asset:           NewAssetService(db),
		Library:         NewLibraryService(db),
		APIKey:          NewAPIKeyService(db),
		Tag:             NewTagService(db),
		Activity:        NewActivityService(db),
		Notification:    NewNotificationService(db),
		Partner:         NewPartnerService(db),
		Person:          NewPersonService(db),
		SharedLink:      NewSharedLinkService(db),
		Stack:           NewStackService(db),
		Job:             NewJobService(db),
		Search:          NewSearchService(db),
		Download:        NewDownloadService(db),
		Server:          NewServerService(),
		SystemConfig:    NewSystemConfigService(db),
		SystemMetadata:  NewSystemMetadataService(db),
		OAuth:           NewOAuthService(db),
	}
}
