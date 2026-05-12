package cmd

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/hrmeetsingh/go-perf/internal/auth"
	"github.com/hrmeetsingh/go-perf/internal/config"
	"github.com/hrmeetsingh/go-perf/internal/payload"
	protolib "github.com/hrmeetsingh/go-perf/internal/proto"
	"github.com/hrmeetsingh/go-perf/internal/report"
	"github.com/hrmeetsingh/go-perf/internal/runner"
	"github.com/hrmeetsingh/go-perf/internal/storage"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a gRPC benchmark",
	Long:  "Execute a performance benchmark against a gRPC service using the specified configuration.",
	RunE:  runBenchmark,
}

var (
	flagTarget      string
	flagProto       string
	flagCall        string
	flagConcurrency int
	flagTotal       int
	flagDuration    string
	flagConnections int
	flagTimeout     string
	flagInsecure    bool
	flagToken       string
	flagOutputDir   string
	flagFormats     []string
	flagSave        bool
	flagStoreDir    string
	flagParallel    bool
)

func init() {
	runCmd.Flags().StringVar(&flagTarget, "target", "", "gRPC server target (host:port)")
	runCmd.Flags().StringVar(&flagProto, "proto", "", "path to proto file or directory")
	runCmd.Flags().StringVar(&flagCall, "call", "", "fully qualified method (pkg.Service/Method); overrides calls list to a single entry")
	runCmd.Flags().IntVar(&flagConcurrency, "concurrency", 0, "number of concurrent workers")
	runCmd.Flags().IntVar(&flagTotal, "total", 0, "total number of requests")
	runCmd.Flags().StringVar(&flagDuration, "duration", "", "benchmark duration (e.g. 30s)")
	runCmd.Flags().IntVar(&flagConnections, "connections", 0, "number of connections")
	runCmd.Flags().StringVar(&flagTimeout, "timeout", "", "request timeout (e.g. 10s)")
	runCmd.Flags().BoolVar(&flagInsecure, "insecure", true, "use insecure connection")
	runCmd.Flags().StringVar(&flagToken, "token", "", "JWT token for authentication")
	runCmd.Flags().StringVar(&flagOutputDir, "output-dir", "", "directory for report output")
	runCmd.Flags().StringSliceVar(&flagFormats, "format", nil, "output formats: cli,html,json,junit")
	runCmd.Flags().BoolVar(&flagSave, "save", false, "save benchmark results for later comparison")
	runCmd.Flags().StringVar(&flagStoreDir, "store-dir", ".go-perf/benchmarks", "directory for stored benchmarks")
	runCmd.Flags().BoolVar(&flagParallel, "parallel", false, "run multiple calls in parallel")

	rootCmd.AddCommand(runCmd)
}

// callResult pairs a CallEntry with its benchmark result.
type callResult struct {
	entry config.CallEntry
	data  *report.BenchmarkData
	bench storage.Benchmark
	err   error
}

func runBenchmark(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	config.ApplyDefaults(cfg)
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	protoFiles, err := protolib.DiscoverProtoFiles(cfg.Proto)
	if err != nil {
		return fmt.Errorf("discovering proto files: %w", err)
	}
	if len(protoFiles) == 0 {
		return fmt.Errorf("no proto files found at %q", cfg.Proto)
	}

	authMeta, err := resolveAuth(ctx, cfg)
	if err != nil {
		return err
	}

	dur, _ := time.ParseDuration(cfg.Duration)
	timeout, _ := time.ParseDuration(cfg.Timeout)

	runOne := func(entry config.CallEntry) callResult {
		variants, err := buildVariantsFromEntry(entry)
		if err != nil {
			return callResult{entry: entry, err: fmt.Errorf("building variants for %q: %w", entry.Call, err)}
		}

		runCfg := runner.RunConfig{
			Target:      cfg.Target,
			Call:        entry.Call,
			ProtoPath:   protoFiles[0],
			Concurrency: cfg.Concurrency,
			Total:       cfg.Total,
			Duration:    dur,
			Connections: cfg.Connections,
			Timeout:     timeout,
			Metadata:    cfg.Metadata,
			Insecure:    flagInsecure,
		}

		orchCfg := runner.OrchestratorConfig{
			RunConfig: runCfg,
			Variants:  variants,
		}

		engine := runner.NewGhzEngine()
		orch := runner.NewOrchestrator(orchCfg, authMeta, engine)
		multiResult, err := orch.Run(ctx)
		if err != nil {
			return callResult{entry: entry, err: fmt.Errorf("benchmark %q failed: %w", entry.Call, err)}
		}

		merged := multiResult.Merge()
		stats := merged.LatencyStats()
		benchData := buildBenchmarkData(entry.Call, cfg, merged, stats)
		bench := buildBenchmark(entry.Call, cfg, stats, merged)
		return callResult{entry: entry, data: benchData, bench: bench}
	}

	results := make([]callResult, len(cfg.Calls))

	if cfg.Parallel {
		var wg sync.WaitGroup
		for i, entry := range cfg.Calls {
			wg.Add(1)
			go func(idx int, e config.CallEntry) {
				defer wg.Done()
				results[idx] = runOne(e)
			}(i, entry)
		}
		wg.Wait()
	} else {
		for i, entry := range cfg.Calls {
			results[i] = runOne(entry)
		}
	}

	// Collect per-call benchmark data; surface first error if any.
	multiData := &report.MultiCallBenchmarkData{
		Timestamp: time.Now(),
		Target:    cfg.Target,
	}
	var benchRun storage.BenchmarkRun
	benchRun.ID = fmt.Sprintf("run_%s", time.Now().Format("20060102T150405"))
	benchRun.Timestamp = time.Now()

	for _, res := range results {
		if res.err != nil {
			return res.err
		}
		multiData.Calls = append(multiData.Calls, *res.data)
		benchRun.Calls = append(benchRun.Calls, res.bench)
	}

	if err := writeMultiCallReports(ctx, cfg, multiData); err != nil {
		return fmt.Errorf("writing reports: %w", err)
	}

	if flagSave {
		if err := saveRun(ctx, &benchRun); err != nil {
			return fmt.Errorf("saving benchmark: %w", err)
		}
	}

	return nil
}

func resolveAuth(ctx context.Context, cfg *config.Config) (map[string]string, error) {
	if cfg.Auth.Type == "" {
		return nil, nil
	}

	oauthCfg := auth.OAuthConfig{
		TokenURL:     cfg.Auth.TokenURL,
		ClientID:     cfg.Auth.ClientID,
		ClientSecret: cfg.Auth.ClientSecret,
	}
	provider, err := auth.NewProviderFromConfig(cfg.Auth.Type, cfg.Auth.Token, oauthCfg)
	if err != nil {
		return nil, fmt.Errorf("creating auth provider: %w", err)
	}
	if provider == nil {
		return nil, nil
	}

	md, err := provider.GetMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting auth metadata: %w", err)
	}
	return md, nil
}

func loadConfig() (*config.Config, error) {
	var cfg *config.Config

	if cfgFile != "" {
		var err error
		cfg, err = config.LoadFromYAML(cfgFile)
		if err != nil {
			return nil, fmt.Errorf("loading config: %w", err)
		}
	} else {
		defaultPath := "go-perf.yaml"
		if _, err := os.Stat(defaultPath); err == nil {
			cfg, _ = config.LoadFromYAML(defaultPath)
		}
		if cfg == nil {
			cfg = &config.Config{}
		}
	}

	overrides := &config.FlagOverrides{
		Target:      flagTarget,
		Proto:       flagProto,
		Call:        flagCall,
		Concurrency: flagConcurrency,
		Total:       flagTotal,
		Duration:    flagDuration,
		Connections: flagConnections,
		Timeout:     flagTimeout,
		Parallel:    flagParallel,
	}

	cfg = config.MergeFlags(cfg, overrides)

	if flagToken != "" {
		cfg.Auth.Type = "static"
		cfg.Auth.Token = flagToken
	}

	if flagOutputDir != "" {
		cfg.Output.Dir = flagOutputDir
	}
	if len(flagFormats) > 0 {
		cfg.Output.Formats = flagFormats
	}

	return cfg, nil
}

func buildVariantsFromEntry(entry config.CallEntry) ([]runner.VariantConfig, error) {
	if len(entry.DynamicFields) == 0 {
		return []runner.VariantConfig{
			runner.NewVariantConfig("default", map[string]interface{}{}),
		}, nil
	}

	providers := make(map[string]payload.DynamicProvider)
	for _, df := range entry.DynamicFields {
		p, err := payload.NewProviderFromConfig(payload.ProviderConfig{
			Type:   df.Type,
			Min:    df.Min,
			Max:    df.Max,
			Length: df.Length,
			Values: df.Values,
		})
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", df.Field, err)
		}
		providers[df.Field] = p
	}

	base := make(map[string]interface{})
	variant := payload.ApplyDynamicFields(base, providers)

	return []runner.VariantConfig{
		runner.NewVariantConfig("dynamic", variant),
	}, nil
}

func buildBenchmarkData(call string, cfg *config.Config, result *runner.Result, stats runner.LatencyStats) *report.BenchmarkData {
	data := &report.BenchmarkData{
		Timestamp:   time.Now(),
		Target:      cfg.Target,
		Call:        call,
		TotalCount:  result.TotalCount,
		ErrorCount:  result.ErrorCount,
		Duration:    result.Duration,
		Concurrency: cfg.Concurrency,
		AvgLatency:  stats.Average,
		MinLatency:  stats.Min,
		MaxLatency:  stats.Max,
		P50Latency:  stats.P50,
		P95Latency:  stats.P95,
		P99Latency:  stats.P99,
		RPS:         result.RPS,
		StatusCodes: result.StatusCodeCounts(),
	}

	groups := result.LatencyByPayloadHash()
	for hash, records := range groups {
		gs := runner.GroupLatencyStats(records)
		data.PayloadGroups = append(data.PayloadGroups, report.PayloadGroupData{
			PayloadHash: hash,
			Count:       gs.Count,
			AvgLatency:  gs.Average,
			MaxLatency:  gs.Max,
			P99Latency:  gs.P99,
		})
	}

	return data
}

func buildBenchmark(call string, cfg *config.Config, stats runner.LatencyStats, result *runner.Result) storage.Benchmark {
	return storage.Benchmark{
		ID:          fmt.Sprintf("%s_%s", call, time.Now().Format("20060102T150405")),
		Timestamp:   time.Now(),
		Target:      cfg.Target,
		Call:        call,
		TotalCount:  result.TotalCount,
		ErrorCount:  result.ErrorCount,
		Duration:    result.Duration,
		Concurrency: cfg.Concurrency,
		AvgLatency:  stats.Average,
		MinLatency:  stats.Min,
		MaxLatency:  stats.Max,
		P50Latency:  stats.P50,
		P95Latency:  stats.P95,
		P99Latency:  stats.P99,
		RPS:         result.RPS,
	}
}

func writeMultiCallReports(_ context.Context, cfg *config.Config, data *report.MultiCallBenchmarkData) error {
	reporters, err := report.ReportersFromFormats(cfg.Output.Formats, cfg.Output.Dir)
	if err != nil {
		return err
	}
	multi := report.NewMultiReporter(reporters...)
	return multi.WriteMultiCall(data)
}

func saveRun(_ context.Context, run *storage.BenchmarkRun) error {
	store := storage.NewJSONStore(flagStoreDir)
	path, err := store.SaveRun(run)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Benchmark run saved: %s\n", path)
	return nil
}
