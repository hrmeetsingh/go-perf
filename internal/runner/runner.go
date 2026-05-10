package runner

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// Engine abstracts the benchmark execution backend, enabling mock-based testing
// of orchestrator logic without a live gRPC server.
type Engine interface {
	Run(ctx context.Context, call, target string, cfg RunConfig, variant VariantConfig, authMeta map[string]string) (*Result, error)
}

type RunConfig struct {
	Target      string
	Call        string
	ProtoPath   string
	Concurrency int
	Total       int
	Duration    time.Duration
	Connections int
	Timeout     time.Duration
	Metadata    map[string]string
	Insecure    bool
}

type VariantConfig struct {
	Name    string
	Payload map[string]interface{}
}

func NewVariantConfig(name string, payload map[string]interface{}) VariantConfig {
	return VariantConfig{
		Name:    name,
		Payload: payload,
	}
}

type LatencyRecord struct {
	Duration    time.Duration
	PayloadHash string
	StatusCode  string
	Timestamp   time.Time
}

type LatencyStats struct {
	Min     time.Duration
	Max     time.Duration
	Average time.Duration
	P50     time.Duration
	P95     time.Duration
	P99     time.Duration
	Count   int
}

type Result struct {
	VariantName string
	TotalCount  int
	ErrorCount  int
	Duration    time.Duration
	Latencies   []LatencyRecord
	RPS         float64
	StartTime   time.Time
	EndTime     time.Time
}

type MultiVariantResult struct {
	Results []*Result
}

type OrchestratorConfig struct {
	RunConfig RunConfig
	Variants  []VariantConfig
}

func (rc *RunConfig) Validate() error {
	if rc.Target == "" {
		return fmt.Errorf("target is required")
	}
	if rc.Call == "" {
		return fmt.Errorf("call is required")
	}
	if rc.ProtoPath == "" {
		return fmt.Errorf("proto path is required")
	}
	return nil
}

func (oc *OrchestratorConfig) Validate() error {
	if err := oc.RunConfig.Validate(); err != nil {
		return err
	}
	if len(oc.Variants) == 0 {
		return fmt.Errorf("at least one variant is required")
	}
	return nil
}

func (r *Result) LatencyStats() LatencyStats {
	if len(r.Latencies) == 0 {
		return LatencyStats{}
	}

	durations := make([]time.Duration, len(r.Latencies))
	var total time.Duration
	for i, l := range r.Latencies {
		durations[i] = l.Duration
		total += l.Duration
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	n := len(durations)
	return LatencyStats{
		Min:     durations[0],
		Max:     durations[n-1],
		Average: total / time.Duration(n),
		P50:     percentileFromSorted(durations, 0.50),
		P95:     percentileFromSorted(durations, 0.95),
		P99:     percentileFromSorted(durations, 0.99),
		Count:   n,
	}
}

// percentileFromSorted returns the value at the given percentile from a
// pre-sorted slice of durations.
func percentileFromSorted(sorted []time.Duration, pct float64) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	idx := int(float64(n) * pct)
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

func (r *Result) LatencyByPayloadHash() map[string][]LatencyRecord {
	groups := make(map[string][]LatencyRecord)
	for _, l := range r.Latencies {
		groups[l.PayloadHash] = append(groups[l.PayloadHash], l)
	}
	return groups
}

func (r *Result) StatusCodeCounts() map[string]int {
	counts := make(map[string]int)
	for _, l := range r.Latencies {
		counts[l.StatusCode]++
	}
	return counts
}

func (mvr *MultiVariantResult) Merge() *Result {
	merged := &Result{}
	for _, r := range mvr.Results {
		merged.TotalCount += r.TotalCount
		merged.ErrorCount += r.ErrorCount
		merged.Latencies = append(merged.Latencies, r.Latencies...)
	}
	return merged
}
