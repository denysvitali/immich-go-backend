package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/webui"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// incomingHeaderMatcher forwards the default grpc-gateway headers plus the
// Immich-specific x-api-key header so API-key-authenticated endpoints (e.g.
// GET /api-keys/me) can inspect the key used for the current request.
func incomingHeaderMatcher(key string) (string, bool) {
	if strings.EqualFold(key, "x-api-key") {
		return "x-api-key", true
	}
	return runtime.DefaultHeaderMatcher(key)
}

func httpResponseModifier(ctx context.Context, w http.ResponseWriter, p proto.Message) error {
	md, ok := runtime.ServerMetadataFromContext(ctx)
	if !ok {
		return nil
	}

	allowedHeaders := map[string]any{
		"set-cookie": struct{}{},
	}

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

	if vals := md.HeaderMD.Get("x-http-code"); len(vals) > 0 {
		code, err := strconv.Atoi(vals[0])
		if err != nil {
			return err
		}

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
		runtime.WithMiddlewares(s.authContextMiddleware),
		runtime.WithIncomingHeaderMatcher(incomingHeaderMatcher),
		runtime.WithErrorHandler(loggingHTTPErrorHandler),
		runtime.WithForwardResponseOption(httpResponseModifier),
	)

	ctx := context.Background()

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
	if err := immichv1.RegisterJobServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register JobService handler")
	}
	if err := immichv1.RegisterQueueServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register QueueService handler")
	}
	if err := immichv1.RegisterMaintenanceServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register MaintenanceService handler")
	}

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
	if s.grpcClientConn != nil {
		if err := immichv1.RegisterSyncServiceHandler(ctx, mux, s.grpcClientConn); err != nil {
			logrus.WithError(err).Error("Failed to register SyncService handler")
		}
	} else {
		if err := immichv1.RegisterSyncServiceHandlerServer(ctx, mux, s.syncServer); err != nil {
			logrus.WithError(err).Error("Failed to register SyncService handler")
		}
	}
	if err := immichv1.RegisterPluginServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register PluginService handler")
	}
	if err := immichv1.RegisterWorkflowServiceHandlerServer(ctx, mux, s); err != nil {
		logrus.WithError(err).Error("Failed to register WorkflowService handler")
	}

	return webui.Handler(s.config.WebUIDir, httpLoggingHandler(s.handleWs(mux)))
}

func (s *Server) handleWs(mux *runtime.ServeMux) http.Handler {
	marshaler := &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			EmitDefaultValues: true,
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/socket.io/" {
			s.wsHub.HandleWebSocket(w, r)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/api/oauth/mobile-redirect" {
			http.Redirect(w, r, mobileOAuthRedirectURL(r.URL.RawQuery), http.StatusTemporaryRedirect)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/api/users/me" {
			ctx := gatewayIncomingContext(r)
			resp, err := s.GetMyUser(ctx, &emptypb.Empty{})
			if err != nil {
				runtime.HTTPError(ctx, mux, marshaler, w, r, err)
				return
			}
			writeUserAdminJSON(w, marshaler, resp)
			return
		}
		if r.Method == http.MethodPut && r.URL.Path == "/api/tags/assets" {
			s.authContextMiddleware(func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
				s.handleBulkTagAssets(w, r, mux, marshaler)
			})(w, r, nil)
			return
		}
		if s.handleActivityRoutes(w, r) {
			return
		}
		if s.handleQueueListShape(w, r) {
			return
		}
		if s.handleFrontendShape(w, r) {
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func gatewayIncomingContext(r *http.Request) context.Context {
	md := metadata.MD{}
	if authorization := requestAuthorization(r); authorization != "" {
		md.Set("authorization", authorization)
	}
	return metadata.NewIncomingContext(r.Context(), md)
}

func writeProtoJSON(w http.ResponseWriter, marshaler runtime.Marshaler, msg proto.Message) {
	data, err := marshaler.Marshal(msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", marshaler.ContentType(msg))
	_, _ = w.Write(data)
}

func loggingHTTPErrorHandler(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
	code := status.Code(err)
	statusCode := runtime.HTTPStatusFromCode(code)
	entry := logrus.WithError(err).WithFields(logrus.Fields{
		"grpcCode": code.String(),
		"method":   r.Method,
		"path":     r.URL.Path,
		"query":    r.URL.RawQuery,
		"status":   statusCode,
	})
	if statusCode >= http.StatusInternalServerError {
		entry.Error("Gateway request failed")
	} else {
		entry.Warn("Gateway request failed")
	}
	runtime.DefaultHTTPErrorHandler(ctx, mux, marshaler, w, r, err)
}

func (s *Server) authContextMiddleware(handlerFunc runtime.HandlerFunc) runtime.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		authHeader := requestAuthorization(r)
		if authHeader != "" && r.Header.Get("Authorization") == "" {
			r = requestWithAuthorization(r, authHeader)
		}
		if !strings.HasPrefix(authHeader, "Bearer ") {
			handlerFunc(w, r, pathParams)
			return
		}

		claims, err := s.authService.ValidateToken(strings.TrimPrefix(authHeader, "Bearer "))
		if err != nil {
			handlerFunc(w, r, pathParams)
			return
		}

		ctx := context.WithValue(r.Context(), auth.ClaimsContextKey, claims)

		if userInfo, err := s.authService.LoadUserInfo(ctx, claims); err == nil {
			ctx = auth.WithUser(ctx, *userInfo)
		}

		handlerFunc(w, r.WithContext(ctx), pathParams)
	}
}

const immichAccessTokenCookie = "immich_access_token"

func requestAuthorization(r *http.Request) string {
	if authorization := r.Header.Get("Authorization"); authorization != "" {
		return authorization
	}

	cookie, err := r.Cookie(immichAccessTokenCookie)
	if err != nil || cookie.Value == "" {
		return ""
	}

	return "Bearer " + cookie.Value
}

func requestWithAuthorization(r *http.Request, authorization string) *http.Request {
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", authorization)
	return clone
}
