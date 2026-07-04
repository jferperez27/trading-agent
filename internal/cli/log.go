package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"tradectl/internal/store"
	"tradectl/internal/tui"
)

func newLogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "log",
		Short: "Log a trade interactively",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			defer a.st.Close()

			// Default the session field to the most recently opened, still-open
			// session, but let the form override it.
			defaultSession, _, err := a.st.LatestOpenSessionID()
			if err != nil {
				return err
			}

			res, ok, err := tui.RunLogForm(defaultSession)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("Canceled — no trade logged.")
				return nil
			}

			exists, err := a.st.SessionExists(res.SessionID)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("session %d does not exist", res.SessionID)
			}

			trade := store.Trade{
				SessionID:  res.SessionID,
				EntryPrice: res.Entry,
				ExitPrice:  res.Exit,
				StopLoss:   res.Stop,
				Size:       res.Size,
				Direction:  res.Direction,
				SetupType:  res.Setup,
				LeakTags:   res.Leaks,
				Notes:      res.Notes,
			}
			tradeID, err := a.st.InsertTrade(trade)
			if err != nil {
				return err
			}

			// Copy the screenshot (if provided) now that the trade ID exists,
			// since the stored filename embeds it. A copy failure must not
			// discard the already-saved trade.
			if res.ScreenshotPath != "" {
				rel, copyErr := a.st.SaveScreenshot(res.SessionID, tradeID, res.ScreenshotPath)
				if copyErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: trade %d saved, but screenshot was not stored: %v\n", tradeID, copyErr)
				} else if err := a.st.SetTradeScreenshot(tradeID, rel); err != nil {
					return err
				}
			}

			pnl, r := store.ComputeMetrics(res.Direction, res.Entry, res.Exit, res.Stop)
			fmt.Printf("Logged trade %d in session %d: %s %s  pnl=%.2f pts  r=%.2fR\n",
				tradeID, res.SessionID, res.Direction, res.Setup, pnl, r)
			return nil
		},
	}
}
