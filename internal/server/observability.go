package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
)

const healthProbeTimeout = 5 * time.Second

type dependencyCheck func(context.Context) error

type healthDependencies struct {
	database       dependencyCheck
	storage        dependencyCheck
	jobs           dependencyCheck
	jobsConfigured bool
}

type healthResponse struct {
	Status string             `json:"status"`
	Checks healthCheckResults `json:"checks"`
}

type healthCheckResults struct {
	Database string `json:"database"`
	Storage  string `json:"storage"`
	Jobs     string `json:"jobs"`
}

// SetMetricsHandler supplies the Prometheus handler owned by the serve-level
// telemetry provider. It must be called before HTTPHandler is built.
func (s *Server) SetMetricsHandler(handler http.Handler) {
	s.metrics = handler
}

func (s *Server) operationalHandler(next http.Handler) http.Handler {
	serverConfig := s.config.Server
	var health http.Handler
	if serverConfig.HealthCheckEnabled {
		health = newHealthHandler(healthDependencies{
			database: func(ctx context.Context) error {
				if s.db == nil {
					return errors.New("database is not initialized")
				}
				return s.db.Ping(ctx)
			},
			storage: func(ctx context.Context) error {
				if s.storageService == nil {
					return errors.New("storage is not initialized")
				}
				return s.storageService.HealthCheck(ctx)
			},
			jobs: func(ctx context.Context) error {
				if s.jobService == nil {
					return errors.New("job queue is not initialized")
				}
				return s.jobService.HealthCheck(ctx)
			},
			jobsConfigured: s.config.Jobs.Enabled && s.config.Jobs.RedisURL != "",
		})
	}

	return operationalEndpoints(serverConfig, s.metrics, health, next)
}

func operationalEndpoints(serverConfig config.ServerConfig, metrics, health, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case serverConfig.MetricsEnabled && serverConfig.MetricsPath != "" && r.URL.Path == serverConfig.MetricsPath:
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}
			if metrics == nil {
				http.Error(w, "metrics unavailable", http.StatusServiceUnavailable)
				return
			}
			metrics.ServeHTTP(w, r)
			return

		case serverConfig.HealthCheckEnabled && serverConfig.HealthCheckPath != "" && r.URL.Path == serverConfig.HealthCheckPath:
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}
			if health == nil {
				http.Error(w, "health check unavailable", http.StatusServiceUnavailable)
				return
			}
			health.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func newHealthHandler(dependencies healthDependencies) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthProbeTimeout)
		defer cancel()
		databaseResult := startDependencyCheck(ctx, dependencies.database)
		storageResult := startDependencyCheck(ctx, dependencies.storage)
		var jobsResult <-chan string
		if dependencies.jobsConfigured {
			jobsResult = startDependencyCheck(ctx, dependencies.jobs)
		}

		response := healthResponse{
			Status: "healthy",
			Checks: healthCheckResults{
				Database: awaitDependencyCheck(ctx, databaseResult),
				Storage:  awaitDependencyCheck(ctx, storageResult),
				Jobs:     "disabled",
			},
		}

		statusCode := http.StatusOK
		if response.Checks.Database != "healthy" || response.Checks.Storage != "healthy" {
			response.Status = "unhealthy"
			statusCode = http.StatusServiceUnavailable
		}
		if dependencies.jobsConfigured {
			if awaitDependencyCheck(ctx, jobsResult) == "healthy" {
				response.Checks.Jobs = "healthy"
			} else {
				response.Checks.Jobs = "degraded"
				if response.Status == "healthy" {
					response.Status = "degraded"
				}
			}
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(response)
	})
}

func startDependencyCheck(ctx context.Context, check dependencyCheck) <-chan string {
	result := make(chan string, 1)
	go func() {
		if check == nil || check(ctx) != nil {
			result <- "unhealthy"
			return
		}
		result <- "healthy"
	}()
	return result
}

func awaitDependencyCheck(ctx context.Context, result <-chan string) string {
	// Prefer a completed result even when the shared deadline expires at the
	// same instant; this keeps a slow dependency from misreporting another one.
	select {
	case status := <-result:
		return status
	default:
	}
	select {
	case status := <-result:
		return status
	case <-ctx.Done():
		return "unhealthy"
	}
}
