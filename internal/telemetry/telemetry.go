package telemetry

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Config represents telemetry configuration
type Config struct {
	ServiceName    string `yaml:"service_name" env:"OTEL_SERVICE_NAME" default:"immich-go-backend"`
	ServiceVersion string `yaml:"service_version" env:"OTEL_SERVICE_VERSION" default:"1.0.0"`
	Environment    string `yaml:"environment" env:"OTEL_ENVIRONMENT" default:"development"`
	
	// Tracing configuration
	TracingEnabled bool `yaml:"tracing_enabled" env:"OTEL_TRACING_ENABLED" default:"true"`
	
	// Metrics configuration
	MetricsEnabled bool `yaml:"metrics_enabled" env:"OTEL_METRICS_ENABLED" default:"true"`
	
	// Sampling configuration
	TraceSampleRate float64 `yaml:"trace_sample_rate" env:"OTEL_TRACE_SAMPLE_RATE" default:"1.0"`
	
	// Resource attributes
	ResourceAttributes map[string]string `yaml:"resource_attributes" env:"OTEL_RESOURCE_ATTRIBUTES"`
}

// Provider manages OpenTelemetry providers
type Provider struct {
	config          Config
	traceProvider   *sdktrace.TracerProvider
	metricProvider  *sdkmetric.MeterProvider
	shutdownFuncs   []func(context.Context) error
}

// NewProvider creates a new telemetry provider
func NewProvider(config Config) (*Provider, error) {
	provider := &Provider{
		config:        config,
		shutdownFuncs: make([]func(context.Context) error, 0),
	}

	// Create resource
	res, err := provider.createResource()
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Setup tracing
	if config.TracingEnabled {
		if err := provider.setupTracing(res); err != nil {
			return nil, fmt.Errorf("failed to setup tracing: %w", err)
		}
	}

	// Setup metrics
	if config.MetricsEnabled {
		if err := provider.setupMetrics(res); err != nil {
			return nil, fmt.Errorf("failed to setup metrics: %w", err)
		}
	}

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return provider, nil
}

// createResource creates an OpenTelemetry resource
func (p *Provider) createResource() (*resource.Resource, error) {
	attributes := []attribute.KeyValue{
		semconv.ServiceName(p.config.ServiceName),
		semconv.ServiceVersion(p.config.ServiceVersion),
		semconv.DeploymentEnvironment(p.config.Environment),
	}

	// Add custom resource attributes
	for key, value := range p.config.ResourceAttributes {
		attributes = append(attributes, attribute.String(key, value))
	}

	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			attributes...,
		),
	)
}

// setupTracing configures OpenTelemetry tracing
func (p *Provider) setupTracing(res *resource.Resource) error {
	// Create trace exporter using autoexport
	traceExporter, err := autoexport.NewSpanExporter(context.Background())
	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create trace provider
	p.traceProvider = sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(p.config.TraceSampleRate)),
	)

	// Set global trace provider
	otel.SetTracerProvider(p.traceProvider)

	// Add shutdown function
	p.shutdownFuncs = append(p.shutdownFuncs, p.traceProvider.Shutdown)

	log.Printf("OpenTelemetry tracing initialized with sample rate: %.2f", p.config.TraceSampleRate)
	return nil
}

// setupMetrics configures OpenTelemetry metrics
func (p *Provider) setupMetrics(res *resource.Resource) error {
	// Create metric exporter using autoexport
	metricReader, err := autoexport.NewMetricReader(context.Background())
	if err != nil {
		return fmt.Errorf("failed to create metric reader: %w", err)
	}

	// Create metric provider
	p.metricProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(metricReader),
	)

	// Set global metric provider
	otel.SetMeterProvider(p.metricProvider)

	// Add shutdown function
	p.shutdownFuncs = append(p.shutdownFuncs, p.metricProvider.Shutdown)

	log.Println("OpenTelemetry metrics initialized")
	return nil
}

// GetTracer returns a tracer for the given name
func (p *Provider) GetTracer(name string, opts ...trace.TracerOption) trace.Tracer {
	if p.traceProvider == nil {
		return otel.GetTracerProvider().Tracer(name, opts...)
	}
	return p.traceProvider.Tracer(name, opts...)
}

// GetMeter returns a meter for the given name
func (p *Provider) GetMeter(name string, opts ...metric.MeterOption) metric.Meter {
	if p.metricProvider == nil {
		return otel.GetMeterProvider().Meter(name, opts...)
	}
	return p.metricProvider.Meter(name, opts...)
}

// GetMeter returns a global meter for the given name
func GetMeter() metric.Meter {
	return otel.GetMeterProvider().Meter("immich-go-backend")
}

// Shutdown gracefully shuts down all telemetry providers
func (p *Provider) Shutdown(ctx context.Context) error {
	var errors []error

	for _, shutdown := range p.shutdownFuncs {
		if err := shutdown(ctx); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to shutdown telemetry providers: %v", errors)
	}

	log.Println("OpenTelemetry providers shut down successfully")
	return nil
}

// Metrics holds application-specific metrics
type Metrics struct {
	// HTTP metrics
	HTTPRequestsTotal     metric.Int64Counter
	HTTPRequestDuration   metric.Float64Histogram
	HTTPRequestSize       metric.Int64Histogram
	HTTPResponseSize      metric.Int64Histogram

	// Storage metrics
	StorageOperationsTotal metric.Int64Counter
	StorageOperationDuration metric.Float64Histogram
	StorageSize           metric.Int64Histogram

	// Database metrics
	DBConnectionsActive   metric.Int64UpDownCounter
	DBConnectionsIdle     metric.Int64UpDownCounter
	DBQueriesTotal        metric.Int64Counter
	DBQueryDuration       metric.Float64Histogram

	// Asset metrics
	AssetsTotal           metric.Int64UpDownCounter
	AssetUploadsTotal     metric.Int64Counter
	AssetDownloadsTotal   metric.Int64Counter
	AssetProcessingDuration metric.Float64Histogram

	// User metrics
	UsersTotal            metric.Int64UpDownCounter
	UserSessionsActive    metric.Int64UpDownCounter
	UserLoginTotal        metric.Int64Counter
}

// NewMetrics creates application-specific metrics
func NewMetrics(meter metric.Meter) (*Metrics, error) {
	metrics := &Metrics{}

	var err error

	// HTTP metrics
	metrics.HTTPRequestsTotal, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_requests_total counter: %w", err)
	}

	metrics.HTTPRequestDuration, err = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_request_duration_seconds histogram: %w", err)
	}

	metrics.HTTPRequestSize, err = meter.Int64Histogram(
		"http_request_size_bytes",
		metric.WithDescription("HTTP request size in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_request_size_bytes histogram: %w", err)
	}

	metrics.HTTPResponseSize, err = meter.Int64Histogram(
		"http_response_size_bytes",
		metric.WithDescription("HTTP response size in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_response_size_bytes histogram: %w", err)
	}

	// Storage metrics
	metrics.StorageOperationsTotal, err = meter.Int64Counter(
		"storage_operations_total",
		metric.WithDescription("Total number of storage operations"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage_operations_total counter: %w", err)
	}

	metrics.StorageOperationDuration, err = meter.Float64Histogram(
		"storage_operation_duration_seconds",
		metric.WithDescription("Storage operation duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage_operation_duration_seconds histogram: %w", err)
	}

	metrics.StorageSize, err = meter.Int64Histogram(
		"storage_size_bytes",
		metric.WithDescription("Storage operation size in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage_size_bytes histogram: %w", err)
	}

	// Database metrics
	metrics.DBConnectionsActive, err = meter.Int64UpDownCounter(
		"db_connections_active",
		metric.WithDescription("Number of active database connections"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create db_connections_active counter: %w", err)
	}

	metrics.DBConnectionsIdle, err = meter.Int64UpDownCounter(
		"db_connections_idle",
		metric.WithDescription("Number of idle database connections"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create db_connections_idle counter: %w", err)
	}

	metrics.DBQueriesTotal, err = meter.Int64Counter(
		"db_queries_total",
		metric.WithDescription("Total number of database queries"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create db_queries_total counter: %w", err)
	}

	metrics.DBQueryDuration, err = meter.Float64Histogram(
		"db_query_duration_seconds",
		metric.WithDescription("Database query duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create db_query_duration_seconds histogram: %w", err)
	}

	// Asset metrics
	metrics.AssetsTotal, err = meter.Int64UpDownCounter(
		"assets_total",
		metric.WithDescription("Total number of assets"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create assets_total counter: %w", err)
	}

	metrics.AssetUploadsTotal, err = meter.Int64Counter(
		"asset_uploads_total",
		metric.WithDescription("Total number of asset uploads"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset_uploads_total counter: %w", err)
	}

	metrics.AssetDownloadsTotal, err = meter.Int64Counter(
		"asset_downloads_total",
		metric.WithDescription("Total number of asset downloads"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset_downloads_total counter: %w", err)
	}

	metrics.AssetProcessingDuration, err = meter.Float64Histogram(
		"asset_processing_duration_seconds",
		metric.WithDescription("Asset processing duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset_processing_duration_seconds histogram: %w", err)
	}

	// User metrics
	metrics.UsersTotal, err = meter.Int64UpDownCounter(
		"users_total",
		metric.WithDescription("Total number of users"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create users_total counter: %w", err)
	}

	metrics.UserSessionsActive, err = meter.Int64UpDownCounter(
		"user_sessions_active",
		metric.WithDescription("Number of active user sessions"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user_sessions_active counter: %w", err)
	}

	metrics.UserLoginTotal, err = meter.Int64Counter(
		"user_login_total",
		metric.WithDescription("Total number of user logins"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user_login_total counter: %w", err)
	}

	return metrics, nil
}

// GetDefaultConfig returns a default telemetry configuration
func GetDefaultConfig() Config {
	return Config{
		ServiceName:     "immich-go-backend",
		ServiceVersion:  "1.0.0",
		Environment:     "development",
		TracingEnabled:  true,
		MetricsEnabled:  true,
		TraceSampleRate: 1.0,
		ResourceAttributes: map[string]string{
			"service.instance.id": "immich-go-backend-1",
		},
	}
}