package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"tradectl/internal/config"
	"tradectl/internal/store"
)

// screenID identifies the active screen of the full-screen app.
type screenID int

const (
	screenMenu screenID = iota
	screenNewSession
	screenDashboard
)

// App is the root BubbleTea model for the continuous full-screen experience:
// launch tradectl and you stay in the app (menu → session dashboard → forms)
// until you quit.
type App struct {
	st  *store.Store
	cfg config.Config

	screen screenID
	menu   menu
	form   stepForm // new-session form (screenNewSession)
	dash   dashboard
	bar    cmdbar

	status string
}

// Run starts the full-screen TUI. It blocks until the user quits.
func Run(st *store.Store, cfg config.Config) error {
	app := &App{st: st, cfg: cfg, bar: newCmdbar()}
	if err := app.refreshMenu(); err != nil {
		return err
	}
	_, err := tea.NewProgram(app, tea.WithAltScreen()).Run()
	return err
}

func (a *App) Init() tea.Cmd { return nil }

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Global: ctrl+c always quits, from anywhere.
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "ctrl+c" {
		return a, tea.Quit
	}

	// Command bar captures input while open.
	if a.bar.active {
		var (
			submitted bool
			line      string
			cmd       tea.Cmd
		)
		a.bar, submitted, line, cmd = a.bar.update(msg)
		if submitted {
			return a.execCommand(line)
		}
		return a, cmd
	}

	// ':' opens the command bar on browse screens (never inside forms, where
	// it must remain a typable character).
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == ":" && a.barAvailable() {
		a.status = ""
		return a, a.bar.open()
	}

	switch a.screen {
	case screenMenu:
		if key, ok := msg.(tea.KeyMsg); ok {
			a.status = ""
			return a.updateMenu(key)
		}
		return a, nil

	case screenNewSession:
		var cmd tea.Cmd
		a.form, cmd = a.form.Update(msg)
		switch {
		case a.form.canceled:
			a.screen = screenMenu
			a.status = "session creation canceled"
		case a.form.done:
			a.createSession()
		}
		return a, cmd

	case screenDashboard:
		return a.updateDashboard(msg)
	}
	return a, nil
}

// barAvailable reports whether ':' should open the command bar in the current
// state (browse contexts only — not while typing into a form).
func (a *App) barAvailable() bool {
	switch a.screen {
	case screenMenu:
		return !a.menu.confirmClose
	case screenDashboard:
		return !a.dash.adding && !a.dash.confirmClose && a.dash.closedMeta == nil
	default:
		return false
	}
}

// createSession persists the completed new-session form and jumps straight
// into the session's dashboard, ready to log trades.
func (a *App) createSession() {
	params := sessionParams(a.form)
	id, err := a.st.CreateSession(params)
	if err != nil {
		a.screen = screenMenu
		a.status = "error: " + err.Error()
		return
	}
	if err := a.openDashboard(id); err != nil {
		a.screen = screenMenu
		a.status = "error: " + err.Error()
		return
	}
	a.status = fmt.Sprintf("session %d (%s) started — press a to log a trade", id, params.Name)
}

// execCommand parses and runs a command-bar line.
func (a *App) execCommand(line string) (tea.Model, tea.Cmd) {
	cmd, err := ParseCommand(line)
	if err != nil {
		a.status = err.Error()
		return a, nil
	}

	switch cmd.Kind {
	case CmdQuit:
		return a, tea.Quit

	case CmdHelp:
		a.status = commandHelp

	case CmdNew:
		a.form = newSessionForm()
		a.screen = screenNewSession
		a.status = ""
		return a, a.form.Init()

	case CmdOpen:
		if err := a.openDashboard(cmd.ID); err != nil {
			a.status = "error: " + err.Error()
		}

	case CmdClose:
		if err := a.st.CloseSession(cmd.ID); err != nil {
			a.status = "error: " + err.Error()
			return a, nil
		}
		a.status = fmt.Sprintf("closed session %d", cmd.ID)
		// Keep whatever screen is showing in sync with the change.
		if a.screen == screenDashboard && a.dash.sess.ID == cmd.ID {
			if err := a.openDashboard(cmd.ID); err != nil {
				a.status = "error: " + err.Error()
			}
		}
		if a.screen == screenMenu {
			if err := a.refreshMenu(); err != nil {
				a.status = "error: " + err.Error()
			}
		}
	}
	return a, nil
}

func (a *App) View() string {
	var body string
	switch a.screen {
	case screenMenu:
		body = a.viewMenu()
	case screenNewSession:
		body = a.form.View()
	case screenDashboard:
		body = a.viewDashboard()
	}

	var footer []string
	if a.bar.active {
		footer = append(footer, a.bar.view())
	}
	if a.status != "" {
		footer = append(footer, statusStyle.Render(a.status))
	}

	if len(footer) == 0 {
		return body
	}
	return body + "\n\n" + strings.Join(footer, "\n")
}
