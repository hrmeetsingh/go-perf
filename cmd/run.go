package cmd

import (
	"context"
	"fmt"
	"os"
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
)

func init() {
	runCmd.Flags().StringVar(&flagTarget, "target", "", "gRPC server target (host:port)")
	runCmd.Flags().StringVar(&flagProto, "proto", "", "path to proto file or directory")
	runCmd.Flags().StringVar(&flagCall, "call", "", "fully qualified method (pkg.Service/Method)")
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

	rootCmd.AddCommand(runCmd)
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

	runCfg := runner.RunConfig{
		Target:      cfg.Target,
		Call:        cfg.Call,
		ProtoPath:   protoFiles[0],
		Concurrency: cfg.Concurrency,
		Total:       cfg.Total,
		Duration:    dur,
		Connections: cfg.Connections,
		Timeout:     timeout,
		Metadata:    cfg.Metadata,
		Insecure:    flagInsecure,
	}

	variants, err := buildVariants(cfg)
	if err != nil {
		return fmt.Errorf("building variants: %w", err)
	}

	orchCfg := runner.OrchestratorConfig{
		RunConfig: runCfg,
		Variants:  variants,
	}

	engine := runner.NewGhzEngine()
	orch := runner.NewOrchestrator(orchCfg, authMeta, engine)
	multiResult, err := orch.Run(ctx)
	if err != nil {
		return fmt.Errorf("benchmark run failed: %w", err)
	}

	merged := multiResult.Merge()
	stats := merged.LatencyStats()
	benchData := buildBenchmarkData(cfg, merged, stats)

	if err := writeReports(ctx, cfg, benchData); err != nil {
		return fmt.Errorf("writing reports: %w", err)
	}

	if flagSave {
		if err := saveBenchmark(ctx, cfg, stats, merged); err != nil {
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

func buildVariants(cfg *config.Config) ([]runner.VariantConfig, error) {
	if len(cfg.DynamicFields) == 0 {
		return []runner.VariantConfig{
			runner.NewVariantConfig("default", map[string]interface{}{}),
		}, nil
	}

	providers := make(map[string]payload.DynamicProvider)
	for _, df := range cfg.DynamicFields {
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

func buildBenchmarkData(cfg *config.Config, result *runner.Result, stats runner.LatencyStats) *report.BenchmarkData {
	data := &report.BenchmarkData{
		Timestamp:   time.Now(),
		Target:      cfg.Target,
		Call:        cfg.Call,
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
		var total time.Duration
		var maxLat time.Duration
		for _, r := range records {
			total += r.Duration
			if r.Duration > maxLat {
				maxLat = r.Duration
			}
		}
		avg := total / time.Duration(len(records))

		data.PayloadGroups = append(data.PayloadGroups, report.PayloadGroupData{
			PayloadHash: hash,
			Count:       len(records),
			AvgLatency:  avg,
			MaxLatency:  maxLat,
		})
	}

	return data
}

func writeReports(_ context.Context, cfg *config.Config, data *report.BenchmarkData) error {
	reporters, err := report.ReportersFromFormats(cfg.Output.Formats, cfg.Output.Dir)
	if err != nil {
		return err
	}
	multi := report.NewMultiReporter(reporters...)
	return multi.Write(data)
}

func saveBenchmark(_ context.Context, cfg *config.Config, stats runner.LatencyStats, result *runner.Result) error {
	store := storage.NewJSONStore(flagStoreDir)
	b := &storage.Benchmark{
		ID:          fmt.Sprintf("%s_%s", cfg.Call, time.Now().Format("20060102T150405")),
		Timestamp:   time.Now(),
		Target:      cfg.Target,
		Call:        cfg.Call,
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

	path, err := store.Save(b)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Benchmark saved: %s\n", path)
	return nil
}
