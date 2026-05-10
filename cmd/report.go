package cmd

import (
	"fmt"
	"os"

	"github.com/hrmeetsingh/go-perf/internal/report"
	"github.com/hrmeetsingh/go-perf/internal/storage"
	"github.com/spf13/cobra"
)

var reportCmd = &cobra.Command{
	Use:   "report [benchmark-file]",
	Short: "Regenerate reports from stored benchmark data",
	Long:  "Load a stored benchmark JSON file and regenerate reports in the specified formats.",
	Args:  cobra.ExactArgs(1),
	RunE:  runReport,
}

var (
	reportFormats   []string
	reportOutputDir string
)

func init() {
	reportCmd.Flags().StringSliceVar(&reportFormats, "format", []string{"cli"}, "output formats: cli,html,json,junit")
	reportCmd.Flags().StringVar(&reportOutputDir, "output-dir", "./reports", "directory for report output")
	rootCmd.AddCommand(reportCmd)
}

func runReport(cmd *cobra.Command, args []string) error {
	store := storage.NewJSONStore("")
	b, err := store.Load(args[0])
	if err != nil {
		return fmt.Errorf("loading benchmark: %w", err)
	}

	data := storageToBenchmarkData(b)

	reporters, err := report.ReportersFromFormats(reportFormats, reportOutputDir)
	if err != nil {
		return err
	}
	multi := report.NewMultiReporter(reporters...)
	return multi.Write(data)
}

func storageToBenchmarkData(b *storage.Benchmark) *report.BenchmarkData {
	return &report.BenchmarkData{
		Timestamp:   b.Timestamp,
		Target:      b.Target,
		Call:        b.Call,
		TotalCount:  b.TotalCount,
		ErrorCount:  b.ErrorCount,
		Duration:    b.Duration,
		Concurrency: b.Concurrency,
		AvgLatency:  b.AvgLatency,
		MinLatency:  b.MinLatency,
		MaxLatency:  b.MaxLatency,
		P50Latency:  b.P50Latency,
		P95Latency:  b.P95Latency,
		P99Latency:  b.P99Latency,
		RPS:         b.RPS,
	}
}

func init() {
	_ = os.Stdout
}
