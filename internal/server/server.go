package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

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
	metrics     http.Handler

	// services holds every domain service. Embedded by value, so the zero
	// Server has nil services rather than a nil struct to dereference.
	services

	queries        *sqlc.Queries
	grpcClientConn *grpc.ClientConn

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

	svcs, err := newServices(cfg, db, authService)
	if err != nil {
		return nil, err
	}

	wsHub := websocket.New()
	go wsHub.Run()

	s := &Server{
		config:      cfg,
		db:          db,
		authService: authService,
		wsHub:       wsHub,
		queries:     db.Queries,
		services:    svcs,
	}
	s.grpcServer = grpc.NewServer(
		grpc.ChainUnaryInterceptor(s.authenticationUnaryInterceptor),
		grpc.ChainStreamInterceptor(s.authenticationStreamInterceptor),
	)

	s.registerGRPCServices()

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
	if s.wsHub != nil {
		s.wsHub.Close()
	}
	if s.jobService != nil {
		s.jobService.Stop()
	}
	if s.grpcServer != nil {
		timeout := 30 * time.Second
		if s.config != nil && s.config.Server.ShutdownTimeout > 0 {
			timeout = s.config.Server.ShutdownTimeout
		}
		gracefulDone := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(gracefulDone)
		}()
		timer := time.NewTimer(timeout)
		select {
		case <-gracefulDone:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
			logrus.WithField("timeout", timeout).Warn("gRPC graceful shutdown timed out; forcing stop")
			s.grpcServer.Stop()
			<-gracefulDone
		}
	}
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

// newMLClient builds the machine-learning client. It is always constructed;
// cfg.MLActive() decides whether it will actually call out, so callers never
// need a nil check.
func newMLClient(cfg *config.Config) *ml.Client {
	client := ml.NewClient(ml.Config{
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

	if client.Enabled() {
		logrus.WithField("url", cfg.MachineLearning.URL).Info("Machine learning client enabled")
	} else {
		logrus.Info("Machine learning client disabled (enable Features.MachineLearningEnabled + machine_learning)")
	}
	return client
}

// startJobService builds the asynq queue, registers the real handlers and
// starts the workers. It returns nil — and logs why — when the queue is
// disabled or unreachable: background processing is degradable, so an
// unavailable Redis must not stop the API from serving. Callers must therefore
// treat a nil *jobs.Service as "no background processing".
func startJobService(
	cfg *config.Config,
	db *db.Conn,
	assetService *assets.Service,
	libraryService *libraries.Service,
	storageService *storage.Service,
	mlClient *ml.Client,
) *jobs.Service {
	if !cfg.Jobs.Enabled || cfg.Jobs.RedisURL == "" {
		logrus.Warn("Job service not configured, background processing disabled")
		return nil
	}

	redisAddr, redisPassword, redisDB := parseRedisURL(cfg.Jobs.RedisURL)
	jobService, err := jobs.NewService(&jobs.Config{
		RedisAddr:     redisAddr,
		RedisPassword: redisPassword,
		RedisDB:       redisDB,
		Concurrency:   cfg.Jobs.Workers,
		QueueName:     "immich",
		MaxRetries:    cfg.Jobs.RetryMaxRetries,
		DB:            db.Queries,
	})
	if err != nil {
		logrus.WithError(err).Warn("Failed to initialize job service, background processing disabled")
		return nil
	}

	jobs.NewHandlers(db.Queries, assetService, libraryService, storageService, mlClient, cfg).
		RegisterAllHandlers(jobService)

	// Without Start the workers never run and enqueued jobs (thumbnails,
	// metadata extraction, transcodes) sit in Redis forever.
	if err := jobService.Start(); err != nil {
		logrus.WithError(err).Warn("Failed to start job workers, background processing disabled")
		jobService.Close()
		return nil
	}
	return jobService
}

// registerGRPCServices binds every service implementation to s.grpcServer.
// Services served by Server itself pass s; the rest pass their own *Server.
func (s *Server) registerGRPCServices() {
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
}

// newServices constructs every domain service in dependency order:
// storage and sync first because other services take them, then the
// per-domain services, then the job workers, and finally admin, which needs
// the job service for its dead-letter operations.
func newServices(cfg *config.Config, db *db.Conn, authService *auth.Service) (services, error) {
	userService, err := users.NewService(db.Queries, cfg)
	if err != nil {
		return services{}, err
	}

	storageService, err := storage.NewService(cfg.Storage)
	if err != nil {
		return services{}, err
	}

	// Sync is constructed first: assets publishes change events through it.
	logger := logrus.StandardLogger()
	syncService := sync.NewService(db.Queries, logger)

	assetService, err := assets.NewService(db.Queries, storageService, cfg, syncService)
	if err != nil {
		return services{}, err
	}
	userService.SetPermanentAssetDeleter(assetService)

	mlClient := newMLClient(cfg)

	apiKeyService := apikeys.NewService(db.Queries)
	libraryService := libraries.NewService(db.Queries, cfg, storageService)
	searchService := search.NewService(db.Queries, mlClient, cfg)
	downloadService := download.NewService(db.Queries, storageService)
	sharedLinksService := sharedlinks.NewService(db.Queries)
	systemConfigService := systemconfig.NewService(db.Queries)
	trashService := trash.NewServer(db.Queries, assetService)
	tagsService := tags.NewServer(db.Queries)
	mapService := mapservice.NewServer(db.Queries)
	peopleService := people.NewServer(db.Queries, storageService)
	partnersService := partners.NewServer(db.Queries)
	activityService := activity.NewServer(db.Queries)

	duplicatesService, err := duplicates.NewService(db.Queries, cfg)
	if err != nil {
		return services{}, err
	}
	duplicatesServer := duplicates.NewServer(duplicatesService)

	facesService, err := faces.NewService(db.Queries, cfg)
	if err != nil {
		return services{}, err
	}
	facesServer := faces.NewServer(facesService)

	stacksService, err := stacks.NewService(db.Queries, cfg)
	if err != nil {
		return services{}, err
	}
	stacksServer := stacks.NewServer(stacksService)

	systemMetadataService, err := systemmetadata.NewService(db.Queries, cfg)
	if err != nil {
		return services{}, err
	}
	systemMetadataServer := systemmetadata.NewServer(systemMetadataService)

	viewService, err := view.NewService(db.Queries, cfg)
	if err != nil {
		return services{}, err
	}
	viewServer := view.NewServer(viewService)

	sessionsService := sessions.NewService(db.Queries, authService, logger)
	sessionsServer := sessions.NewServer(sessionsService)

	syncServer := sync.NewServer(syncService)

	memoriesService := memories.NewService(db.Queries)
	memoriesServer := memories.NewServer(memoriesService)

	notificationsService := notifications.NewService(db.Queries)
	notificationsServer := notifications.NewServer(notificationsService)

	timelineService := timeline.NewService(db.Queries)
	timelineServer := timeline.NewServer(timelineService)

	maintenanceService, err := maintenance.NewService(db.Queries, cfg)
	if err != nil {
		return services{}, err
	}

	queueService, err := queue.NewService(db.Queries, cfg)
	if err != nil {
		return services{}, err
	}

	pluginService, err := plugin.NewService(db.Queries, cfg)
	if err != nil {
		return services{}, err
	}

	workflowService, err := workflow.NewService(db.Queries, cfg)
	if err != nil {
		return services{}, err
	}

	libraryServer := libraries.NewServer(libraryService)
	searchServer := search.NewServer(searchService)

	// Finish every fallible constructor before starting background workers.
	// Admin's server wrapper receives the job service after worker startup.
	adminService, err := admin.NewService(db.Queries, cfg, storageService)
	if err != nil {
		return services{}, err
	}
	adminService.SetPermanentAssetDeleter(assetService)

	jobService := startJobService(cfg, db, assetService, libraryService, storageService, mlClient)
	adminServer := admin.NewServer(adminService, jobService)

	return services{
		userService:           userService,
		assetService:          assetService,
		storageService:        storageService,
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
	}, nil
}
