package telemetry

import (
	"fmt"

	"go.opentelemetry.io/otel/metric"
)

// ServiceMetrics is the instrument set the domain services in this repository
// record: how many operations ran, how long they took, and how many things the
// domain currently owns.
//
// The zero value is not usable; build one with NewServiceMetrics.
type ServiceMetrics struct {
	// Operations counts operations performed, keyed by an "operation" attribute.
	Operations metric.Int64Counter
	// Duration records operation wall time in seconds.
	Duration metric.Float64Histogram
	// Items tracks how many domain objects exist, and so can go down as well
	// as up.
	Items metric.Int64UpDownCounter
}

// ServiceMetricNames spells out the instrument names and descriptions for one
// service.
//
// They are given explicitly rather than derived from a prefix because metric
// names are an external contract: the Grafana dashboards and Prometheus alert
// rules under deploy/monitoring/ query them by name, so a "tidier" naming
// scheme would silently break them.
type ServiceMetricNames struct {
	Operations            string
	OperationsDescription string
	Duration              string
	DurationDescription   string
	Items                 string
	ItemsDescription      string
}

// NewServiceMetrics creates the three instruments on the global meter.
//
// It returns an error only if the configured meter provider rejects an
// instrument name. The global no-op provider — what runs when telemetry is
// disabled — accepts anything, so in practice this only fails once a real SDK
// provider is installed. The error is surfaced rather than swallowed so a typo
// cannot silently disable a dashboard.
func NewServiceMetrics(names ServiceMetricNames) (ServiceMetrics, error) {
	meter := GetMeter()

	operations, err := meter.Int64Counter(names.Operations,
		metric.WithDescription(names.OperationsDescription))
	if err != nil {
		return ServiceMetrics{}, fmt.Errorf("create counter %q: %w", names.Operations, err)
	}

	duration, err := meter.Float64Histogram(names.Duration,
		metric.WithDescription(names.DurationDescription))
	if err != nil {
		return ServiceMetrics{}, fmt.Errorf("create histogram %q: %w", names.Duration, err)
	}

	items, err := meter.Int64UpDownCounter(names.Items,
		metric.WithDescription(names.ItemsDescription))
	if err != nil {
		return ServiceMetrics{}, fmt.Errorf("create up/down counter %q: %w", names.Items, err)
	}

	return ServiceMetrics{Operations: operations, Duration: duration, Items: items}, nil
}
