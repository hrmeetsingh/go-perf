package report

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleData() *BenchmarkData {
	return &BenchmarkData{
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
		StatusCodes: map[string]int{
			"OK":          995,
			"UNAVAILABLE": 5,
		},
		PayloadGroups: []PayloadGroupData{
			{
				PayloadHash: "abc123",
				SamplePayload: map[string]interface{}{
					"name": "alice",
					"age":  30,
				},
				Count:      500,
				AvgLatency: 10 * time.Millisecond,
				MaxLatency: 50 * time.Millisecond,
				P99Latency: 40 * time.Millisecond,
			},
			{
				PayloadHash: "def456",
				SamplePayload: map[string]interface{}{
					"name": "bob",
					"age":  99,
				},
				Count:      500,
				AvgLatency: 20 * time.Millisecond,
				MaxLatency: 250 * time.Millisecond,
				P99Latency: 200 * time.Millisecond,
			},
		},
	}
}

func TestCLIReporter_Write(t *testing.T) {
	var buf bytes.Buffer
	r := NewCLIReporter(&buf)
	data := sampleData()

	if err := r.Write(data); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	output := buf.String()

	requiredStrings := []string{
		"localhost:50051",
		"testpkg.Greeter/SayHello",
		"1000",
		"abc123",
		"def456",
	}
	for _, s := range requiredStrings {
		if !strings.Contains(output, s) {
			t.Errorf("CLI output missing %q", s)
		}
	}
}

func TestCLIReporter_Write_NilData(t *testing.T) {
	var buf bytes.Buffer
	r := NewCLIReporter(&buf)
	err := r.Write(nil)
	if err == nil {
		t.Error("expected error for nil data")
	}
}

func TestJSONReporter_Write(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSONReporter(&buf)
	data := sampleData()

	if err := r.Write(data); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed["target"] != "localhost:50051" {
		t.Errorf("target = %v", parsed["target"])
	}
	if parsed["call"] != "testpkg.Greeter/SayHello" {
		t.Errorf("call = %v", parsed["call"])
	}
}

func TestJSONReporter_Write_NilData(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSONReporter(&buf)
	err := r.Write(nil)
	if err == nil {
		t.Error("expected error for nil data")
	}
}

func TestHTMLReporter_Write(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "report.html")

	r := NewHTMLReporter(outPath, defaultTemplate())
	data := sampleData()

	if err := r.Write(data); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	html := string(content)
	if !strings.Contains(html, "localhost:50051") {
		t.Error("HTML missing target")
	}
	if !strings.Contains(html, "testpkg.Greeter/SayHello") {
		t.Error("HTML missing call")
	}
}

func TestHTMLReporter_Write_NilData(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "report.html")
	r := NewHTMLReporter(outPath, defaultTemplate())
	err := r.Write(nil)
	if err == nil {
		t.Error("expected error for nil data")
	}
}

func TestJUnitReporter_Write(t *testing.T) {
	var buf bytes.Buffer
	r := NewJUnitReporter(&buf)
	data := sampleData()

	if err := r.Write(data); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Validate it's valid XML
	var ts JUnitTestSuites
	if err := xml.Unmarshal(buf.Bytes(), &ts); err != nil {
		t.Fatalf("output is not valid JUnit XML: %v", err)
	}

	if len(ts.Suites) == 0 {
		t.Fatal("expected at least one test suite")
	}

	suite := ts.Suites[0]
	if suite.Name != "testpkg.Greeter/SayHello" {
		t.Errorf("suite name = %q", suite.Name)
	}
	if suite.Tests == 0 {
		t.Error("expected tests > 0")
	}
}

func TestJUnitReporter_Write_WithErrors(t *testing.T) {
	var buf bytes.Buffer
	r := NewJUnitReporter(&buf)
	data := sampleData()
	data.ErrorCount = 50

	if err := r.Write(data); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	var ts JUnitTestSuites
	if err := xml.Unmarshal(buf.Bytes(), &ts); err != nil {
		t.Fatalf("output is not valid JUnit XML: %v", err)
	}

	if ts.Suites[0].Failures == 0 {
		t.Error("expected failures > 0 when error count > 0")
	}
}

func TestMultiReporter_Write(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	r1 := NewCLIReporter(&buf1)
	r2 := NewJSONReporter(&buf2)

	multi := NewMultiReporter(r1, r2)
	data := sampleData()

	if err := multi.Write(data); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if buf1.Len() == 0 {
		t.Error("CLI reporter produced no output")
	}
	if buf2.Len() == 0 {
		t.Error("JSON reporter produced no output")
	}
}

// MultiCallBenchmarkData tests

func sampleMultiCallData() *MultiCallBenchmarkData {
	return &MultiCallBenchmarkData{
		Timestamp: time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC),
		Target:    "localhost:50051",
		Calls: []BenchmarkData{
			*sampleData(),
			{
				Timestamp:   time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC),
				Target:      "localhost:50051",
				Call:        "testpkg.Greeter/SayBye",
				TotalCount:  500,
				ErrorCount:  0,
				Duration:    10 * time.Second,
				Concurrency: 10,
				AvgLatency:  5 * time.Millisecond,
				RPS:         50.0,
				StatusCodes: map[string]int{"OK": 500},
			},
		},
	}
}

func TestCLIReporter_WriteMultiCall(t *testing.T) {
	var buf bytes.Buffer
	r := NewCLIReporter(&buf)

	data := sampleMultiCallData()
	if err := r.WriteMultiCall(data); err != nil {
		t.Fatalf("WriteMultiCall() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "SayHello") {
		t.Error("expected first call name in output")
	}
	if !strings.Contains(output, "SayBye") {
		t.Error("expected second call name in output")
	}
}

func TestJSONReporter_WriteMultiCall(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSONReporter(&buf)

	data := sampleMultiCallData()
	if err := r.WriteMultiCall(data); err != nil {
		t.Fatalf("WriteMultiCall() error = %v", err)
	}

	var decoded MultiCallBenchmarkData
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(decoded.Calls) != 2 {
		t.Errorf("decoded Calls count = %d, want 2", len(decoded.Calls))
	}
}

func TestMultiCallBenchmarkData_CallCount(t *testing.T) {
	data := sampleMultiCallData()
	if len(data.Calls) != 2 {
		t.Errorf("Calls count = %d, want 2", len(data.Calls))
	}
}
