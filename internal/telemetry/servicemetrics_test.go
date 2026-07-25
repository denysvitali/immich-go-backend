package telemetry

import "testing"

func TestNewServiceMetrics(t *testing.T) {
	got, err := NewServiceMetrics(ServiceMetricNames{
		Operations:            "test_operations_total",
		OperationsDescription: "Total number of test operations",
		Duration:              "test_operation_duration_seconds",
		DurationDescription:   "Time spent on test operations",
		Items:                 "tests_total",
		ItemsDescription:      "Total number of tests",
	})
	if err != nil {
		t.Fatalf("NewServiceMetrics: %v", err)
	}
	if got.Operations == nil {
		t.Error("Operations instrument is nil")
	}
	if got.Duration == nil {
		t.Error("Duration instrument is nil")
	}
	if got.Items == nil {
		t.Error("Items instrument is nil")
	}
}
