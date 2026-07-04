package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
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
			instrument, err := promptLine(r, "Instrument (e.g. NQ, ES): ")
			if err != nil {
				return err
			}
			if instrument == "" {
				return fmt.Errorf("instrument is required")
			}
			notes, err := promptLine(r, "Notes (optional): ")
			if err != nil {
				return err
			}

			id, err := a.st.CreateSession(instrument, notes)
			if err != nil {
				return err
			}
			fmt.Printf("Created session %d (%s).\n", id, instrument)
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
			fmt.Fprintln(w, "ID\tINSTRUMENT\tSTARTED\tENDED\tTRADES")
			for _, r := range rows {
				ended := "(open)"
				if r.EndedAt != nil {
					ended = r.EndedAt.Local().Format("2006-01-02 15:04")
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\n",
					r.ID, r.Instrument, r.StartedAt.Local().Format("2006-01-02 15:04"), ended, r.TradeCount)
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
