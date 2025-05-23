package server

import (
	"context"
	"net"
	"net/http"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/handlers"
	"github.com/denysvitali/immich-go-backend/internal/middleware"
	"github.com/denysvitali/immich-go-backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

type Server struct {
	config      *config.Config
	db          *gorm.DB
	grpcServer  *grpc.Server
	authService *auth.Service
	services    *services.Services
	handlers    *handlers.Handlers
}

func NewServer(cfg *config.Config, db *gorm.DB) *Server {
	authService := auth.NewService(cfg.JWT.SecretKey)
	servicesContainer := services.NewServices(db, authService)
	handlersContainer := handlers.NewHandlers(servicesContainer)

	grpcServer := grpc.NewServer()

	return &Server{
		config:      cfg,
		db:          db,
		grpcServer:  grpcServer,
		authService: authService,
		services:    servicesContainer,
		handlers:    handlersContainer,
	}
}

func (s *Server) ServeGRPC(listener net.Listener) error {
	logrus.Info("gRPC server starting...")
	return s.grpcServer.Serve(listener)
}

func (s *Server) HTTPHandler() http.Handler {
	if s.config.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Middleware
	r.Use(middleware.CORS())
	r.Use(middleware.LoggingMiddleware())
	r.Use(gin.Recovery())

	authMiddleware := middleware.NewAuthMiddleware(s.authService)

	// API routes
	api := r.Group("/api")
	{
		// Public routes (no authentication required)
		public := api.Group("")
		{
			// Authentication endpoints
			auth := public.Group("/auth")
			{
				auth.POST("/login", s.handlers.Auth.Login)
				auth.POST("/admin-sign-up", s.handlers.Auth.AdminSignUp)
				auth.POST("/change-password", authMiddleware.RequireAuth(), s.handlers.Auth.ChangePassword)
				auth.POST("/logout", authMiddleware.RequireAuth(), s.handlers.Auth.Logout)
				auth.POST("/validateToken", authMiddleware.RequireAuth(), s.handlers.Auth.ValidateToken)
			}

			// Server info endpoints
			server := public.Group("/server")
			{
				server.GET("/ping", s.handlers.Server.Ping)
				server.GET("/config", s.handlers.Server.GetConfig)
				server.GET("/features", s.handlers.Server.GetFeatures)
				server.GET("/about", authMiddleware.RequireAuth(), s.handlers.Server.GetAbout)
				server.GET("/media-types", s.handlers.Server.GetMediaTypes)
				server.GET("/statistics", authMiddleware.RequireAuth(), s.handlers.Server.GetStatistics)
				server.GET("/storage", authMiddleware.RequireAuth(), s.handlers.Server.GetStorage)
				server.GET("/theme", authMiddleware.RequireAuth(), s.handlers.Server.GetTheme)
				server.GET("/version", authMiddleware.RequireAuth(), s.handlers.Server.GetVersion)
			}

			// OAuth endpoints
			oauth := public.Group("/oauth")
			{
				oauth.POST("/authorize", s.handlers.OAuth.StartOAuth)
				oauth.POST("/callback", s.handlers.OAuth.FinishOAuth)
				oauth.GET("/mobile-redirect", s.handlers.OAuth.RedirectOAuthToMobile)
				oauth.POST("/link", authMiddleware.RequireAuth(), s.handlers.OAuth.LinkOAuthAccount)
				oauth.POST("/unlink", authMiddleware.RequireAuth(), s.handlers.OAuth.UnlinkOAuthAccount)
			}
		}

		// Protected routes (authentication required)
		protected := api.Group("", authMiddleware.RequireAuth())
		{
			// User management
			users := protected.Group("/users")
			{
				users.GET("", s.handlers.User.SearchUsers)
				users.GET("/me", s.handlers.User.GetMyUser)
				users.PUT("/me", s.handlers.User.UpdateMyUser)
				users.GET("/me/preferences", s.handlers.User.GetMyPreferences)
				users.PUT("/me/preferences", s.handlers.User.UpdateMyPreferences)
				users.PUT("/me/license", s.handlers.User.SetMyLicense)
				users.GET("/:id", s.handlers.User.GetUser)
				users.GET("/:id/profile-image", s.handlers.User.GetProfileImage)
			}

			// API Keys
			apiKeys := protected.Group("/api-keys")
			{
				apiKeys.GET("", s.handlers.APIKey.GetAPIKeys)
				apiKeys.POST("", s.handlers.APIKey.CreateAPIKey)
				apiKeys.GET("/:id", s.handlers.APIKey.GetAPIKey)
				apiKeys.PUT("/:id", s.handlers.APIKey.UpdateAPIKey)
				apiKeys.DELETE("/:id", s.handlers.APIKey.DeleteAPIKey)
			}

			// Albums
			albums := protected.Group("/albums")
			{
				albums.GET("", s.handlers.Album.GetAllAlbums)
				albums.POST("", s.handlers.Album.CreateAlbum)
				albums.GET("/:id", s.handlers.Album.GetAlbumInfo)
				albums.PATCH("/:id", s.handlers.Album.UpdateAlbumInfo)
				albums.DELETE("/:id", s.handlers.Album.DeleteAlbum)
				albums.PUT("/:id/assets", s.handlers.Album.AddAssetsToAlbum)
				albums.DELETE("/:id/assets", s.handlers.Album.RemoveAssetFromAlbum)
				albums.PUT("/:id/users", s.handlers.Album.AddUsersToAlbum)
				albums.DELETE("/:id/users/:userId", s.handlers.Album.RemoveUserFromAlbum)
			}

			// Assets
			assets := protected.Group("/assets")
			{
				assets.POST("", s.handlers.Asset.UploadAsset)
				assets.GET("", s.handlers.Asset.GetAllAssets)
				assets.PUT("", s.handlers.Asset.UpdateAssets)
				assets.DELETE("", s.handlers.Asset.DeleteAssets)
				assets.POST("/bulk-upload-check", s.handlers.Asset.CheckBulkUpload)
				assets.POST("/exist", s.handlers.Asset.CheckExistingAssets)
				assets.POST("/jobs", s.handlers.Asset.RunAssetJobs)
				assets.GET("/random", s.handlers.Asset.GetRandom)
				assets.GET("/statistics", s.handlers.Asset.GetAssetStatistics)
				assets.GET("/device/:deviceId", s.handlers.Asset.GetAllUserAssetsByDeviceId)
				assets.GET("/:id", s.handlers.Asset.GetAssetInfo)
				assets.PUT("/:id", s.handlers.Asset.UpdateAsset)
				assets.GET("/:id/original", s.handlers.Asset.DownloadAsset)
				assets.PUT("/:id/original", s.handlers.Asset.ReplaceAsset)
				assets.GET("/:id/thumbnail", s.handlers.Asset.ServeAssetThumbnail)
				assets.GET("/:id/video/playback", s.handlers.Asset.PlayAssetVideo)
			}

			// Libraries
			libraries := protected.Group("/libraries")
			{
				libraries.GET("", s.handlers.Library.GetAllLibraries)
				libraries.POST("", s.handlers.Library.CreateLibrary)
				libraries.GET("/:id", s.handlers.Library.GetLibrary)
				libraries.PUT("/:id", s.handlers.Library.UpdateLibrary)
				libraries.DELETE("/:id", s.handlers.Library.DeleteLibrary)
				libraries.POST("/:id/scan", s.handlers.Library.ScanLibrary)
				libraries.GET("/:id/statistics", s.handlers.Library.GetLibraryStatistics)
				libraries.POST("/:id/validate", s.handlers.Library.ValidateLibrary)
			}

			// Search
			search := protected.Group("/search")
			{
				search.GET("/explore", s.handlers.Search.GetExploreData)
				search.POST("/metadata", s.handlers.Search.SearchAssets)
				search.GET("/person", s.handlers.Search.SearchPerson)
				search.GET("/places", s.handlers.Search.SearchPlaces)
				search.POST("/random", s.handlers.Search.SearchRandom)
				search.POST("/smart", s.handlers.Search.SearchSmart)
				search.GET("/suggestions", s.handlers.Search.GetSearchSuggestions)
			}

			// Tags
			tags := protected.Group("/tags")
			{
				tags.GET("", s.handlers.Tag.GetAllTags)
				tags.POST("", s.handlers.Tag.CreateTag)
				tags.PUT("", s.handlers.Tag.UpsertTags)
				tags.PUT("/assets", s.handlers.Tag.BulkTagAssets)
				tags.GET("/:id", s.handlers.Tag.GetTagById)
				tags.PUT("/:id", s.handlers.Tag.UpdateTag)
				tags.DELETE("/:id", s.handlers.Tag.DeleteTag)
				tags.PUT("/:id/assets", s.handlers.Tag.TagAssets)
				tags.DELETE("/:id/assets", s.handlers.Tag.UntagAssets)
			}

			// Activities
			activities := protected.Group("/activities")
			{
				activities.GET("", s.handlers.Activity.GetActivities)
				activities.POST("", s.handlers.Activity.CreateActivity)
				activities.GET("/statistics", s.handlers.Activity.GetActivityStatistics)
				activities.DELETE("/:id", s.handlers.Activity.DeleteActivity)
			}

			// Notifications
			notifications := protected.Group("/notifications")
			{
				notifications.GET("", s.handlers.Notification.GetNotifications)
				notifications.DELETE("", s.handlers.Notification.DeleteNotifications)
				notifications.PUT("", s.handlers.Notification.UpdateNotifications)
				notifications.GET("/:id", s.handlers.Notification.GetNotification)
				notifications.PUT("/:id", s.handlers.Notification.UpdateNotification)
				notifications.DELETE("/:id", s.handlers.Notification.DeleteNotification)
			}

			// Partners
			partners := protected.Group("/partners")
			{
				partners.GET("", s.handlers.Partner.GetPartners)
				partners.POST("/:id", s.handlers.Partner.CreatePartner)
				partners.PUT("/:id", s.handlers.Partner.UpdatePartner)
				partners.DELETE("/:id", s.handlers.Partner.RemovePartner)
			}

			// People
			people := protected.Group("/people")
			{
				people.GET("", s.handlers.Person.GetAllPeople)
				people.POST("", s.handlers.Person.CreatePerson)
				people.PUT("", s.handlers.Person.UpdatePeople)
				people.PUT("/reassign", s.handlers.Person.ReassignFaces)
				people.GET("/:id", s.handlers.Person.GetPerson)
				people.PUT("/:id", s.handlers.Person.UpdatePerson)
				people.GET("/:id/merge", s.handlers.Person.MergePerson)
				people.POST("/:id/merge", s.handlers.Person.MergePerson)
				people.GET("/:id/statistics", s.handlers.Person.GetPersonStatistics)
				people.POST("/:id/thumbnail", s.handlers.Person.CreatePersonThumbnail)
			}

			// Shared Links
			sharedLinks := protected.Group("/shared-links")
			{
				sharedLinks.GET("", s.handlers.SharedLink.GetAllSharedLinks)
				sharedLinks.POST("", s.handlers.SharedLink.CreateSharedLink)
				sharedLinks.GET("/me", s.handlers.SharedLink.GetMySharedLink)
				sharedLinks.GET("/:id", s.handlers.SharedLink.GetSharedLinkById)
				sharedLinks.PATCH("/:id", s.handlers.SharedLink.UpdateSharedLink)
				sharedLinks.DELETE("/:id", s.handlers.SharedLink.RemoveSharedLink)
				sharedLinks.PUT("/:id/assets", s.handlers.SharedLink.AddSharedLinkAssets)
				sharedLinks.DELETE("/:id/assets", s.handlers.SharedLink.RemoveSharedLinkAssets)
			}

			// Stacks
			stacks := protected.Group("/stacks")
			{
				stacks.GET("", s.handlers.Stack.SearchStacks)
				stacks.POST("", s.handlers.Stack.CreateStack)
				stacks.DELETE("", s.handlers.Stack.DeleteStacks)
				stacks.GET("/:id", s.handlers.Stack.GetStack)
				stacks.PUT("/:id", s.handlers.Stack.UpdateStack)
				stacks.DELETE("/:id", s.handlers.Stack.DeleteStack)
			}

			// Jobs
			jobs := protected.Group("/jobs")
			{
				jobs.GET("", s.handlers.Job.GetAllJobsStatus)
				jobs.POST("", s.handlers.Job.CreateJob)
				jobs.PUT("/:id", s.handlers.Job.SendJobCommand)
			}

			// Downloads
			download := protected.Group("/download")
			{
				download.POST("/archive", s.handlers.Download.DownloadArchive)
				download.POST("/info", s.handlers.Download.GetDownloadInfo)
			}

			// System Config
			systemConfig := protected.Group("/system-config")
			{
				systemConfig.GET("", s.handlers.SystemConfig.GetConfig)
				systemConfig.PUT("", s.handlers.SystemConfig.UpdateConfig)
				systemConfig.GET("/defaults", s.handlers.SystemConfig.GetConfigDefaults)
				systemConfig.GET("/storage-template-options", s.handlers.SystemConfig.GetStorageTemplateOptions)
			}

			// System Metadata
			systemMetadata := protected.Group("/system-metadata")
			{
				systemMetadata.GET("/admin-onboarding", s.handlers.SystemMetadata.GetAdminOnboarding)
				systemMetadata.POST("/admin-onboarding", s.handlers.SystemMetadata.UpdateAdminOnboarding)
				systemMetadata.GET("/reverse-geocoding-state", s.handlers.SystemMetadata.GetReverseGeocodingState)
			}
		}

		// Admin routes (admin authentication required)
		admin := api.Group("/admin", authMiddleware.RequireAuth(), authMiddleware.RequireAdmin())
		{
			// Admin notifications
			adminNotifications := admin.Group("/notifications")
			{
				adminNotifications.POST("", s.handlers.AdminNotification.CreateNotification)
				adminNotifications.POST("/templates/:name", s.handlers.AdminNotification.GetNotificationTemplate)
				adminNotifications.POST("/test-email", s.handlers.AdminNotification.SendTestEmail)
			}

			// Admin users
			adminUsers := admin.Group("/users")
			{
				adminUsers.GET("", s.handlers.AdminUser.SearchUsersAdmin)
				adminUsers.POST("", s.handlers.AdminUser.CreateUserAdmin)
				adminUsers.GET("/:id", s.handlers.AdminUser.GetUserAdmin)
				adminUsers.PUT("/:id", s.handlers.AdminUser.UpdateUserAdmin)
				adminUsers.DELETE("/:id", s.handlers.AdminUser.DeleteUserAdmin)
				adminUsers.POST("/:id/restore", s.handlers.AdminUser.RestoreUserAdmin)
				adminUsers.GET("/:id/preferences", s.handlers.AdminUser.GetUserPreferencesAdmin)
				adminUsers.PUT("/:id/preferences", s.handlers.AdminUser.UpdateUserPreferencesAdmin)
			}
		}
	}

	return r
}

func (s *Server) Stop() {
	logrus.Info("Stopping gRPC server...")
	s.grpcServer.GracefulStop()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}
