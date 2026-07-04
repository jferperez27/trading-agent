package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"tradectl/internal/store"
)

// maxDashTrades caps how many recent trades render under the stats header.
const maxDashTrades = 10

// dashboard is the live in-session screen: a persistent stats header that
// refreshes as trades are logged, the trade list, and the embedded trade form.
type dashboard struct {
	sess         store.Session
	stats        store.Stats
	trades       []store.Trade
	form         stepForm
	adding       bool
	confirmClose bool
	closedMeta   *store.ClosedMeta // non-nil => showing the close summary
}

// openDashboard loads a session and switches to its dashboard.
func (a *App) openDashboard(id int64) error {
	sess, err := a.st.GetSession(id)
	if err != nil {
		return err
	}
	a.dash = dashboard{sess: sess}
	if err := a.refreshDashboard(); err != nil {
		return err
	}
	a.screen = screenDashboard
	a.status = ""
	return nil
}

// refreshDashboard reloads trades + stats for the current session.
func (a *App) refreshDashboard() error {
	trades, err := a.st.GetSessionTrades(a.dash.sess.ID)
	if err != nil {
		return err
	}
	a.dash.trades = trades
	a.dash.stats = store.ComputeStats(a.dash.sess.InitialBalance, trades)
	return nil
}

func (a *App) updateDashboard(msg tea.Msg) (tea.Model, tea.Cmd) {
	d := &a.dash

	// Close summary showing: any key returns to the menu.
	if d.closedMeta != nil {
		if _, ok := msg.(tea.KeyMsg); ok {
			d.closedMeta = nil
			a.screen = screenMenu
			if err := a.refreshMenu(); err != nil {
				a.status = "error: " + err.Error()
			}
		}
		return a, nil
	}

	// Embedded trade form gets every message while active.
	if d.adding {
		var cmd tea.Cmd
		d.form, cmd = d.form.Update(msg)
		switch {
		case d.form.canceled:
			d.adding = false
			a.status = "trade entry canceled"
		case d.form.done:
			a.submitTrade()
			// Stay in the loop: fresh form, ready for the next trade.
			d.form = newTradeForm()
			return a, d.form.Init()
		}
		return a, cmd
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return a, nil
	}

	if d.confirmClose {
		switch key.String() {
		case "y", "Y":
			d.confirmClose = false
			a.closeCurrentSession()
		default:
			d.confirmClose = false
			a.status = "close canceled"
		}
		return a, nil
	}

	switch key.String() {
	case "a":
		if d.sess.EndedAt != nil {
			a.status = "session is closed — reopen is not supported; start a new one"
			return a, nil
		}
		d.form = newTradeForm()
		d.adding = true
		a.status = ""
		return a, d.form.Init()
	case "c":
		if d.sess.EndedAt != nil {
			a.status = "session is already closed"
			return a, nil
		}
		d.confirmClose = true
	case "esc":
		a.screen = screenMenu
		if err := a.refreshMenu(); err != nil {
			a.status = "error: " + err.Error()
		}
	}
	return a, nil
}

// submitTrade persists the completed trade form, refreshes stats, and reports
// via the status line. Screenshot failures warn but never discard the trade.
func (a *App) submitTrade() {
	res := tradeResult(a.dash.form, a.dash.sess.ID)
	tradeID, err := a.st.InsertTrade(store.Trade{
		SessionID:  res.SessionID,
		EntryPrice: res.Entry,
		ExitPrice:  res.Exit,
		StopLoss:   res.Stop,
		Size:       res.Size,
		Direction:  res.Direction,
		SetupType:  res.Setup,
		LeakTags:   res.Leaks,
		Notes:      res.Notes,
	})
	if err != nil {
		a.status = "error: " + err.Error()
		return
	}

	if res.ScreenshotPath != "" {
		rel, copyErr := a.st.SaveScreenshot(res.SessionID, tradeID, res.ScreenshotPath)
		if copyErr != nil {
			a.status = fmt.Sprintf("trade %d saved, but screenshot was not stored: %v", tradeID, copyErr)
		} else if err := a.st.SetTradeScreenshot(tradeID, rel); err != nil {
			a.status = fmt.Sprintf("trade %d saved, but screenshot path not recorded: %v", tradeID, err)
		}
	}

	if err := a.refreshDashboard(); err != nil {
		a.status = "error: " + err.Error()
		return
	}
	if a.status == "" || !strings.HasPrefix(a.status, fmt.Sprintf("trade %d saved,", tradeID)) {
		a.status = fmt.Sprintf("trade %d logged — esc to stop adding", tradeID)
	}
}

// closeCurrentSession closes the dashboard's session, then shows the summary.
func (a *App) closeCurrentSession() {
	d := &a.dash
	if err := a.st.CloseSession(d.sess.ID); err != nil {
		a.status = "error: " + err.Error()
		return
	}
	sess, err := a.st.GetSession(d.sess.ID)
	if err != nil {
		a.status = "error: " + err.Error()
		return
	}
	d.sess = sess

	var meta store.ClosedMeta
	if err := json.Unmarshal([]byte(sess.ClosedMeta), &meta); err != nil {
		a.status = "error: reading closed metadata: " + err.Error()
		return
	}
	d.closedMeta = &meta
	a.status = ""
}

func (a *App) viewDashboard() string {
	d := a.dash
	var b strings.Builder

	b.WriteString(a.viewStatsHeader())
	b.WriteString("\n")

	if d.closedMeta != nil {
		b.WriteString(a.viewCloseSummary(*d.closedMeta))
		return b.String()
	}

	if d.adding {
		b.WriteString(d.form.View())
		return b.String()
	}

	// Trades list (most recent last, capped).
	if len(d.trades) == 0 {
		b.WriteString(helpStyle.Render("No trades yet — press a to log the first one."))
		b.WriteString("\n")
	} else {
		b.WriteString(headerStyle.Render(fmt.Sprintf("%-4s %-6s %-6s %-7s %9s %11s %7s",
			"ID", "DIR", "SETUP", "SIZE", "PNL(pts)", "PNL($)", "R")))
		b.WriteString("\n")
		trades := d.trades
		if len(trades) > maxDashTrades {
			b.WriteString(helpStyle.Render(fmt.Sprintf("… %d earlier trades", len(trades)-maxDashTrades)))
			b.WriteString("\n")
			trades = trades[len(trades)-maxDashTrades:]
		}
		for _, t := range trades {
			b.WriteString(fmt.Sprintf("%-4d %-6s %-6s %-7.2f %9.2f %11s %6.2fR\n",
				t.ID, t.Direction, t.SetupType, t.Size, t.PnL, money(t.PnLCash), t.RMultiple))
		}
	}

	b.WriteString("\n")
	if d.confirmClose {
		b.WriteString(errStyle.Render(fmt.Sprintf("Close session %d (%s)? y/n", d.sess.ID, d.sess.Name)))
	} else if d.sess.EndedAt == nil {
		b.WriteString(helpStyle.Render("a: add trade · c: close session · :: command · esc: back to menu"))
	} else {
		b.WriteString(helpStyle.Render("session closed (read-only) · esc: back to menu"))
	}
	return b.String()
}

// viewStatsHeader renders the persistent live-stats box.
func (a *App) viewStatsHeader() string {
	d := a.dash
	s := d.stats

	status := "OPEN"
	if d.sess.EndedAt != nil {
		status = "CLOSED"
	}
	title := fmt.Sprintf("#%d %s  ·  %s/%s  ·  %s",
		d.sess.ID, d.sess.Name, d.sess.Market, d.sess.Instrument, status)

	line1 := fmt.Sprintf("balance %s   p/l %s (%.2f pts)",
		formatMoney(s.CurrentBalance), money(s.TotalPnLCash), s.TotalPnLPoints)
	line2 := fmt.Sprintf("trades %d   won %d   lost %d   win rate %.0f%%   avg %.2fR",
		s.TradeCount, s.Wins, s.Losses, s.WinRate*100, s.AvgRMultiple)

	return statBoxStyle.Render(titleStyle.Render(title) + "\n" + line1 + "\n" + line2)
}

// viewCloseSummary renders the final metadata after closing a session.
func (a *App) viewCloseSummary(m store.ClosedMeta) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Session closed"))
	b.WriteString("\n\n")
	dur := m.DurationSeconds
	b.WriteString(fmt.Sprintf("  duration     %dh %dm %ds\n", dur/3600, (dur%3600)/60, dur%60))
	b.WriteString(fmt.Sprintf("  trades       %d (won %d / lost %d, win rate %.0f%%)\n",
		m.TradeCount, m.Wins, m.Losses, m.WinRate*100))
	b.WriteString(fmt.Sprintf("  p/l          %s (%.2f pts, avg %.2fR)\n",
		money(m.TotalPnLCash), m.TotalPnLPoints, m.AvgRMultiple))
	b.WriteString(fmt.Sprintf("  final balance %s\n", formatMoney(m.FinalBalance)))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("press any key to return to the menu"))
	return b.String()
}
