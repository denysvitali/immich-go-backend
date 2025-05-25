package server

import (
	"context"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
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
	authSvc      immichv1.UnimplementedAuthServiceServer
	immichAPI    immichv1.UnimplementedImmichAPIServer
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
	immichv1.RegisterAuthServiceServer(grpcServer, &s.authSvc)
	immichv1.RegisterImmichAPIServer(grpcServer, &s.immichAPI)
	s.grpcServer = grpcServer
	return &s
}

func (s *Server) ServeGRPC(listener net.Listener) error {
	logrus.Info("gRPC server starting...")
	return s.grpcServer.Serve(listener)
}

// getGRPCEndpoint returns the gRPC server endpoint
func (s *Server) getGRPCEndpoint() string {
	return s.config.Server.Host + ":" + s.config.Server.GRPCPort
}

// HTTPHandler creates and returns the HTTP handler with grpc-gateway
func (s *Server) HTTPHandler() http.Handler {
	if s.config.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create grpc-gateway mux
	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{}),
	)

	// Register all the service handlers directly with the server implementations
	// This avoids the need for an external network connection
	ctx := context.Background()

	// Register AuthService
	err := immichv1.RegisterAuthServiceHandlerServer(ctx, mux, &s.authSvc)
	if err != nil {
		logrus.WithError(err).Error("Failed to register AuthService handler")
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

	// Register ImmichAPI
	err = immichv1.RegisterImmichAPIHandlerServer(ctx, mux, &s.immichAPI)
	if err != nil {
		logrus.WithError(err).Error("Failed to register ImmichAPI handler")
	}

	return mux
}

func (s *Server) Stop() {
	logrus.Info("Stopping gRPC server...")
	s.grpcServer.GracefulStop()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}
