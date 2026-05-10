package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func sampleBenchmark(name string) *Benchmark {
	return &Benchmark{
		ID:          name,
		Timestamp:   time.Date(2026, 5, 10, 14, 30, 0, 0, time.UTC),
		Target:      "localhost:50051",
		Call:        "testpkg.Greeter/SayHello",
		TotalCount:  1000,
		ErrorCount:  5,
		Duration:    30 * time.Second,
		Concurrency: 10,
		AvgLatency:  15 * time.Millisecond,
		MinLatency:  2 * time.Millisecond,
		MaxLatency:  250 * time.Millisecond,
		P50Latency:  12 * time.Millisecond,
		P95Latency:  45 * time.Millisecond,
		P99Latency:  120 * time.Millisecond,
		RPS:         33.33,
	}
}

func TestJSONStore_Save(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore(dir)

	b := sampleBenchmark("run-1")
	path, err := store.Save(b)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("saved file does not exist: %s", path)
	}
}

func TestJSONStore_Save_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "benchmarks")
	store := NewJSONStore(dir)

	b := sampleBenchmark("run-1")
	_, err := store.Save(b)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}

func TestJSONStore_Load(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore(dir)

	original := sampleBenchmark("run-1")
	path, err := store.Save(original)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.ID != original.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, original.ID)
	}
	if loaded.Target != original.Target {
		t.Errorf("Target = %q, want %q", loaded.Target, original.Target)
	}
	if loaded.TotalCount != original.TotalCount {
		t.Errorf("TotalCount = %d, want %d", loaded.TotalCount, original.TotalCount)
	}
	if loaded.AvgLatency != original.AvgLatency {
		t.Errorf("AvgLatency = %v, want %v", loaded.AvgLatency, original.AvgLatency)
	}
	if loaded.RPS != original.RPS {
		t.Errorf("RPS = %f, want %f", loaded.RPS, original.RPS)
	}
}

func TestJSONStore_Load_NotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore(dir)

	_, err := store.Load(filepath.Join(dir, "nonexistent.json"))
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestJSONStore_List(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore(dir)

	store.Save(sampleBenchmark("run-1"))
	store.Save(sampleBenchmark("run-2"))
	store.Save(sampleBenchmark("run-3"))

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestJSONStore_List_Empty(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore(dir)

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestCompare(t *testing.T) {
	baseline := sampleBenchmark("baseline")
	baseline.AvgLatency = 15 * time.Millisecond
	baseline.P99Latency = 120 * time.Millisecond
	baseline.RPS = 33.33
	baseline.ErrorCount = 5

	current := sampleBenchmark("current")
	current.AvgLatency = 20 * time.Millisecond
	current.P99Latency = 150 * time.Millisecond
	current.RPS = 28.0
	current.ErrorCount = 10

	result := Compare(baseline, current)

	if result.AvgLatencyDelta <= 0 {
		t.Errorf("expected positive avg latency delta, got %v", result.AvgLatencyDelta)
	}
	if result.P99LatencyDelta <= 0 {
		t.Errorf("expected positive P99 latency delta, got %v", result.P99LatencyDelta)
	}
	if result.RPSDelta >= 0 {
		t.Errorf("expected negative RPS delta (regression), got %v", result.RPSDelta)
	}
	if result.ErrorDelta <= 0 {
		t.Errorf("expected positive error delta, got %d", result.ErrorDelta)
	}
}

func TestCompare_Improvement(t *testing.T) {
	baseline := sampleBenchmark("baseline")
	baseline.AvgLatency = 20 * time.Millisecond
	baseline.RPS = 25.0

	current := sampleBenchmark("current")
	current.AvgLatency = 10 * time.Millisecond
	current.RPS = 40.0

	result := Compare(baseline, current)

	if result.AvgLatencyDelta >= 0 {
		t.Errorf("expected negative avg latency delta (improvement), got %v", result.AvgLatencyDelta)
	}
	if result.RPSDelta <= 0 {
		t.Errorf("expected positive RPS delta (improvement), got %v", result.RPSDelta)
	}
}

func TestCompare_Identical(t *testing.T) {
	a := sampleBenchmark("a")
	b := sampleBenchmark("b")
	// Make them identical
	b.AvgLatency = a.AvgLatency
	b.P99Latency = a.P99Latency
	b.RPS = a.RPS
	b.ErrorCount = a.ErrorCount

	result := Compare(a, b)

	if result.AvgLatencyDelta != 0 {
		t.Errorf("expected 0 avg latency delta, got %v", result.AvgLatencyDelta)
	}
	if result.RPSDelta != 0 {
		t.Errorf("expected 0 RPS delta, got %v", result.RPSDelta)
	}
}

func TestCompareResult_Summary(t *testing.T) {
	result := &CompareResult{
		BaselineID:      "baseline",
		CurrentID:       "current",
		AvgLatencyDelta: 5 * time.Millisecond,
		P99LatencyDelta: 30 * time.Millisecond,
		RPSDelta:        -5.33,
		ErrorDelta:      5,
	}

	summary := result.Summary()
	if summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestJSONStore_FilenameContainsTimestamp(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore(dir)

	b := sampleBenchmark("run-1")
	path, err := store.Save(b)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	filename := filepath.Base(path)
	if len(filename) < 10 {
		t.Errorf("filename %q seems too short to contain timestamp", filename)
	}
}
