package server

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/denysvitali/immich-go-backend/internal/activity"
	"github.com/denysvitali/immich-go-backend/internal/admin"
	"github.com/denysvitali/immich-go-backend/internal/albums"
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
	mapservice "github.com/denysvitali/immich-go-backend/internal/map"
	"github.com/denysvitali/immich-go-backend/internal/memories"
	"github.com/denysvitali/immich-go-backend/internal/notifications"
	"github.com/denysvitali/immich-go-backend/internal/partners"
	"github.com/denysvitali/immich-go-backend/internal/people"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
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
	albumService          *albums.Service
	apiKeyService         *apikeys.Service
	libraryService        *libraries.Service
	librariesServer       *libraries.Server
	searchService         *search.Service
	searchServer          *search.Server
	downloadService       *download.Service
	sharedLinksService    *sharedlinks.Service
	systemConfigService   *systemconfig.Service
	jobService            *jobs.Service
	trashService          *trash.Server
	tagsService           *tags.Server
	mapService            *mapservice.Server
	peopleService         *people.Server
	partnersService       *partners.Server
	activityService       *activity.Server
	adminService          *admin.Service
	adminServer           *admin.Server
	duplicatesService     *duplicates.Service
	duplicatesServer      *duplicates.Server
	facesService          *faces.Service
	facesServer           *faces.Server
	stacksService         *stacks.Service
	stacksServer          *stacks.Server
	systemMetadataService *systemmetadata.Service
	systemMetadataServer  *systemmetadata.Server
	viewService           *view.Service
	viewServer            *view.Server
	sessionsService       *sessions.Service
	sessionsServer        *sessions.Server
	syncService           *sync.Service
	syncServer            *sync.Server
	memoriesService       *memories.Service
	memoriesServer        *memories.Server
	notificationsService  *notifications.Service
	notificationsServer   *notifications.Server
	timelineService       *timeline.Service
	timelineServer        *timeline.Server
	queries               *sqlc.Queries

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

	albumService := albums.NewService(db.Queries)
	apiKeyService := apikeys.NewService(db.Queries)
	libraryService := libraries.NewService(db.Queries, cfg, storageService)
	searchService := search.NewService(db.Queries)
	downloadService := download.NewService(db.Queries, storageService)
	sharedLinksService := sharedlinks.NewService(db.Queries)
	systemConfigService := systemconfig.NewService(db.Queries)
	trashService := trash.NewServer(db.Queries)
	tagsService := tags.NewServer(db.Queries)
	mapService := mapservice.NewServer(db.Queries)
	peopleService := people.NewServer(db.Queries)
	partnersService := partners.NewServer(db.Queries)
	activityService := activity.NewServer(db.Queries)

	// Initialize new services
	adminService, err := admin.NewService(db.Queries, cfg)
	if err != nil {
		return nil, err
	}
	adminServer := admin.NewServer(adminService)

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

	// Create server implementations for libraries and search
	libraryServer := libraries.NewServer(libraryService)
	searchServer := search.NewServer(searchService)

	// Initialize job service (requires Redis configuration)
	var jobService *jobs.Service
	if cfg.Jobs.Enabled && cfg.Jobs.RedisURL != "" {
		// Parse Redis URL to get components
		// For now, use a simple default - in production this should parse the URL
		jobCfg := &jobs.Config{
			RedisAddr:     "localhost:6379", // TODO: Parse from RedisURL
			RedisPassword: "",
			RedisDB:       0,
			Concurrency:   cfg.Jobs.Workers,
			QueueName:     "immich",
		}
		var err error
		jobService, err = jobs.NewService(jobCfg)
		if err != nil {
			logrus.WithError(err).Warn("Failed to initialize job service, background processing disabled")
		}
	} else {
		logrus.Warn("Job service not configured, background processing disabled")
	}

	s := &Server{
		config:                cfg,
		db:                    db,
		authService:           authService,
		wsHub:                 wsHub,
		userService:           userService,
		assetService:          assetService,
		albumService:          albumService,
		apiKeyService:         apiKeyService,
		libraryService:        libraryService,
		librariesServer:       libraryServer,
		searchService:         searchService,
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
		adminService:          adminService,
		adminServer:           adminServer,
		duplicatesService:     duplicatesService,
		duplicatesServer:      duplicatesServer,
		facesService:          facesService,
		facesServer:           facesServer,
		stacksService:         stacksService,
		stacksServer:          stacksServer,
		systemMetadataService: systemMetadataService,
		systemMetadataServer:  systemMetadataServer,
		viewService:           viewService,
		viewServer:            viewServer,
		sessionsService:       sessionsService,
		sessionsServer:        sessionsServer,
		syncService:           syncService,
		syncServer:            syncServer,
		memoriesService:       memoriesService,
		memoriesServer:        memoriesServer,
		notificationsService:  notificationsService,
		notificationsServer:   notificationsServer,
		timelineService:       timelineService,
		timelineServer:        timelineServer,
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

	return s, nil
}

func (s *Server) ServeGRPC(listener net.Listener) error {
	logrus.Info("gRPC server starting...")
	return s.grpcServer.Serve(listener)
}

func httpResponseModifier(ctx context.Context, w http.ResponseWriter, p proto.Message) error {
	md, ok := runtime.ServerMetadataFromContext(ctx)
	if !ok {
		return nil
	}

	allowedHeaders := map[string]any{
		"set-cookie": struct{}{},
	}

	// Set some headers
	for key, values := range md.HeaderMD {
		cleanKey := strings.TrimPrefix(strings.ToLower(key), "grpc-metadata-")
		if _, ok := allowedHeaders[cleanKey]; !ok {
			logrus.Warnf("Ignoring header %s in HTTP response", key)
			continue
		}

		if len(values) > 1 {
			logrus.Warnf("Multiple values for header %s, using first value only", key)
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
		delete(md.HeaderMD, key)
		w.Header().Del("Grpc-Metadata-" + key)
	}

	// set http status code
	if vals := md.HeaderMD.Get("x-http-code"); len(vals) > 0 {
		code, err := strconv.Atoi(vals[0])
		if err != nil {
			return err
		}
		// delete the headers to not expose any grpc-metadata in http response
		delete(md.HeaderMD, "x-http-code")
		delete(w.Header(), "Grpc-Metadata-X-Http-Code")
		w.WriteHeader(code)
	}

	return nil
}

// HTTPHandler creates and returns the HTTP handler with grpc-gateway
func (s *Server) HTTPHandler() http.Handler {
	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				EmitDefaultValues: true,
			},
		}),
		runtime.WithMiddlewares(loggingMiddleware),
		runtime.WithForwardResponseOption(httpResponseModifier),
	)

	// Register all the service handlers directly with the server implementations
	// This avoids the need for an external network connection
	ctx := context.Background()

	// Register service handlers directly
	if err := immichv1.RegisterAuthServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register AuthService handler")
	}
	if err := immichv1.RegisterAlbumServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register AlbumService handler")
	}
	if err := immichv1.RegisterApiKeyServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register ApiKeyService handler")
	}
	if err := immichv1.RegisterLibrariesServiceHandlerServer(ctx, mux, s.librariesServer); err != nil {
		logrus.WithError(err).Error("Failed to register LibrariesService handler")
	}
	if err := immichv1.RegisterSearchServiceHandlerServer(ctx, mux, s.searchServer); err != nil {
		logrus.WithError(err).Error("Failed to register SearchService handler")
	}
	if err := immichv1.RegisterAssetServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register AssetService handler")
	}
	if err := immichv1.RegisterMemoryServiceHandlerServer(ctx, mux, s.memoriesServer); err != nil {
		logrus.WithError(err).Error("Failed to register MemoryService handler")
	}
	if err := immichv1.RegisterNotificationsServiceHandlerServer(ctx, mux, s.notificationsServer); err != nil {
		logrus.WithError(err).Error("Failed to register NotificationsService handler")
	}
	if err := immichv1.RegisterOAuthServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register OAuthService handler")
	}
	if err := immichv1.RegisterServerServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register ServerService handler")
	}
	if err := immichv1.RegisterTimelineServiceHandlerServer(ctx, mux, s.timelineServer); err != nil {
		logrus.WithError(err).Error("Failed to register TimelineService handler")
	}
	if err := immichv1.RegisterUsersServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register UsersService handler")
	}
	if err := immichv1.RegisterDownloadServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register DownloadService handler")
	}
	if err := immichv1.RegisterSharedLinksServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register SharedLinksService handler")
	}
	if err := immichv1.RegisterSystemConfigServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register SystemConfigService handler")
	}
	if s.jobService != nil {
		if err := immichv1.RegisterJobServiceHandlerServer(ctx, mux, s); err != nil {
			logrus.WithError(err).Error("Failed to register JobService handler")
		}
	}
	// Register new services
	if err := immichv1.RegisterTrashServiceHandlerServer(ctx, mux, s.trashService); err != nil {
		logrus.WithError(err).Error("Failed to register TrashService handler")
	}
	if err := immichv1.RegisterTagsServiceHandlerServer(ctx, mux, s.tagsService); err != nil {
		logrus.WithError(err).Error("Failed to register TagsService handler")
	}
	if err := immichv1.RegisterMapServiceHandlerServer(ctx, mux, s.mapService); err != nil {
		logrus.WithError(err).Error("Failed to register MapService handler")
	}
	if err := immichv1.RegisterPeopleServiceHandlerServer(ctx, mux, s.peopleService); err != nil {
		logrus.WithError(err).Error("Failed to register PeopleService handler")
	}
	if err := immichv1.RegisterPartnersServiceHandlerServer(ctx, mux, s.partnersService); err != nil {
		logrus.WithError(err).Error("Failed to register PartnersService handler")
	}
	if err := immichv1.RegisterActivityServiceHandlerServer(ctx, mux, s.activityService); err != nil {
		logrus.WithError(err).Error("Failed to register ActivityService handler")
	}
	if err := immichv1.RegisterAdminServiceHandlerServer(ctx, mux, s.adminServer); err != nil {
		logrus.WithError(err).Error("Failed to register AdminService handler")
	}
	if err := immichv1.RegisterDuplicatesServiceHandlerServer(ctx, mux, s.duplicatesServer); err != nil {
		logrus.WithError(err).Error("Failed to register DuplicatesService handler")
	}
	if err := immichv1.RegisterFacesServiceHandlerServer(ctx, mux, s.facesServer); err != nil {
		logrus.WithError(err).Error("Failed to register FacesService handler")
	}
	if err := immichv1.RegisterStacksServiceHandlerServer(ctx, mux, s.stacksServer); err != nil {
		logrus.WithError(err).Error("Failed to register StacksService handler")
	}
	if err := immichv1.RegisterSystemMetadataServiceHandlerServer(ctx, mux, s.systemMetadataServer); err != nil {
		logrus.WithError(err).Error("Failed to register SystemMetadataService handler")
	}
	if err := immichv1.RegisterViewServiceHandlerServer(ctx, mux, s.viewServer); err != nil {
		logrus.WithError(err).Error("Failed to register ViewService handler")
	}
	if err := immichv1.RegisterSessionsServiceHandlerServer(ctx, mux, s.sessionsServer); err != nil {
		logrus.WithError(err).Error("Failed to register SessionsService handler")
	}
	if err := immichv1.RegisterSyncServiceHandlerServer(ctx, mux, s.syncServer); err != nil {
		logrus.WithError(err).Error("Failed to register SyncService handler")
	}
	return s.handleWs(mux)
}

func (s *Server) handleWs(mux *runtime.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/socket.io/" {
			s.wsHub.HandleWebSocket(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func loggingMiddleware(handlerFunc runtime.HandlerFunc) runtime.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		logrus.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
			"query":  r.URL.RawQuery,
		}).Info("Handling request")
		handlerFunc(w, r, pathParams)
	}
}

func (s *Server) Stop() {
	logrus.Info("Stopping gRPC server...")
	s.grpcServer.GracefulStop()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}
