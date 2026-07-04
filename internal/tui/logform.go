// Package tui contains tradectl's interactive terminal forms built on BubbleTea.
package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tradectl/internal/store"
)

// LogResult holds the raw values captured by the log form. Numeric fields are
// pre-parsed; pnl/r_multiple are computed downstream by the store.
type LogResult struct {
	SessionID      int64
	Direction      string
	Entry          float64
	Exit           float64
	Stop           float64
	Setup          string
	Leaks          []string
	Notes          string
	ScreenshotPath string // optional source path, as typed by the user
}

type stepKind int

const (
	kindText stepKind = iota
	kindSelect
	kindMulti
)

type step struct {
	key      string
	label    string
	help     string
	kind     stepKind
	options  []string
	numeric  bool
	optional bool
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	labelStyle  = lipgloss.NewStyle().Bold(true)
	helpStyle   = lipgloss.NewStyle().Faint(true)
	cursorStyle = lipgloss.NewStyle().Bold(true)
	errStyle    = lipgloss.NewStyle().Bold(true)
	selStyle    = lipgloss.NewStyle().Bold(true)
)

type model struct {
	steps     []step
	idx       int
	input     textinput.Model
	cursor    int          // highlighted option for select/multi steps
	checked   map[int]bool // toggled options for the current multi step
	textVals  map[string]string
	selVals   map[string]string
	multiVals map[string][]string
	defaults  map[string]string
	err       string
	done      bool
	canceled  bool
}

func newModel(defaultSessionID int64) model {
	steps := []step{
		{key: "session_id", label: "Session ID", help: "trade belongs to this session", kind: kindText, numeric: true},
		{key: "direction", label: "Direction", kind: kindSelect, options: []string{store.DirectionLong, store.DirectionShort}},
		{key: "entry", label: "Entry price", kind: kindText, numeric: true},
		{key: "exit", label: "Exit price", kind: kindText, numeric: true},
		{key: "stop", label: "Stop loss", kind: kindText, numeric: true},
		{key: "setup", label: "Setup type", kind: kindSelect, options: []string{store.SetupORB, store.SetupFVG, store.SetupOther}},
		{key: "leaks", label: "Leak tags", help: "space to toggle, enter to confirm (none = leave all unchecked)", kind: kindMulti, options: store.LeakTags},
		{key: "notes", label: "Notes", help: "optional, free text", kind: kindText, optional: true},
		{key: "screenshot", label: "Screenshot path", help: "optional; leave blank for none", kind: kindText, optional: true},
	}

	defaults := map[string]string{}
	if defaultSessionID > 0 {
		defaults["session_id"] = strconv.FormatInt(defaultSessionID, 10)
	}

	ti := textinput.New()
	ti.Prompt = "> "

	m := model{
		steps:     steps,
		input:     ti,
		checked:   map[int]bool{},
		textVals:  map[string]string{},
		selVals:   map[string]string{},
		multiVals: map[string][]string{},
		defaults:  defaults,
	}
	m.prepareStep()
	return m
}

// prepareStep configures interactive state for the step at m.idx.
func (m *model) prepareStep() {
	m.err = ""
	m.cursor = 0
	m.checked = map[int]bool{}
	s := m.steps[m.idx]
	if s.kind == kindText {
		m.input.SetValue(m.defaults[s.key])
		m.input.CursorEnd()
		m.input.Focus()
	} else {
		m.input.Blur()
	}
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch keyMsg.String() {
	case "ctrl+c", "esc":
		m.canceled = true
		return m, tea.Quit
	}

	switch m.steps[m.idx].kind {
	case kindText:
		return m.updateText(keyMsg)
	case kindSelect:
		return m.updateSelect(keyMsg)
	case kindMulti:
		return m.updateMulti(keyMsg)
	}
	return m, nil
}

func (m model) updateText(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.steps[m.idx]
	if key.String() == "enter" {
		val := strings.TrimSpace(m.input.Value())
		if val == "" && !s.optional {
			m.err = "this field is required"
			return m, nil
		}
		if val != "" && s.numeric {
			if _, err := strconv.ParseFloat(val, 64); err != nil {
				m.err = "please enter a valid number"
				return m, nil
			}
		}
		m.textVals[s.key] = val
		return m.advance()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(key)
	return m, cmd
}

func (m model) updateSelect(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.steps[m.idx]
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(s.options)-1 {
			m.cursor++
		}
	case "enter":
		m.selVals[s.key] = s.options[m.cursor]
		return m.advance()
	}
	return m, nil
}

func (m model) updateMulti(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.steps[m.idx]
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(s.options)-1 {
			m.cursor++
		}
	case " ":
		m.checked[m.cursor] = !m.checked[m.cursor]
	case "enter":
		var chosen []string
		for i, opt := range s.options {
			if m.checked[i] {
				chosen = append(chosen, opt)
			}
		}
		m.multiVals[s.key] = chosen
		return m.advance()
	}
	return m, nil
}

func (m model) advance() (tea.Model, tea.Cmd) {
	m.idx++
	if m.idx >= len(m.steps) {
		m.done = true
		return m, tea.Quit
	}
	m.prepareStep()
	return m, textinput.Blink
}

func (m model) View() string {
	if m.done || m.canceled {
		return ""
	}
	s := m.steps[m.idx]

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Log trade  (step %d/%d)", m.idx+1, len(m.steps))))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render(s.label))
	b.WriteString("\n")
	if s.help != "" {
		b.WriteString(helpStyle.Render(s.help))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	switch s.kind {
	case kindText:
		b.WriteString(m.input.View())
	case kindSelect:
		for i, opt := range s.options {
			b.WriteString(renderRadio(i == m.cursor, opt))
			b.WriteString("\n")
		}
	case kindMulti:
		for i, opt := range s.options {
			b.WriteString(renderCheckbox(i == m.cursor, m.checked[i], opt))
			b.WriteString("\n")
		}
	}

	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(errStyle.Render("! " + m.err))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(navHelp(s.kind)))
	return b.String()
}

func renderRadio(active bool, label string) string {
	marker := "  "
	if active {
		marker = cursorStyle.Render("> ")
		label = selStyle.Render(label)
	}
	return marker + label
}

func renderCheckbox(active, checked bool, label string) string {
	box := "[ ]"
	if checked {
		box = "[x]"
	}
	marker := "  "
	if active {
		marker = cursorStyle.Render("> ")
		label = selStyle.Render(label)
	}
	return fmt.Sprintf("%s%s %s", marker, box, label)
}

func navHelp(kind stepKind) string {
	switch kind {
	case kindText:
		return "enter: next · esc: cancel"
	case kindSelect:
		return "↑/↓: move · enter: select · esc: cancel"
	default:
		return "↑/↓: move · space: toggle · enter: confirm · esc: cancel"
	}
}

// RunLogForm runs the interactive trade-logging form. The returned bool is
// false if the user canceled. defaultSessionID, when > 0, prefills the session
// field with the most recently opened unclosed session.
func RunLogForm(defaultSessionID int64) (LogResult, bool, error) {
	p := tea.NewProgram(newModel(defaultSessionID))
	final, err := p.Run()
	if err != nil {
		return LogResult{}, false, err
	}
	m := final.(model)
	if m.canceled || !m.done {
		return LogResult{}, false, nil
	}

	sid, _ := strconv.ParseInt(m.textVals["session_id"], 10, 64)
	entry, _ := strconv.ParseFloat(m.textVals["entry"], 64)
	exit, _ := strconv.ParseFloat(m.textVals["exit"], 64)
	stop, _ := strconv.ParseFloat(m.textVals["stop"], 64)

	return LogResult{
		SessionID:      sid,
		Direction:      m.selVals["direction"],
		Entry:          entry,
		Exit:           exit,
		Stop:           stop,
		Setup:          m.selVals["setup"],
		Leaks:          m.multiVals["leaks"],
		Notes:          m.textVals["notes"],
		ScreenshotPath: m.textVals["screenshot"],
	}, true, nil
}
