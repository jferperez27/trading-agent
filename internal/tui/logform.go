// Package tui contains tradectl's interactive terminal UI built on BubbleTea:
// the full-screen app (Run) and the standalone trade-logging form (RunLogForm)
// used by the `tradectl log` subcommand.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// standaloneForm adapts a stepForm into a self-contained tea.Model for the
// `tradectl log` subcommand (outside the full-screen app).
type standaloneForm struct {
	f stepForm
}

func (m standaloneForm) Init() tea.Cmd { return m.f.Init() }

func (m standaloneForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.f, cmd = m.f.Update(msg)
	if m.f.done || m.f.canceled {
		return m, tea.Quit
	}
	return m, cmd
}

func (m standaloneForm) View() string { return m.f.View() }

// RunLogForm runs the interactive trade-logging form as its own program. The
// returned bool is false if the user canceled. defaultSessionID, when > 0,
// prefills the session field with the most recently opened unclosed session.
func RunLogForm(defaultSessionID int64) (LogResult, bool, error) {
	p := tea.NewProgram(standaloneForm{f: newStandaloneTradeForm(defaultSessionID)})
	final, err := p.Run()
	if err != nil {
		return LogResult{}, false, err
	}
	m := final.(standaloneForm)
	if m.f.canceled || !m.f.done {
		return LogResult{}, false, nil
	}
	return tradeResult(m.f, 0), true, nil
}
