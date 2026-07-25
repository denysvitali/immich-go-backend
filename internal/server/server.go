package server

import (
	"context"
	"errors"
	"net"

	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/denysvitali/immich-go-backend/internal/activity"
	"github.com/denysvitali/immich-go-backend/internal/admin"
	"github.com/denysvitali/immich-go-backend/internal/apikeys"
	"github.com/denysvitali/immich-go-backend/internal/assets"
	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/download"
	"github.com/denysvitali/immich-go-backend/internal/duplicates"
	"github.com/denysvitali/immich-go-backend/internal/faces"
	"github.com/denysvitali/immich-go-backend/internal/jobs"
	"github.com/denysvitali/immich-go-backend/internal/libraries"
	"github.com/denysvitali/immich-go-backend/internal/maintenance"
	mapservice "github.com/denysvitali/immich-go-backend/internal/map"
	"github.com/denysvitali/immich-go-backend/internal/memories"
	"github.com/denysvitali/immich-go-backend/internal/ml"
	"github.com/denysvitali/immich-go-backend/internal/notifications"
	"github.com/denysvitali/immich-go-backend/internal/partners"
	"github.com/denysvitali/immich-go-backend/internal/people"
	"github.com/denysvitali/immich-go-backend/internal/plugin"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/queue"
	"github.com/denysvitali/immich-go-backend/internal/search"
	"github.com/denysvitali/immich-go-backend/internal/sessions"
	"github.com/denysvitali/immich-go-backend/internal/sharedlinks"
	"github.com/denysvitali/immich-go-backend/internal/stacks"
	"github.com/denysvitali/immich-go-backend/internal/storage"
	"github.com/denysvitali/immich-go-backend/internal/sync"
	"github.com/denysvitali/immich-go-backend/internal/systemconfig"
	"github.com/denysvitali/immich-go-backend/internal/systemmetadata"
	"github.com/denysvitali/immich-go-backend/internal/tags"
	"github.com/denysvitali/immich-go-backend/internal/timeline"
	"github.com/denysvitali/immich-go-backend/internal/trash"
	"github.com/denysvitali/immich-go-backend/internal/users"
	"github.com/denysvitali/immich-go-backend/internal/view"
	"github.com/denysvitali/immich-go-backend/internal/websocket"
	"github.com/denysvitali/immich-go-backend/internal/workflow"
)

var (
	// All these fields are set at build time using ldflags
	Version      = "dev"
	SourceCommit = "unknown"
	SourceRef    = "unknown"
	SourceUrl    = "unknown"
)

type Server struct {
	config      *config.Config
	db          *db.Conn
	grpcServer  *grpc.Server
	authService *auth.Service
	wsHub       *websocket.Hub

	// Service layer
	userService           *users.Service
	assetService          *assets.Service
	apiKeyService         *apikeys.Service
	librariesServer       *libraries.Server
	searchServer          *search.Server
	downloadService       *download.Service
	sharedLinksService    *sharedlinks.Service
	systemConfigService   *systemconfig.Service
	jobService            *jobs.Service
	trashService          *trash.Server
	tagsService           immichv1.TagsServiceServer
	mapService            *mapservice.Server
	peopleService         *people.Server
	partnersService       *partners.Server
	activityService       *activity.Server
	adminServer           *admin.Server
	duplicatesServer      *duplicates.Server
	facesServer           *faces.Server
	stacksServer          *stacks.Server
	systemMetadataService *systemmetadata.Service
	systemMetadataServer  *systemmetadata.Server
	viewServer            *view.Server
	sessionsService       *sessions.Service
	sessionsServer        *sessions.Server
	syncServer            *sync.Server
	memoriesServer        *memories.Server
	notificationsServer   *notifications.Server
	timelineService       *timeline.Service
	timelineServer        *timeline.Server
	maintenanceService    *maintenance.Service
	queueService          *queue.Service
	pluginService         *plugin.Service
	workflowService       *workflow.Service
	queries               *sqlc.Queries
	grpcClientConn        *grpc.ClientConn

	immichv1.UnimplementedAlbumServiceServer
	immichv1.UnimplementedApiKeyServiceServer
	immichv1.UnimplementedAssetServiceServer
	immichv1.UnimplementedAuthServiceServer
	immichv1.UnimplementedDownloadServiceServer
	immichv1.UnimplementedJobServiceServer
	// immichv1.UnimplementedLibrariesServiceServer // TODO: Re-enable when proto is updated
	// immichv1.UnimplementedMemoryServiceServer // Now implemented
	// immichv1.UnimplementedNotificationsServiceServer // Now implemented
	immichv1.UnimplementedOAuthServiceServer
	// immichv1.UnimplementedSearchServiceServer // TODO: Re-enable when proto is updated
	immichv1.UnimplementedServerServiceServer
	immichv1.UnimplementedSharedLinksServiceServer
	immichv1.UnimplementedSystemConfigServiceServer
	// immichv1.UnimplementedTimelineServiceServer // Now implemented
	immichv1.UnimplementedUsersServiceServer
	immichv1.UnimplementedTrashServiceServer
	immichv1.UnimplementedTagsServiceServer
	immichv1.UnimplementedMapServiceServer
	immichv1.UnimplementedPeopleServiceServer
	immichv1.UnimplementedPartnersServiceServer
	immichv1.UnimplementedActivityServiceServer
	immichv1.UnimplementedAdminServiceServer
	immichv1.UnimplementedDuplicatesServiceServer
	immichv1.UnimplementedFacesServiceServer
	immichv1.UnimplementedStacksServiceServer
	immichv1.UnimplementedSystemMetadataServiceServer
	immichv1.UnimplementedViewServiceServer
	immichv1.UnimplementedSessionsServiceServer
	immichv1.UnimplementedSyncServiceServer
	immichv1.UnimplementedMaintenanceServiceServer
	immichv1.UnimplementedQueueServiceServer
	immichv1.UnimplementedPluginServiceServer
	immichv1.UnimplementedWorkflowServiceServer
}

func NewServer(cfg *config.Config, db *db.Conn) (*Server, error) {
	authService := auth.NewService(cfg.Auth, db.Queries)
	wsHub := websocket.New()

	// Start the websocket hub
	go wsHub.Run()

	// Initialize services
	userService, err := users.NewService(db.Queries, cfg)
	if err != nil {
		return nil, err
	}

	// Initialize storage service
	storageService, err := storage.NewService(cfg.Storage)
	if err != nil {
		return nil, err
	}

	// Initialize Sync service early so it can be used by other services
	logger := logrus.StandardLogger()
	syncService := sync.NewService(db.Queries, logger)

	assetService, err := assets.NewService(db.Queries, storageService, cfg, syncService)
	if err != nil {
		return nil, err
	}

	// Machine learning client (disabled unless feature flags + URL set).
	mlClient := ml.NewClient(ml.Config{
		Enabled:              cfg.MLActive(),
		URL:                  cfg.MachineLearning.URL,
		Timeout:              cfg.MachineLearning.Timeout,
		CLIPModel:            cfg.MachineLearning.Clip.ModelName,
		FaceModel:            cfg.MachineLearning.FacialRecognition.ModelName,
		FaceMinScore:         cfg.MachineLearning.FacialRecognition.MinScore,
		FaceMaxDistance:      cfg.MachineLearning.FacialRecognition.MaxDistance,
		CLIPMaxDistance:      cfg.MachineLearning.Clip.MaxDistance,
		DuplicateMaxDistance: cfg.MachineLearning.DuplicateDetection.MaxDistance,
	})
	if mlClient.Enabled() {
		logrus.WithField("url", cfg.MachineLearning.URL).Info("Machine learning client enabled")
	} else {
		logrus.Info("Machine learning client disabled (enable Features.MachineLearningEnabled + machine_learning)")
	}

	apiKeyService := apikeys.NewService(db.Queries)
	libraryService := libraries.NewService(db.Queries, cfg, storageService)
	searchService := search.NewService(db.Queries, mlClient, cfg)
	downloadService := download.NewService(db.Queries, storageService)
	sharedLinksService := sharedlinks.NewService(db.Queries)
	systemConfigService := systemconfig.NewService(db.Queries)
	trashService := trash.NewServer(db.Queries)
	tagsService := tags.NewServer(db.Queries)
	mapService := mapservice.NewServer(db.Queries)
	peopleService := people.NewServer(db.Queries, storageService)
	partnersService := partners.NewServer(db.Queries)
	activityService := activity.NewServer(db.Queries)

	duplicatesService, err := duplicates.NewService(db.Queries, cfg)
	if err != nil {
		return nil, err
	}
	duplicatesServer := duplicates.NewServer(duplicatesService)

	facesService, err := faces.NewService(db.Queries, cfg)
	if err != nil {
		return nil, err
	}
	facesServer := faces.NewServer(facesService)

	stacksService, err := stacks.NewService(db.Queries, cfg)
	if err != nil {
		return nil, err
	}
	stacksServer := stacks.NewServer(stacksService)

	systemMetadataService, err := systemmetadata.NewService(db.Queries, cfg)
	if err != nil {
		return nil, err
	}
	systemMetadataServer := systemmetadata.NewServer(systemMetadataService)

	viewService, err := view.NewService(db.Queries, cfg)
	if err != nil {
		return nil, err
	}
	viewServer := view.NewServer(viewService)

	// Initialize Sessions service
	sessionsService := sessions.NewService(db.Queries, authService, logger)
	sessionsServer := sessions.NewServer(sessionsService)

	// Create sync server
	syncServer := sync.NewServer(syncService)

	// Initialize Memories service
	memoriesService := memories.NewService(db.Queries)
	memoriesServer := memories.NewServer(memoriesService)

	// Initialize Notifications service
	notificationsService := notifications.NewService(db.Queries)
	notificationsServer := notifications.NewServer(notificationsService)

	// Initialize Timeline service
	timelineService := timeline.NewService(db.Queries)
	timelineServer := timeline.NewServer(timelineService)

	// Initialize Maintenance service
	maintenanceService, err := maintenance.NewService(db.Queries, cfg)
	if err != nil {
		return nil, err
	}

	// Initialize Queue service
	queueService, err := queue.NewService(db.Queries, cfg)
	if err != nil {
		return nil, err
	}

	// Initialize Plugin service
	pluginService, err := plugin.NewService(db.Queries, cfg)
	if err != nil {
		return nil, err
	}

	// Initialize Workflow service
	workflowService, err := workflow.NewService(db.Queries, cfg)
	if err != nil {
		return nil, err
	}

	// Create server implementations for libraries and search
	libraryServer := libraries.NewServer(libraryService)
	searchServer := search.NewServer(searchService)

	// Initialize job service (requires Redis configuration)
	var jobService *jobs.Service
	if cfg.Jobs.Enabled && cfg.Jobs.RedisURL != "" {
		// Parse Redis URL to get components
		redisAddr, redisPassword, redisDB := parseRedisURL(cfg.Jobs.RedisURL)
		jobCfg := &jobs.Config{
			RedisAddr:     redisAddr,
			RedisPassword: redisPassword,
			RedisDB:       redisDB,
			Concurrency:   cfg.Jobs.Workers,
			QueueName:     "immich",
			MaxRetries:    cfg.Jobs.RetryMaxRetries,
			DB:            db.Queries,
		}
		var err error
		jobService, err = jobs.NewService(jobCfg)
		if err != nil {
			logrus.WithError(err).Warn("Failed to initialize job service, background processing disabled")
		} else {
			// Register real job handlers so enqueued jobs are processed
			jobHandlers := jobs.NewHandlers(db.Queries, assetService, libraryService, storageService, mlClient, cfg)
			jobHandlers.RegisterAllHandlers(jobService)
			// Start the asynq worker server; without this, enqueued jobs
			// (thumbnails, metadata extraction, transcodes) sit in Redis
			// forever.
			if err := jobService.Start(); err != nil {
				logrus.WithError(err).Warn("Failed to start job workers, background processing disabled")
				jobService = nil
			}
		}
	} else {
		logrus.Warn("Job service not configured, background processing disabled")
	}

	// Initialize admin service (depends on job service for dead-letter ops)
	adminService, err := admin.NewService(db.Queries, cfg, storageService)
	if err != nil {
		return nil, err
	}
	adminServer := admin.NewServer(adminService, jobService)

	s := &Server{
		config:                cfg,
		db:                    db,
		authService:           authService,
		wsHub:                 wsHub,
		userService:           userService,
		assetService:          assetService,
		apiKeyService:         apiKeyService,
		librariesServer:       libraryServer,
		searchServer:          searchServer,
		downloadService:       downloadService,
		sharedLinksService:    sharedLinksService,
		systemConfigService:   systemConfigService,
		jobService:            jobService,
		trashService:          trashService,
		tagsService:           tagsService,
		mapService:            mapService,
		peopleService:         peopleService,
		partnersService:       partnersService,
		activityService:       activityService,
		adminServer:           adminServer,
		duplicatesServer:      duplicatesServer,
		facesServer:           facesServer,
		stacksServer:          stacksServer,
		systemMetadataService: systemMetadataService,
		systemMetadataServer:  systemMetadataServer,
		viewServer:            viewServer,
		sessionsService:       sessionsService,
		sessionsServer:        sessionsServer,
		syncServer:            syncServer,
		memoriesServer:        memoriesServer,
		notificationsServer:   notificationsServer,
		timelineService:       timelineService,
		timelineServer:        timelineServer,
		maintenanceService:    maintenanceService,
		queueService:          queueService,
		pluginService:         pluginService,
		workflowService:       workflowService,
		queries:               db.Queries,
	}
	s.grpcServer = grpc.NewServer()

	// Register gRPC services
	immichv1.RegisterAuthServiceServer(s.grpcServer, s)
	immichv1.RegisterAlbumServiceServer(s.grpcServer, s)
	immichv1.RegisterApiKeyServiceServer(s.grpcServer, s)
	immichv1.RegisterAssetServiceServer(s.grpcServer, s)
	immichv1.RegisterDownloadServiceServer(s.grpcServer, s)
	immichv1.RegisterJobServiceServer(s.grpcServer, s)
	immichv1.RegisterLibrariesServiceServer(s.grpcServer, s.librariesServer)
	immichv1.RegisterMemoryServiceServer(s.grpcServer, s.memoriesServer)
	immichv1.RegisterNotificationsServiceServer(s.grpcServer, s.notificationsServer)
	immichv1.RegisterOAuthServiceServer(s.grpcServer, s)
	immichv1.RegisterSearchServiceServer(s.grpcServer, s.searchServer)
	immichv1.RegisterServerServiceServer(s.grpcServer, s)
	immichv1.RegisterSharedLinksServiceServer(s.grpcServer, s)
	immichv1.RegisterSystemConfigServiceServer(s.grpcServer, s)
	immichv1.RegisterTimelineServiceServer(s.grpcServer, s.timelineServer)
	immichv1.RegisterUsersServiceServer(s.grpcServer, s)
	immichv1.RegisterTrashServiceServer(s.grpcServer, s.trashService)
	immichv1.RegisterTagsServiceServer(s.grpcServer, s.tagsService)
	immichv1.RegisterMapServiceServer(s.grpcServer, s.mapService)
	immichv1.RegisterPeopleServiceServer(s.grpcServer, s.peopleService)
	immichv1.RegisterPartnersServiceServer(s.grpcServer, s.partnersService)
	immichv1.RegisterActivityServiceServer(s.grpcServer, s.activityService)
	immichv1.RegisterAdminServiceServer(s.grpcServer, s.adminServer)
	immichv1.RegisterDuplicatesServiceServer(s.grpcServer, s.duplicatesServer)
	immichv1.RegisterFacesServiceServer(s.grpcServer, s.facesServer)
	immichv1.RegisterStacksServiceServer(s.grpcServer, s.stacksServer)
	immichv1.RegisterSystemMetadataServiceServer(s.grpcServer, s.systemMetadataServer)
	immichv1.RegisterViewServiceServer(s.grpcServer, s.viewServer)
	immichv1.RegisterSessionsServiceServer(s.grpcServer, s.sessionsServer)
	immichv1.RegisterSyncServiceServer(s.grpcServer, s.syncServer)
	immichv1.RegisterMaintenanceServiceServer(s.grpcServer, s)
	immichv1.RegisterQueueServiceServer(s.grpcServer, s)
	immichv1.RegisterPluginServiceServer(s.grpcServer, s)
	immichv1.RegisterWorkflowServiceServer(s.grpcServer, s)

	s.recordVersionHistory(context.Background())

	return s, nil
}

// recordVersionHistory appends the running server version to the
// version_history table when it differs from the most recent entry.
func (s *Server) recordVersionHistory(ctx context.Context) {
	current := Version
	if current == "" {
		current = "dev"
	}

	latest, err := s.queries.GetLatestVersionHistory(ctx)
	if err == nil && latest.Version == current {
		return
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		logrus.WithError(err).Warn("failed to read version history")
		return
	}

	if _, err := s.queries.CreateVersionHistory(ctx, current); err != nil {
		logrus.WithError(err).Warn("failed to record version history")
	}
}

func (s *Server) ServeGRPC(listener net.Listener) error {
	logrus.Info("gRPC server starting...")
	return s.grpcServer.Serve(listener)
}

// incomingHeaderMatcher forwards the default grpc-gateway headers plus the
// Immich-specific x-api-key header so API-key-authenticated endpoints (e.g.
// GET /api-keys/me) can inspect the key used for the current request.

// Set some headers

// set http status code

// delete the headers to not expose any grpc-metadata in http response

// HTTPHandler creates and returns the HTTP handler with grpc-gateway

// Register all the service handlers directly with the server implementations
// This avoids the need for an external network connection

// Register service handlers directly

// Register new services

// Outermost wrapper serves a static frontend (e.g. the Immich web build)
// from s.config.WebUIDir. Unmatched requests fall through to the API mux
// so REST/gRPC routes keep working. Empty WebUIDir is a transparent
// passthrough — the API is reachable directly.

// auth.RequireUser/RequireAdmin (used by admin.* and systemmetadata.*
// services) read UserContextKey, not ClaimsContextKey. Without this
// lookup those endpoints reject every caller, admins included.

func (s *Server) SetGRPCConn(conn *grpc.ClientConn) {
	s.grpcClientConn = conn
}

func (s *Server) Stop() {
	logrus.Info("Stopping gRPC server...")
	if s.jobService != nil {
		s.jobService.Stop()
	}
	s.grpcServer.GracefulStop()
	if s.grpcClientConn != nil {
		if err := s.grpcClientConn.Close(); err != nil {
			logrus.WithError(err).Error("Failed to close gRPC client connection")
		}
	}
}

// parseRedisURL parses a Redis URL and returns address, password, and database number.
// Supports formats:
// - redis://localhost:6379
// - redis://:password@localhost:6379
// - redis://:password@localhost:6379/0
// - rediss://... (TLS)

// Default values

// Extract host:port

// Add default port if not specified

// Extract password

// Extract database number from path (e.g., /0, /1, etc.)
