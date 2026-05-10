package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/hrmeetsingh/go-perf/internal/storage"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored benchmark results",
	Long:  "Display all stored benchmark results available for comparison.",
	RunE:  runList,
}

var listStoreDir string

func init() {
	listCmd.Flags().StringVar(&listStoreDir, "store-dir", ".go-perf/benchmarks", "directory for stored benchmarks")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	store := storage.NewJSONStore(listStoreDir)

	entries, err := store.List()
	if err != nil {
		return fmt.Errorf("listing benchmarks: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No stored benchmarks found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTIMESTAMP\tPATH")
	for _, e := range entries {
		fmt.Fprintf(w, "%s\t%s\t%s\n", e.ID, e.Timestamp.Format("2006-01-02 15:04:05"), e.Path)
	}
	w.Flush()

	return nil
}
