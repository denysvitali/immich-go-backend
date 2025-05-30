package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/websocket"
)

var (
	// All these fields are set at build time using ldflags
	Version      = "dev"
	SourceCommit = "unknown"
	SourceRef    = "unknown"
	SourceUrl    = "unknown"
)

// CustomMarshaler wraps the default JSONPb marshaler to unwrap single repeated fields
type CustomMarshaler struct {
	*runtime.JSONPb
}

// Marshal implements the Marshaler interface
func (m *CustomMarshaler) Marshal(v interface{}) ([]byte, error) {
	// First marshal with the default marshaler
	data, err := m.JSONPb.Marshal(v)
	if err != nil {
		return nil, err
	}

	// Check if this is a message we want to unwrap
	if msg, ok := v.(proto.Message); ok {
		msgName := string(msg.ProtoReflect().Descriptor().FullName())

		// Unwrap GetAllAlbumsResponse to return albums array directly
		if msgName == "immich.v1.GetAllAlbumsResponse" {
			var obj map[string]interface{}
			if err := json.Unmarshal(data, &obj); err != nil {
				return data, nil // Return original on error
			}

			if albums, exists := obj["albums"]; exists {
				return json.Marshal(albums)
			}
		}
	}

	return data, nil
}

type Server struct {
	config      *config.Config
	db          *db.Conn
	grpcServer  *grpc.Server
	authService *auth.Service
	wsHub       *websocket.Hub

	immichv1.UnimplementedActivityServiceServer
	immichv1.UnimplementedAdminServiceServer
	immichv1.UnimplementedAlbumServiceServer
	immichv1.UnimplementedApiKeyServiceServer
	immichv1.UnimplementedAssetServiceServer
	immichv1.UnimplementedAuthServiceServer
	immichv1.UnimplementedJobServiceServer
	immichv1.UnimplementedMemoryServiceServer
	immichv1.UnimplementedNotificationsServiceServer
	immichv1.UnimplementedSearchServiceServer
	immichv1.UnimplementedServerServiceServer
	immichv1.UnimplementedSystemConfigServiceServer
	immichv1.UnimplementedTimelineServiceServer
	immichv1.UnimplementedUsersServiceServer
}

func NewServer(cfg *config.Config, db *db.Conn) *Server {
	authService := auth.NewService(cfg.JWT.SecretKey)
	wsHub := websocket.New()

	// Start the websocket hub
	go wsHub.Run()

	s := Server{
		config:      cfg,
		db:          db,
		authService: authService,
		wsHub:       wsHub,
	}
	s.grpcServer = grpc.NewServer()
	return &s
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
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &CustomMarshaler{
			JSONPb: &runtime.JSONPb{
				MarshalOptions: protojson.MarshalOptions{},
			},
		}),
		runtime.WithMiddlewares(loggingMiddleware),
		runtime.WithForwardResponseOption(httpResponseModifier),
	)

	// Register all the service handlers directly with the server implementations
	// This avoids the need for an external network connection
	ctx := context.Background()

	// Register service handlers directly
	if err := immichv1.RegisterActivityServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register ActivityService handler")
	}
	if err := immichv1.RegisterAdminServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register AdminService handler")
	}
	if err := immichv1.RegisterAlbumServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register AlbumService handler")
	}
	if err := immichv1.RegisterApiKeyServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register ApiKeyService handler")
	}
	if err := immichv1.RegisterAssetServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register AssetService handler")
	}
	if err := immichv1.RegisterAuthServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register AuthService handler")
	}
	if err := immichv1.RegisterJobServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register JobService handler")
	}
	if err := immichv1.RegisterMemoryServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register MemoryService handler")
	}
	if err := immichv1.RegisterNotificationsServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register NotificationsService handler")
	}
	if err := immichv1.RegisterSearchServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register SearchService handler")
	}
	if err := immichv1.RegisterServerServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register ServerService handler")
	}
	if err := immichv1.RegisterSystemConfigServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register SystemConfigService handler")
	}
	if err := immichv1.RegisterTimelineServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register TimelineService handler")
	}
	if err := immichv1.RegisterUsersServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register UsersService handler")
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
