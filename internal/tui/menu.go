package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// menu is the home screen: a table of all sessions.
type menu struct {
	rows         []sessionRow
	cursor       int
	confirmClose bool // pending y/n on the selected session
}

// sessionRow is a display row (store list row + derived status).
type sessionRow struct {
	id         int64
	name       string
	market     string
	instrument string
	started    string
	open       bool
	trades     int
	pnlCash    float64
}

// refreshMenu reloads the sessions table from the store.
func (a *App) refreshMenu() error {
	list, err := a.st.ListSessions()
	if err != nil {
		return err
	}
	rows := make([]sessionRow, 0, len(list))
	for _, r := range list {
		rows = append(rows, sessionRow{
			id:         r.ID,
			name:       r.Name,
			market:     r.Market,
			instrument: r.Instrument,
			started:    r.StartedAt.Local().Format("2006-01-02 15:04"),
			open:       r.EndedAt == nil,
			trades:     r.TradeCount,
			pnlCash:    r.TotalPnLCash,
		})
	}
	a.menu.rows = rows
	if a.menu.cursor >= len(rows) {
		a.menu.cursor = max(0, len(rows)-1)
	}
	return nil
}

func (a *App) updateMenu(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	m := &a.menu

	if m.confirmClose {
		switch key.String() {
		case "y", "Y":
			m.confirmClose = false
			sel := m.rows[m.cursor]
			if err := a.st.CloseSession(sel.id); err != nil {
				a.status = "error: " + err.Error()
			} else {
				a.status = fmt.Sprintf("closed session %d (%s)", sel.id, sel.name)
			}
			if err := a.refreshMenu(); err != nil {
				a.status = "error: " + err.Error()
			}
		default:
			m.confirmClose = false
			a.status = "close canceled"
		}
		return a, nil
	}

	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case "enter":
		if len(m.rows) > 0 {
			if err := a.openDashboard(m.rows[m.cursor].id); err != nil {
				a.status = "error: " + err.Error()
			}
		}
	case "n":
		a.form = newSessionForm()
		a.screen = screenNewSession
		return a, a.form.Init()
	case "c":
		if len(m.rows) > 0 {
			if !m.rows[m.cursor].open {
				a.status = fmt.Sprintf("session %d is already closed", m.rows[m.cursor].id)
				return a, nil
			}
			m.confirmClose = true
		}
	case "q":
		return a, tea.Quit
	}
	return a, nil
}

func (a *App) viewMenu() string {
	m := a.menu
	var b strings.Builder
	b.WriteString(titleStyle.Render("tradectl — sessions"))
	b.WriteString("\n\n")

	if len(m.rows) == 0 {
		b.WriteString(helpStyle.Render("No sessions yet — press n to start one."))
		b.WriteString("\n")
	} else {
		b.WriteString(headerStyle.Render(fmt.Sprintf("%-4s %-22s %-8s %-8s %-17s %-8s %-7s %s",
			"ID", "NAME", "MARKET", "INSTR", "STARTED", "STATUS", "TRADES", "P/L")))
		b.WriteString("\n")
		for i, r := range m.rows {
			status := "closed"
			if r.open {
				status = "open"
			}
			name := r.name
			if len(name) > 21 {
				name = name[:21] + "…"
			}
			line := fmt.Sprintf("%-4d %-22s %-8s %-8s %-17s %-8s %-7d ",
				r.id, name, r.market, r.instrument, r.started, status, r.trades)
			if i == m.cursor {
				line = selStyle.Render("> " + line)
			} else {
				line = "  " + line
			}
			b.WriteString(line)
			b.WriteString(money(r.pnlCash))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.confirmClose && len(m.rows) > 0 {
		sel := m.rows[m.cursor]
		b.WriteString(errStyle.Render(fmt.Sprintf("Close session %d (%s)? y/n", sel.id, sel.name)))
	} else {
		b.WriteString(helpStyle.Render("↑/↓: move · enter: open · n: new session · c: close · :: command · q: quit"))
	}
	return b.String()
}
