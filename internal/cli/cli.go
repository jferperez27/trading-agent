// Package cli assembles tradectl's cobra command tree and shared per-run
// application context (loaded config + open store).
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"tradectl/internal/config"
	"tradectl/internal/store"
	"tradectl/internal/tui"
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

// newRootCmd builds the root command with all subcommands attached. Invoked
// with no subcommand, tradectl launches the full-screen interactive app; the
// subcommands remain for shell scripting.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "tradectl",
		Short:         "Personal FxReplay backtest session logger & tracker",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			defer a.st.Close()
			return tui.Run(a.st, a.cfg)
		},
	}
	root.AddCommand(newSessionsCmd(), newLogCmd())
	return root
}

// Execute runs the root command and exits non-zero on error.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
