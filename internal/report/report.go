package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"
	"time"
)

type Reporter interface {
	Write(data *BenchmarkData) error
}

type BenchmarkData struct {
	Timestamp     time.Time
	Target        string
	Call          string
	TotalCount    int
	ErrorCount    int
	Duration      time.Duration
	Concurrency   int
	AvgLatency    time.Duration
	MinLatency    time.Duration
	MaxLatency    time.Duration
	P50Latency    time.Duration
	P95Latency    time.Duration
	P99Latency    time.Duration
	RPS           float64
	StatusCodes   map[string]int
	PayloadGroups []PayloadGroupData
}

type PayloadGroupData struct {
	PayloadHash   string
	SamplePayload map[string]interface{}
	Count         int
	AvgLatency    time.Duration
	MaxLatency    time.Duration
	P99Latency    time.Duration
}

// --- CLI Reporter ---

type cliReporter struct {
	w io.Writer
}

func NewCLIReporter(w io.Writer) Reporter {
	return &cliReporter{w: w}
}

func (r *cliReporter) Write(data *BenchmarkData) error {
	if data == nil {
		return fmt.Errorf("benchmark data is nil")
	}

	fmt.Fprintf(r.w, "\n%s\n", strings.Repeat("=", 60))
	fmt.Fprintf(r.w, "  gRPC Benchmark Report\n")
	fmt.Fprintf(r.w, "%s\n\n", strings.Repeat("=", 60))

	fmt.Fprintf(r.w, "  Target:       %s\n", data.Target)
	fmt.Fprintf(r.w, "  Call:         %s\n", data.Call)
	fmt.Fprintf(r.w, "  Timestamp:    %s\n", data.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(r.w, "  Duration:     %s\n", data.Duration)
	fmt.Fprintf(r.w, "  Concurrency:  %d\n", data.Concurrency)
	fmt.Fprintf(r.w, "\n")

	fmt.Fprintf(r.w, "  Total Requests:  %d\n", data.TotalCount)
	fmt.Fprintf(r.w, "  Errors:          %d (%.2f%%)\n", data.ErrorCount, errorRate(data))
	fmt.Fprintf(r.w, "  RPS:             %.2f\n", data.RPS)
	fmt.Fprintf(r.w, "\n")

	fmt.Fprintf(r.w, "  Latency:\n")
	fmt.Fprintf(r.w, "    Min:    %s\n", data.MinLatency)
	fmt.Fprintf(r.w, "    Avg:    %s\n", data.AvgLatency)
	fmt.Fprintf(r.w, "    P50:    %s\n", data.P50Latency)
	fmt.Fprintf(r.w, "    P95:    %s\n", data.P95Latency)
	fmt.Fprintf(r.w, "    P99:    %s\n", data.P99Latency)
	fmt.Fprintf(r.w, "    Max:    %s\n", data.MaxLatency)
	fmt.Fprintf(r.w, "\n")

	if len(data.StatusCodes) > 0 {
		fmt.Fprintf(r.w, "  Status Codes:\n")
		for code, count := range data.StatusCodes {
			fmt.Fprintf(r.w, "    %-20s %d\n", code, count)
		}
		fmt.Fprintf(r.w, "\n")
	}

	if len(data.PayloadGroups) > 0 {
		fmt.Fprintf(r.w, "  Payload Variant Breakdown:\n")
		fmt.Fprintf(r.w, "  %-16s %-8s %-12s %-12s %-12s\n", "Hash", "Count", "Avg", "P99", "Max")
		fmt.Fprintf(r.w, "  %s\n", strings.Repeat("-", 60))
		for _, pg := range data.PayloadGroups {
			fmt.Fprintf(r.w, "  %-16s %-8d %-12s %-12s %-12s\n",
				pg.PayloadHash, pg.Count, pg.AvgLatency, pg.P99Latency, pg.MaxLatency)
		}
		fmt.Fprintf(r.w, "\n")
	}

	return nil
}

func errorRate(data *BenchmarkData) float64 {
	if data.TotalCount == 0 {
		return 0
	}
	return float64(data.ErrorCount) / float64(data.TotalCount) * 100
}

// --- JSON Reporter ---

type jsonReporter struct {
	w io.Writer
}

func NewJSONReporter(w io.Writer) Reporter {
	return &jsonReporter{w: w}
}

type jsonOutput struct {
	Timestamp    string                 `json:"timestamp"`
	Target       string                 `json:"target"`
	Call         string                 `json:"call"`
	TotalCount   int                    `json:"total_count"`
	ErrorCount   int                    `json:"error_count"`
	DurationMs   int64                  `json:"duration_ms"`
	Concurrency  int                    `json:"concurrency"`
	AvgLatencyMs float64                `json:"avg_latency_ms"`
	MinLatencyMs float64                `json:"min_latency_ms"`
	MaxLatencyMs float64                `json:"max_latency_ms"`
	P50LatencyMs float64                `json:"p50_latency_ms"`
	P95LatencyMs float64                `json:"p95_latency_ms"`
	P99LatencyMs float64                `json:"p99_latency_ms"`
	RPS          float64                `json:"rps"`
	StatusCodes  map[string]int         `json:"status_codes"`
	Payloads     []jsonPayloadGroup     `json:"payload_groups,omitempty"`
}

type jsonPayloadGroup struct {
	Hash         string                 `json:"hash"`
	Sample       map[string]interface{} `json:"sample_payload"`
	Count        int                    `json:"count"`
	AvgLatencyMs float64                `json:"avg_latency_ms"`
	MaxLatencyMs float64                `json:"max_latency_ms"`
	P99LatencyMs float64                `json:"p99_latency_ms"`
}

func (r *jsonReporter) Write(data *BenchmarkData) error {
	if data == nil {
		return fmt.Errorf("benchmark data is nil")
	}

	out := jsonOutput{
		Timestamp:    data.Timestamp.Format(time.RFC3339),
		Target:       data.Target,
		Call:         data.Call,
		TotalCount:   data.TotalCount,
		ErrorCount:   data.ErrorCount,
		DurationMs:   data.Duration.Milliseconds(),
		Concurrency:  data.Concurrency,
		AvgLatencyMs: msFromDuration(data.AvgLatency),
		MinLatencyMs: msFromDuration(data.MinLatency),
		MaxLatencyMs: msFromDuration(data.MaxLatency),
		P50LatencyMs: msFromDuration(data.P50Latency),
		P95LatencyMs: msFromDuration(data.P95Latency),
		P99LatencyMs: msFromDuration(data.P99Latency),
		RPS:          data.RPS,
		StatusCodes:  data.StatusCodes,
	}

	for _, pg := range data.PayloadGroups {
		out.Payloads = append(out.Payloads, jsonPayloadGroup{
			Hash:         pg.PayloadHash,
			Sample:       pg.SamplePayload,
			Count:        pg.Count,
			AvgLatencyMs: msFromDuration(pg.AvgLatency),
			MaxLatencyMs: msFromDuration(pg.MaxLatency),
			P99LatencyMs: msFromDuration(pg.P99Latency),
		})
	}

	enc := json.NewEncoder(r.w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func msFromDuration(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

// --- HTML Reporter ---

type htmlReporter struct {
	outputPath string
	tmpl       *template.Template
}

func NewHTMLReporter(outputPath string, tmpl *template.Template) Reporter {
	return &htmlReporter{outputPath: outputPath, tmpl: tmpl}
}

func (r *htmlReporter) Write(data *BenchmarkData) error {
	if data == nil {
		return fmt.Errorf("benchmark data is nil")
	}

	f, err := os.Create(r.outputPath)
	if err != nil {
		return fmt.Errorf("creating HTML file: %w", err)
	}
	defer f.Close()

	return r.tmpl.Execute(f, data)
}

func defaultTemplate() *template.Template {
	const tmplStr = `<!DOCTYPE html>
<html>
<head><title>gRPC Benchmark Report</title></head>
<body>
<h1>gRPC Benchmark Report</h1>
<p>Target: {{.Target}}</p>
<p>Call: {{.Call}}</p>
<p>Total: {{.TotalCount}}</p>
<p>Errors: {{.ErrorCount}}</p>
<p>RPS: {{printf "%.2f" .RPS}}</p>
<p>Avg Latency: {{.AvgLatency}}</p>
<p>P99 Latency: {{.P99Latency}}</p>
<h2>Payload Variants</h2>
<table>
<tr><th>Hash</th><th>Count</th><th>Avg Latency</th><th>Max Latency</th></tr>
{{range .PayloadGroups}}
<tr><td>{{.PayloadHash}}</td><td>{{.Count}}</td><td>{{.AvgLatency}}</td><td>{{.MaxLatency}}</td></tr>
{{end}}
</table>
</body>
</html>`
	return template.Must(template.New("report").Parse(tmplStr))
}

// --- JUnit Reporter ---

type JUnitTestSuites struct {
	XMLName xml.Name         `xml:"testsuites"`
	Suites  []JUnitTestSuite `xml:"testsuite"`
}

type JUnitTestSuite struct {
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Time     string          `xml:"time,attr"`
	Cases    []JUnitTestCase `xml:"testcase"`
}

type JUnitTestCase struct {
	Name    string        `xml:"name,attr"`
	Time    string        `xml:"time,attr"`
	Failure *JUnitFailure `xml:"failure,omitempty"`
}

type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
}

type junitReporter struct {
	w io.Writer
}

func NewJUnitReporter(w io.Writer) Reporter {
	return &junitReporter{w: w}
}

func (r *junitReporter) Write(data *BenchmarkData) error {
	if data == nil {
		return fmt.Errorf("benchmark data is nil")
	}

	suite := JUnitTestSuite{
		Name:     data.Call,
		Tests:    data.TotalCount,
		Failures: data.ErrorCount,
		Time:     fmt.Sprintf("%.3f", data.Duration.Seconds()),
	}

	suite.Cases = append(suite.Cases, JUnitTestCase{
		Name: "latency_avg",
		Time: fmt.Sprintf("%.6f", data.AvgLatency.Seconds()),
	})
	suite.Cases = append(suite.Cases, JUnitTestCase{
		Name: "latency_p99",
		Time: fmt.Sprintf("%.6f", data.P99Latency.Seconds()),
	})
	suite.Cases = append(suite.Cases, JUnitTestCase{
		Name: fmt.Sprintf("throughput_rps_%.2f", data.RPS),
		Time: fmt.Sprintf("%.3f", data.Duration.Seconds()),
	})

	if data.ErrorCount > 0 {
		suite.Cases = append(suite.Cases, JUnitTestCase{
			Name: "error_rate",
			Time: "0",
			Failure: &JUnitFailure{
				Message: fmt.Sprintf("%d errors out of %d requests (%.2f%%)", data.ErrorCount, data.TotalCount, errorRate(data)),
				Type:    "error_threshold",
			},
		})
	}

	for _, pg := range data.PayloadGroups {
		suite.Cases = append(suite.Cases, JUnitTestCase{
			Name: fmt.Sprintf("payload_%s_avg_latency", pg.PayloadHash),
			Time: fmt.Sprintf("%.6f", pg.AvgLatency.Seconds()),
		})
	}

	suites := JUnitTestSuites{Suites: []JUnitTestSuite{suite}}
	output, err := xml.MarshalIndent(suites, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JUnit XML: %w", err)
	}

	_, err = r.w.Write(output)
	return err
}

// --- Multi Reporter ---

type multiReporter struct {
	reporters []Reporter
}

func NewMultiReporter(reporters ...Reporter) Reporter {
	return &multiReporter{reporters: reporters}
}

func (r *multiReporter) Write(data *BenchmarkData) error {
	for _, reporter := range r.reporters {
		if err := reporter.Write(data); err != nil {
			return err
		}
	}
	return nil
}
