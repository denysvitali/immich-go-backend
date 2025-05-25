package server

import (
	"context"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

type Server struct {
	config      *config.Config
	db          *gorm.DB
	grpcServer  *grpc.Server
	authService *auth.Service

	albumService immichv1.UnimplementedAlbumServiceServer
	assetService immichv1.UnimplementedAssetServiceServer

	immichv1.UnimplementedAuthServiceServer
	immichv1.UnimplementedNotificationsServiceServer
	immichv1.UnimplementedServerServiceServer
}

func NewServer(cfg *config.Config, db *gorm.DB) *Server {
	authService := auth.NewService(cfg.JWT.SecretKey)
	s := Server{
		config:      cfg,
		db:          db,
		authService: authService,
	}

	grpcServer := grpc.NewServer()
	immichv1.RegisterAlbumServiceServer(grpcServer, &s.albumService)
	immichv1.RegisterAssetServiceServer(grpcServer, &s.assetService)
	immichv1.RegisterAuthServiceServer(grpcServer, &s)
	immichv1.RegisterNotificationsServiceServer(grpcServer, &s)
	immichv1.RegisterServerServiceServer(grpcServer, &s)
	s.grpcServer = grpcServer
	return &s
}

func (s *Server) ServeGRPC(listener net.Listener) error {
	logrus.Info("gRPC server starting...")
	return s.grpcServer.Serve(listener)
}

// HTTPHandler creates and returns the HTTP handler with grpc-gateway
func (s *Server) HTTPHandler() http.Handler {
	// Create grpc-gateway mux
	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{}),
		runtime.WithMiddlewares(loggingMiddleware),
	)

	// Register all the service handlers directly with the server implementations
	// This avoids the need for an external network connection
	ctx := context.Background()

	// Register AuthService
	err := immichv1.RegisterAuthServiceHandlerServer(ctx, mux, s)
	if err != nil {
		logrus.WithError(err).Error("Failed to register AuthService handler")
	}

	// Register NotificationsService
	err = immichv1.RegisterNotificationsServiceHandlerServer(ctx, mux, s)
	if err != nil {
		logrus.WithError(err).Error("Failed to register NotificationsService handler")
	}

	// Register AlbumService
	err = immichv1.RegisterAlbumServiceHandlerServer(ctx, mux, &s.albumService)
	if err != nil {
		logrus.WithError(err).Error("Failed to register AlbumService handler")
	}

	// Register AssetService
	err = immichv1.RegisterAssetServiceHandlerServer(ctx, mux, &s.assetService)
	if err != nil {
		logrus.WithError(err).Error("Failed to register AssetService handler")
	}

	err = immichv1.RegisterServerServiceHandlerServer(ctx, mux, s)
	if err != nil {
		logrus.WithError(err).Error("Failed to register ServerService handler")
	}

	return mux
}

func loggingMiddleware(handlerFunc runtime.HandlerFunc) runtime.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		logrus.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
			"query":  r.URL.RawQuery,
		}).Info("Handling request")

		// Call the original handler
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
