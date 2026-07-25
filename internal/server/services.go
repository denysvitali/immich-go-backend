package server

import (
	"github.com/denysvitali/immich-go-backend/internal/activity"
	"github.com/denysvitali/immich-go-backend/internal/admin"
	"github.com/denysvitali/immich-go-backend/internal/apikeys"
	"github.com/denysvitali/immich-go-backend/internal/assets"
	"github.com/denysvitali/immich-go-backend/internal/download"
	"github.com/denysvitali/immich-go-backend/internal/duplicates"
	"github.com/denysvitali/immich-go-backend/internal/faces"
	"github.com/denysvitali/immich-go-backend/internal/jobs"
	"github.com/denysvitali/immich-go-backend/internal/libraries"
	"github.com/denysvitali/immich-go-backend/internal/maintenance"
	mapservice "github.com/denysvitali/immich-go-backend/internal/map"
	"github.com/denysvitali/immich-go-backend/internal/memories"
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
	"github.com/denysvitali/immich-go-backend/internal/sync"
	"github.com/denysvitali/immich-go-backend/internal/systemconfig"
	"github.com/denysvitali/immich-go-backend/internal/systemmetadata"
	"github.com/denysvitali/immich-go-backend/internal/timeline"
	"github.com/denysvitali/immich-go-backend/internal/trash"
	"github.com/denysvitali/immich-go-backend/internal/users"
	"github.com/denysvitali/immich-go-backend/internal/view"
	"github.com/denysvitali/immich-go-backend/internal/workflow"
)

// services is every domain service a Server delegates to. It exists so
// NewServer stays readable: construction lives in newServices, and Server's
// own fields are limited to what the transport layer owns (config, database,
// gRPC server, auth, websocket hub).
//
// It is embedded in Server by value, so field access reads as s.assetService
// and the zero value is usable in tests that only need one service populated.
type services struct {
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
}
