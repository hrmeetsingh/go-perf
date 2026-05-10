package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/bojand/ghz/runner"
)

// GhzEngine implements Engine using the ghz benchmarking library.
type GhzEngine struct{}

func NewGhzEngine() *GhzEngine {
	return &GhzEngine{}
}

func (e *GhzEngine) Run(ctx context.Context, call, target string, cfg RunConfig, variant VariantConfig, authMeta map[string]string) (*Result, error) {
	// Pass the map directly — ghz marshals it internally.
	// Passing pre-marshaled []byte would cause ghz to base64-encode it again.
	data := variant.Payload
	if data == nil {
		data = map[string]interface{}{}
	}

	opts := []runner.Option{
		runner.WithProtoFile(cfg.ProtoPath, []string{}),
		runner.WithData(data),
		runner.WithInsecure(cfg.Insecure),
	}

	if cfg.Concurrency > 0 {
		opts = append(opts, runner.WithConcurrency(uint(cfg.Concurrency)))
	}
	if cfg.Total > 0 {
		opts = append(opts, runner.WithTotalRequests(uint(cfg.Total)))
	}
	if cfg.Connections > 0 {
		opts = append(opts, runner.WithConnections(uint(cfg.Connections)))
	}
	if cfg.Duration > 0 {
		opts = append(opts, runner.WithRunDuration(cfg.Duration))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, runner.WithTimeout(cfg.Timeout))
	}

	if len(authMeta) > 0 {
		opts = append(opts, runner.WithMetadata(authMeta))
	}
	if len(cfg.Metadata) > 0 {
		opts = append(opts, runner.WithMetadata(cfg.Metadata))
	}

	report, err := runner.Run(call, target, opts...)
	if err != nil {
		return nil, fmt.Errorf("ghz run: %w", err)
	}

	result := &Result{
		VariantName: variant.Name,
		TotalCount:  int(report.Count),
		Duration:    report.Total,
		RPS:         report.Rps,
		StartTime:   report.Date,
		EndTime:     report.Date.Add(report.Total),
	}

	payloadHash := variant.Name
	for _, detail := range report.Details {
		record := LatencyRecord{
			Duration:    detail.Latency,
			PayloadHash: payloadHash,
			StatusCode:  detail.Status,
			Timestamp:   detail.Timestamp,
		}
		if detail.Error != "" {
			result.ErrorCount++
		}
		result.Latencies = append(result.Latencies, record)
	}

	return result, nil
}

// StreamRunConfig holds configuration for bidirectional streaming benchmarks.
type StreamRunConfig struct {
	SendRate    int
	StreamCount int
	Duration    time.Duration
}
