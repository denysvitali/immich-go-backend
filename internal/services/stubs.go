package services

import (
	"gorm.io/gorm"
)

// Stub services for completeness - these can be expanded later

type TagService struct {
	db *gorm.DB
}

func NewTagService(db *gorm.DB) *TagService {
	return &TagService{db: db}
}

type ActivityService struct {
	db *gorm.DB
}

func NewActivityService(db *gorm.DB) *ActivityService {
	return &ActivityService{db: db}
}

type NotificationService struct {
	db *gorm.DB
}

func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{db: db}
}

type PartnerService struct {
	db *gorm.DB
}

func NewPartnerService(db *gorm.DB) *PartnerService {
	return &PartnerService{db: db}
}

type PersonService struct {
	db *gorm.DB
}

func NewPersonService(db *gorm.DB) *PersonService {
	return &PersonService{db: db}
}

type SharedLinkService struct {
	db *gorm.DB
}

func NewSharedLinkService(db *gorm.DB) *SharedLinkService {
	return &SharedLinkService{db: db}
}

type StackService struct {
	db *gorm.DB
}

func NewStackService(db *gorm.DB) *StackService {
	return &StackService{db: db}
}

type JobService struct {
	db *gorm.DB
}

func NewJobService(db *gorm.DB) *JobService {
	return &JobService{db: db}
}

type SearchService struct {
	db *gorm.DB
}

func NewSearchService(db *gorm.DB) *SearchService {
	return &SearchService{db: db}
}

type DownloadService struct {
	db *gorm.DB
}

func NewDownloadService(db *gorm.DB) *DownloadService {
	return &DownloadService{db: db}
}

type ServerService struct{}

func NewServerService() *ServerService {
	return &ServerService{}
}

type SystemConfigService struct {
	db *gorm.DB
}

func NewSystemConfigService(db *gorm.DB) *SystemConfigService {
	return &SystemConfigService{db: db}
}

type SystemMetadataService struct {
	db *gorm.DB
}

func NewSystemMetadataService(db *gorm.DB) *SystemMetadataService {
	return &SystemMetadataService{db: db}
}

type OAuthService struct {
	db *gorm.DB
}

func NewOAuthService(db *gorm.DB) *OAuthService {
	return &OAuthService{db: db}
}

// Server info and configuration methods
func (s *ServerService) GetServerInfo() map[string]interface{} {
	return map[string]interface{}{
		"version": "1.0.0",
		"name":    "Immich Go Backend",
	}
}

func (s *ServerService) GetServerVersion() map[string]interface{} {
	return map[string]interface{}{
		"major": 1,
		"minor": 0,
		"patch": 0,
	}
}

func (s *ServerService) GetServerFeatures() map[string]interface{} {
	return map[string]interface{}{
		"facialRecognition": false,
		"map":               false,
		"reverseGeocoding":  false,
		"oauth":             false,
		"passwordLogin":     true,
		"search":            false,
		"sidecar":           false,
		"configFile":        false,
		"trash":             true,
	}
}

func (s *ServerService) GetServerStats() map[string]interface{} {
	return map[string]interface{}{
		"photos":      0,
		"videos":      0,
		"usage":       0,
		"usageByUser": []interface{}{},
	}
}

func (s *ServerService) GetServerConfig() map[string]interface{} {
	return map[string]interface{}{
		"loginPageMessage": "",
		"oauthButtonText":  "Login with OAuth",
		"mapTileUrl":       "",
		"trashDays":        30,
		"userDeleteDelay":  7,
		"isInitialized":    true,
		"isOnboarded":      true,
	}
}

func (s *ServerService) PingServer() map[string]interface{} {
	return map[string]interface{}{
		"res": "pong",
	}
}
