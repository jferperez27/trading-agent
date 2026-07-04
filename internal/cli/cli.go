// Package cli assembles tradectl's cobra command tree and shared per-run
// application context (loaded config + open store).
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"tradectl/internal/config"
	"tradectl/internal/store"
)

// app bundles the loaded config and an open store for a single command run.
type app struct {
	cfg config.Config
	st  *store.Store
}

// loadApp loads config (creating it on first run) and opens the database,
// applying migrations. Callers must Close the store.
func loadApp() (*app, error) {
	cfg, createdPath, err := config.Load()
	if err != nil {
		return nil, err
	}
	if createdPath != "" {
		fmt.Fprintf(os.Stderr, "Created default config at %s\n", createdPath)
	}
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	return &app{cfg: cfg, st: st}, nil
}

// newRootCmd builds the root command with all subcommands attached.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "tradectl",
		Short:         "Personal FxReplay backtest session analyzer & documentor",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newSessionsCmd(), newLogCmd(), newAnalyzeCmd())
	return root
}

// Execute runs the root command and exits non-zero on error.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
