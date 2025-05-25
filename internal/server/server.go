package server

import (
	"context"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/middleware"
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

func (s *Server) HTTPHandler() http.Handler {
	if s.config.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
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
