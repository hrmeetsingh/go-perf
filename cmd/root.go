package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:   "go-perf",
	Short: "A standalone performance benchmarking tool for gRPC services",
	Long: `go-perf is a CLI tool for benchmarking gRPC services using the ghz library.

It supports:
  - Unary and bidirectional streaming RPCs
  - Dynamic payload generation from proto files
  - Multi-variant parallel benchmarking
  - HTML, JSON, JUnit report generation
  - Benchmark storage and comparison
  - JWT authentication (static token or OAuth2/OIDC)
  - CI/CD pipeline integration`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: ./go-perf.yaml)")
}
