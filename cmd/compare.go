package cmd

import (
	"fmt"

	"github.com/hrmeetsingh/go-perf/internal/storage"
	"github.com/spf13/cobra"
)

var compareCmd = &cobra.Command{
	Use:   "compare [baseline] [current]",
	Short: "Compare two benchmark results",
	Long:  "Load two stored benchmark files and compare their performance metrics.",
	Args:  cobra.ExactArgs(2),
	RunE:  runCompare,
}

var compareStoreDir string

func init() {
	compareCmd.Flags().StringVar(&compareStoreDir, "store-dir", ".go-perf/benchmarks", "directory for stored benchmarks")
	rootCmd.AddCommand(compareCmd)
}

func runCompare(cmd *cobra.Command, args []string) error {
	store := storage.NewJSONStore(compareStoreDir)

	baseline, err := store.Load(args[0])
	if err != nil {
		return fmt.Errorf("loading baseline: %w", err)
	}

	current, err := store.Load(args[1])
	if err != nil {
		return fmt.Errorf("loading current: %w", err)
	}

	result := storage.Compare(baseline, current)
	fmt.Print(result.Summary())

	return nil
}
