package server

import (
	"context"
	"errors"
	"fmt"
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
func (s *Server) HTTPHandler() (http.Handler, error) {
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

	if err := s.registerGatewayHandlers(context.Background(), mux); err != nil {
		return nil, err
	}

	apiHandler := s.httpAuthenticationHandler(s.handleWs(mux))
	if s.config.Server.CORSEnabled {
		apiHandler = s.corsHandler(apiHandler)
	}
	// Operational endpoints sit outside the SPA handler. Otherwise its
	// extension-less route fallback serves index.html for /metrics and /health.
	handler := s.operationalHandler(webui.Handler(s.config.WebUIDir, apiHandler))
	return requestLoggingHandler(s.config.Server.RequestLogging, handler), nil
}

// gatewayRegistration names one generated grpc-gateway registrar so a failure
// can be reported against the service it belongs to.
type gatewayRegistration struct {
	service  string
	register func(context.Context, *runtime.ServeMux) error
}

// gatewayHandler adapts a generated Register…HandlerServer function into a
// gatewayRegistration. It is applied in two stages — gatewayHandler(name, fn)
// then (impl) — so that T is inferred from the registrar alone; the
// implementation is then checked for assignability against the service
// interface. A mismatch is a compile error rather than a service that 404s at
// runtime.
func gatewayHandler[T any](service string, fn func(context.Context, *runtime.ServeMux, T) error) func(T) gatewayRegistration {
	return func(impl T) gatewayRegistration {
		return gatewayRegistration{
			service: service,
			register: func(ctx context.Context, mux *runtime.ServeMux) error {
				return fn(ctx, mux, impl)
			},
		}
	}
}

// registerGatewayHandlers wires every service's REST surface onto mux.
//
// A failure here means an entire service returns 404 over REST. That is
// precisely the regression the OPENAPI_COVERAGE_FAIL_UNDER=100 gate exists to
// prevent, and the gate cannot see it because it reads the AST rather than the
// running mux — so registration failure is fatal, not logged. Every failure is
// reported, not just the first.
func (s *Server) registerGatewayHandlers(ctx context.Context, mux *runtime.ServeMux) error {
	registrations := []gatewayRegistration{
		gatewayHandler("ActivityService", immichv1.RegisterActivityServiceHandlerServer)(s.activityService),
		gatewayHandler("AdminService", immichv1.RegisterAdminServiceHandlerServer)(s.adminServer),
		gatewayHandler("AlbumService", immichv1.RegisterAlbumServiceHandlerServer)(s),
		gatewayHandler("ApiKeyService", immichv1.RegisterApiKeyServiceHandlerServer)(s),
		gatewayHandler("AssetService", immichv1.RegisterAssetServiceHandlerServer)(s),
		gatewayHandler("AuthService", immichv1.RegisterAuthServiceHandlerServer)(s),
		gatewayHandler("DownloadService", immichv1.RegisterDownloadServiceHandlerServer)(s),
		gatewayHandler("DuplicatesService", immichv1.RegisterDuplicatesServiceHandlerServer)(s.duplicatesServer),
		gatewayHandler("FacesService", immichv1.RegisterFacesServiceHandlerServer)(s.facesServer),
		gatewayHandler("JobService", immichv1.RegisterJobServiceHandlerServer)(s),
		gatewayHandler("LibrariesService", immichv1.RegisterLibrariesServiceHandlerServer)(s.librariesServer),
		gatewayHandler("MaintenanceService", immichv1.RegisterMaintenanceServiceHandlerServer)(s),
		gatewayHandler("MapService", immichv1.RegisterMapServiceHandlerServer)(s.mapService),
		gatewayHandler("MemoryService", immichv1.RegisterMemoryServiceHandlerServer)(s.memoriesServer),
		gatewayHandler("NotificationsService", immichv1.RegisterNotificationsServiceHandlerServer)(s.notificationsServer),
		gatewayHandler("OAuthService", immichv1.RegisterOAuthServiceHandlerServer)(s),
		gatewayHandler("PartnersService", immichv1.RegisterPartnersServiceHandlerServer)(s.partnersService),
		gatewayHandler("PeopleService", immichv1.RegisterPeopleServiceHandlerServer)(s.peopleService),
		gatewayHandler("PluginService", immichv1.RegisterPluginServiceHandlerServer)(s),
		gatewayHandler("SearchService", immichv1.RegisterSearchServiceHandlerServer)(s.searchServer),
		gatewayHandler("ServerService", immichv1.RegisterServerServiceHandlerServer)(s),
		gatewayHandler("SessionsService", immichv1.RegisterSessionsServiceHandlerServer)(s.sessionsServer),
		gatewayHandler("SharedLinksService", immichv1.RegisterSharedLinksServiceHandlerServer)(s),
		gatewayHandler("StacksService", immichv1.RegisterStacksServiceHandlerServer)(s.stacksServer),
		gatewayHandler("SystemConfigService", immichv1.RegisterSystemConfigServiceHandlerServer)(s),
		gatewayHandler("SystemMetadataService", immichv1.RegisterSystemMetadataServiceHandlerServer)(s.systemMetadataServer),
		gatewayHandler("TagsService", immichv1.RegisterTagsServiceHandlerServer)(s.tagsService),
		gatewayHandler("TimelineService", immichv1.RegisterTimelineServiceHandlerServer)(s.timelineServer),
		gatewayHandler("TrashService", immichv1.RegisterTrashServiceHandlerServer)(s.trashService),
		gatewayHandler("UsersService", immichv1.RegisterUsersServiceHandlerServer)(s),
		gatewayHandler("ViewService", immichv1.RegisterViewServiceHandlerServer)(s.viewServer),
		gatewayHandler("WorkflowService", immichv1.RegisterWorkflowServiceHandlerServer)(s),
		s.syncGatewayRegistration(),
		gatewayHandler("QueueService", immichv1.RegisterQueueServiceHandlerServer)(s),
	}

	var errs []error
	for _, r := range registrations {
		if err := r.register(ctx, mux); err != nil {
			errs = append(errs, fmt.Errorf("register %s gateway handler: %w", r.service, err))
		}
	}
	return errors.Join(errs...)
}

// syncGatewayRegistration prefers the client-connection form so SyncService's
// server-streaming RPCs are forwarded over a real gRPC connection; without a
// connection the gateway falls back to the in-process server, which cannot
// stream.
func (s *Server) syncGatewayRegistration() gatewayRegistration {
	if s.grpcClientConn != nil {
		return gatewayHandler("SyncService", immichv1.RegisterSyncServiceHandler)(s.grpcClientConn)
	}
	return gatewayHandler("SyncService", immichv1.RegisterSyncServiceHandlerServer)(s.syncServer)
}

func (s *Server) handleWs(mux *runtime.ServeMux) http.Handler {
	marshaler := &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			EmitDefaultValues: true,
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/socket.io/" {
			if _, ok := s.requireAuth(w, r); !ok {
				return
			}
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
	if apiKey := strings.TrimSpace(r.Header.Get("x-api-key")); apiKey != "" {
		md.Set("x-api-key", apiKey)
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
		"grpcCode":    code.String(),
		"method":      r.Method,
		"path":        r.URL.Path,
		"queryParams": loggedQueryKeys(r.URL.RawQuery),
		"status":      statusCode,
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
		handlerFunc(w, r.WithContext(s.authenticatedHTTPRequestContext(r)), pathParams)
	}
}

func (s *Server) contextWithAuthenticatedUser(ctx context.Context, claims *auth.Claims) context.Context {
	if _, ok := auth.GetUserFromStdContext(ctx); ok {
		return auth.WithClaims(ctx, claims)
	}
	userInfo, err := s.authService.LoadUserInfo(ctx, claims)
	if err != nil {
		return ctx
	}
	claims.Email = userInfo.Email
	claims.IsAdmin = userInfo.IsAdmin
	ctx = auth.WithClaims(ctx, claims)
	return auth.WithUser(ctx, *userInfo)
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
