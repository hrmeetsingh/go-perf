package runner

import (
	"testing"
	"time"
)

func TestNewVariantConfig(t *testing.T) {
	vc := NewVariantConfig(
		"variant-small",
		map[string]interface{}{"size": "small", "count": 1},
	)

	if vc.Name != "variant-small" {
		t.Errorf("Name = %q, want %q", vc.Name, "variant-small")
	}
	if vc.Payload == nil {
		t.Fatal("expected non-nil Payload")
	}
	if vc.Payload["size"] != "small" {
		t.Errorf("Payload[size] = %v, want small", vc.Payload["size"])
	}
}

func TestRunConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     RunConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: RunConfig{
				Target:      "localhost:50051",
				Call:        "pkg.Svc/Method",
				ProtoPath:   "service.proto",
				Concurrency: 10,
				Total:       100,
			},
			wantErr: false,
		},
		{
			name: "missing target",
			cfg: RunConfig{
				Call:      "pkg.Svc/Method",
				ProtoPath: "service.proto",
			},
			wantErr: true,
		},
		{
			name: "missing call",
			cfg: RunConfig{
				Target:    "localhost:50051",
				ProtoPath: "service.proto",
			},
			wantErr: true,
		},
		{
			name: "missing proto",
			cfg: RunConfig{
				Target: "localhost:50051",
				Call:   "pkg.Svc/Method",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestResult_LatencyStats(t *testing.T) {
	r := &Result{
		Latencies: []LatencyRecord{
			{Duration: 10 * time.Millisecond, PayloadHash: "aaa"},
			{Duration: 20 * time.Millisecond, PayloadHash: "bbb"},
			{Duration: 30 * time.Millisecond, PayloadHash: "aaa"},
			{Duration: 100 * time.Millisecond, PayloadHash: "ccc"},
			{Duration: 15 * time.Millisecond, PayloadHash: "bbb"},
		},
	}

	stats := r.LatencyStats()

	if stats.Min != 10*time.Millisecond {
		t.Errorf("Min = %v, want 10ms", stats.Min)
	}
	if stats.Max != 100*time.Millisecond {
		t.Errorf("Max = %v, want 100ms", stats.Max)
	}
	if stats.Count != 5 {
		t.Errorf("Count = %d, want 5", stats.Count)
	}
	// Average: (10+20+30+100+15)/5 = 35ms
	expectedAvg := 35 * time.Millisecond
	if stats.Average != expectedAvg {
		t.Errorf("Average = %v, want %v", stats.Average, expectedAvg)
	}
}

func TestResult_LatencyByPayloadHash(t *testing.T) {
	r := &Result{
		Latencies: []LatencyRecord{
			{Duration: 10 * time.Millisecond, PayloadHash: "aaa"},
			{Duration: 20 * time.Millisecond, PayloadHash: "bbb"},
			{Duration: 30 * time.Millisecond, PayloadHash: "aaa"},
			{Duration: 100 * time.Millisecond, PayloadHash: "ccc"},
		},
	}

	groups := r.LatencyByPayloadHash()

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if len(groups["aaa"]) != 2 {
		t.Errorf("group aaa has %d records, want 2", len(groups["aaa"]))
	}
	if len(groups["bbb"]) != 1 {
		t.Errorf("group bbb has %d records, want 1", len(groups["bbb"]))
	}
}

func TestResult_StatusCodeCounts(t *testing.T) {
	r := &Result{
		Latencies: []LatencyRecord{
			{StatusCode: "OK"},
			{StatusCode: "OK"},
			{StatusCode: "UNAVAILABLE"},
			{StatusCode: "OK"},
			{StatusCode: "DEADLINE_EXCEEDED"},
		},
	}

	counts := r.StatusCodeCounts()
	if counts["OK"] != 3 {
		t.Errorf("OK count = %d, want 3", counts["OK"])
	}
	if counts["UNAVAILABLE"] != 1 {
		t.Errorf("UNAVAILABLE count = %d, want 1", counts["UNAVAILABLE"])
	}
	if counts["DEADLINE_EXCEEDED"] != 1 {
		t.Errorf("DEADLINE_EXCEEDED count = %d, want 1", counts["DEADLINE_EXCEEDED"])
	}
}

func TestMultiVariantResult_Merge(t *testing.T) {
	r1 := &Result{
		VariantName: "small",
		TotalCount:  100,
		ErrorCount:  5,
		Latencies: []LatencyRecord{
			{Duration: 10 * time.Millisecond, PayloadHash: "aaa"},
		},
	}
	r2 := &Result{
		VariantName: "large",
		TotalCount:  200,
		ErrorCount:  10,
		Latencies: []LatencyRecord{
			{Duration: 50 * time.Millisecond, PayloadHash: "bbb"},
		},
	}

	mvr := &MultiVariantResult{
		Results: []*Result{r1, r2},
	}

	merged := mvr.Merge()
	if merged.TotalCount != 300 {
		t.Errorf("merged TotalCount = %d, want 300", merged.TotalCount)
	}
	if merged.ErrorCount != 15 {
		t.Errorf("merged ErrorCount = %d, want 15", merged.ErrorCount)
	}
	if len(merged.Latencies) != 2 {
		t.Errorf("merged Latencies count = %d, want 2", len(merged.Latencies))
	}
}

func TestOrchestratorConfig_Validate(t *testing.T) {
	cfg := OrchestratorConfig{
		RunConfig: RunConfig{
			Target:      "localhost:50051",
			Call:        "pkg.Svc/Method",
			ProtoPath:   "service.proto",
			Concurrency: 10,
			Total:       100,
		},
		Variants: []VariantConfig{
			NewVariantConfig("v1", map[string]interface{}{"key": "val"}),
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestOrchestratorConfig_Validate_NoVariants(t *testing.T) {
	cfg := OrchestratorConfig{
		RunConfig: RunConfig{
			Target:      "localhost:50051",
			Call:        "pkg.Svc/Method",
			ProtoPath:   "service.proto",
			Concurrency: 10,
			Total:       100,
		},
		Variants: []VariantConfig{},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for empty variants")
	}
}
