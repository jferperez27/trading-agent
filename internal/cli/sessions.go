package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"tradectl/internal/store"
)

func newSessionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage backtest sessions",
	}
	cmd.AddCommand(newSessionsNewCmd(), newSessionsCloseCmd(), newSessionsListCmd())
	return cmd
}

func newSessionsNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new",
		Short: "Start a new backtest session",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			defer a.st.Close()

			r := bufio.NewReader(os.Stdin)
			name, err := promptLine(r, "Session name: ")
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("session name is required")
			}
			market, err := promptLine(r, fmt.Sprintf("Market (%s): ", strings.Join(store.Markets, "/")))
			if err != nil {
				return err
			}
			if !slices.Contains(store.Markets, market) {
				return fmt.Errorf("invalid market %q (want one of: %s)", market, strings.Join(store.Markets, ", "))
			}
			instrument, err := promptLine(r, "Instrument (e.g. NQ, ES): ")
			if err != nil {
				return err
			}
			if instrument == "" {
				return fmt.Errorf("instrument is required")
			}
			balanceStr, err := promptLine(r, "Initial money (e.g. 50000): ")
			if err != nil {
				return err
			}
			balance, err := strconv.ParseFloat(balanceStr, 64)
			if err != nil {
				return fmt.Errorf("invalid initial money %q", balanceStr)
			}
			notes, err := promptLine(r, "Notes (optional): ")
			if err != nil {
				return err
			}

			id, err := a.st.CreateSession(store.SessionParams{
				Name:           name,
				Market:         market,
				Instrument:     instrument,
				InitialBalance: balance,
				Notes:          notes,
			})
			if err != nil {
				return err
			}
			fmt.Printf("Created session %d (%s, %s %s).\n", id, name, market, instrument)
			return nil
		},
	}
}

func newSessionsCloseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "close <id>",
		Short: "Close an open session",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid session id %q", args[0])
			}
			a, err := loadApp()
			if err != nil {
				return err
			}
			defer a.st.Close()

			if err := a.st.CloseSession(id); err != nil {
				return err
			}
			fmt.Printf("Closed session %d.\n", id)
			return nil
		},
	}
}

func newSessionsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all sessions",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			defer a.st.Close()

			rows, err := a.st.ListSessions()
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Println("No sessions yet. Create one with: tradectl sessions new")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tMARKET\tINSTRUMENT\tSTARTED\tENDED\tTRADES\tP/L")
			for _, r := range rows {
				ended := "(open)"
				if r.EndedAt != nil {
					ended = r.EndedAt.Local().Format("2006-01-02 15:04")
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%d\t%+.2f\n",
					r.ID, r.Name, r.Market, r.Instrument,
					r.StartedAt.Local().Format("2006-01-02 15:04"), ended, r.TradeCount, r.TotalPnLCash)
			}
			return w.Flush()
		},
	}
}

func promptLine(r *bufio.Reader, label string) (string, error) {
	fmt.Print(label)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
