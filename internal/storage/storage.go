package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BenchmarkRun holds the results from a single execution of go-perf, which may
// include multiple RPC calls run sequentially or in parallel.
type BenchmarkRun struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	Calls     []Benchmark `json:"calls"`
}

type Benchmark struct {
	ID          string        `json:"id"`
	Timestamp   time.Time     `json:"timestamp"`
	Target      string        `json:"target"`
	Call        string        `json:"call"`
	TotalCount  int           `json:"total_count"`
	ErrorCount  int           `json:"error_count"`
	Duration    time.Duration `json:"duration"`
	Concurrency int           `json:"concurrency"`
	AvgLatency  time.Duration `json:"avg_latency"`
	MinLatency  time.Duration `json:"min_latency"`
	MaxLatency  time.Duration `json:"max_latency"`
	P50Latency  time.Duration `json:"p50_latency"`
	P95Latency  time.Duration `json:"p95_latency"`
	P99Latency  time.Duration `json:"p99_latency"`
	RPS         float64       `json:"rps"`
}

type BenchmarkEntry struct {
	Path      string
	ID        string
	Timestamp time.Time
}

type CompareResult struct {
	BaselineID      string
	CurrentID       string
	AvgLatencyDelta time.Duration
	P99LatencyDelta time.Duration
	RPSDelta        float64
	ErrorDelta      int
}

type JSONStore struct {
	dir string
}

func NewJSONStore(dir string) *JSONStore {
	return &JSONStore{dir: dir}
}

// SaveRun persists a multi-call BenchmarkRun as a single JSON file.
func (s *JSONStore) SaveRun(run *BenchmarkRun) (string, error) {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return "", fmt.Errorf("creating storage directory: %w", err)
	}

	ts := run.Timestamp.Format("20060102T150405")
	safeID := strings.ReplaceAll(run.ID, "/", "_")
	filename := fmt.Sprintf("run_%s_%s.json", ts, safeID)
	path := filepath.Join(s.dir, filename)

	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling benchmark run: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("writing benchmark run file: %w", err)
	}

	return path, nil
}

// LoadRun reads a BenchmarkRun from a JSON file.
func (s *JSONStore) LoadRun(path string) (*BenchmarkRun, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading benchmark run file: %w", err)
	}

	var run BenchmarkRun
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("parsing benchmark run: %w", err)
	}

	return &run, nil
}

func (s *JSONStore) Save(b *Benchmark) (string, error) {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return "", fmt.Errorf("creating storage directory: %w", err)
	}

	ts := b.Timestamp.Format("20060102T150405")
	safeID := strings.ReplaceAll(b.ID, "/", "_")
	filename := fmt.Sprintf("%s_%s.json", ts, safeID)
	path := filepath.Join(s.dir, filename)

	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling benchmark: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("writing benchmark file: %w", err)
	}

	return path, nil
}

func (s *JSONStore) Load(path string) (*Benchmark, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading benchmark file: %w", err)
	}

	var b Benchmark
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parsing benchmark: %w", err)
	}

	return &b, nil
}

func (s *JSONStore) List() ([]BenchmarkEntry, error) {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return nil, fmt.Errorf("creating storage directory: %w", err)
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("listing benchmark directory: %w", err)
	}

	var results []BenchmarkEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.dir, e.Name())
		b, err := s.Load(path)
		if err != nil {
			continue
		}

		results = append(results, BenchmarkEntry{
			Path:      path,
			ID:        b.ID,
			Timestamp: b.Timestamp,
		})
	}

	return results, nil
}

func Compare(baseline, current *Benchmark) *CompareResult {
	return &CompareResult{
		BaselineID:      baseline.ID,
		CurrentID:       current.ID,
		AvgLatencyDelta: current.AvgLatency - baseline.AvgLatency,
		P99LatencyDelta: current.P99Latency - baseline.P99Latency,
		RPSDelta:        current.RPS - baseline.RPS,
		ErrorDelta:      current.ErrorCount - baseline.ErrorCount,
	}
}

func (cr *CompareResult) Summary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Comparison: %s vs %s\n", cr.BaselineID, cr.CurrentID))

	if cr.AvgLatencyDelta > 0 {
		sb.WriteString(fmt.Sprintf("  Avg Latency: +%s (regression)\n", cr.AvgLatencyDelta))
	} else if cr.AvgLatencyDelta < 0 {
		sb.WriteString(fmt.Sprintf("  Avg Latency: %s (improvement)\n", cr.AvgLatencyDelta))
	} else {
		sb.WriteString("  Avg Latency: no change\n")
	}

	if cr.P99LatencyDelta > 0 {
		sb.WriteString(fmt.Sprintf("  P99 Latency: +%s (regression)\n", cr.P99LatencyDelta))
	} else if cr.P99LatencyDelta < 0 {
		sb.WriteString(fmt.Sprintf("  P99 Latency: %s (improvement)\n", cr.P99LatencyDelta))
	} else {
		sb.WriteString("  P99 Latency: no change\n")
	}

	if cr.RPSDelta > 0 {
		sb.WriteString(fmt.Sprintf("  RPS: +%.2f (improvement)\n", cr.RPSDelta))
	} else if cr.RPSDelta < 0 {
		sb.WriteString(fmt.Sprintf("  RPS: %.2f (regression)\n", cr.RPSDelta))
	} else {
		sb.WriteString("  RPS: no change\n")
	}

	if cr.ErrorDelta > 0 {
		sb.WriteString(fmt.Sprintf("  Errors: +%d (regression)\n", cr.ErrorDelta))
	} else if cr.ErrorDelta < 0 {
		sb.WriteString(fmt.Sprintf("  Errors: %d (improvement)\n", cr.ErrorDelta))
	} else {
		sb.WriteString("  Errors: no change\n")
	}

	return sb.String()
}
