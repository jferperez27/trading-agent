package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// stepform.go is the generic step-by-step form engine shared by the trade form
// and the new-session form. It is an embeddable sub-model: parents route
// messages through Update and check done/canceled after each call.

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

type stepForm struct {
	title     string
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

func newStepForm(title string, steps []step, defaults map[string]string) stepForm {
	ti := textinput.New()
	ti.Prompt = "> "
	if defaults == nil {
		defaults = map[string]string{}
	}
	f := stepForm{
		title:     title,
		steps:     steps,
		input:     ti,
		checked:   map[int]bool{},
		textVals:  map[string]string{},
		selVals:   map[string]string{},
		multiVals: map[string][]string{},
		defaults:  defaults,
	}
	f.prepareStep()
	return f
}

// prepareStep configures interactive state for the step at f.idx.
func (f *stepForm) prepareStep() {
	f.err = ""
	f.cursor = 0
	f.checked = map[int]bool{}
	s := f.steps[f.idx]
	if s.kind == kindText {
		f.input.SetValue(f.defaults[s.key])
		f.input.CursorEnd()
		f.input.Focus()
	} else {
		f.input.Blur()
	}
}

func (f stepForm) Init() tea.Cmd { return textinput.Blink }

func (f stepForm) Update(msg tea.Msg) (stepForm, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		return f, cmd
	}

	switch keyMsg.String() {
	case "ctrl+c", "esc":
		f.canceled = true
		return f, nil
	}

	switch f.steps[f.idx].kind {
	case kindText:
		return f.updateText(keyMsg)
	case kindSelect:
		return f.updateSelect(keyMsg)
	case kindMulti:
		return f.updateMulti(keyMsg)
	}
	return f, nil
}

func (f stepForm) updateText(key tea.KeyMsg) (stepForm, tea.Cmd) {
	s := f.steps[f.idx]
	if key.String() == "enter" {
		val := strings.TrimSpace(f.input.Value())
		if val == "" && !s.optional {
			f.err = "this field is required"
			return f, nil
		}
		if val != "" && s.numeric {
			if _, err := strconv.ParseFloat(val, 64); err != nil {
				f.err = "please enter a valid number"
				return f, nil
			}
		}
		f.textVals[s.key] = val
		return f.advance()
	}
	var cmd tea.Cmd
	f.input, cmd = f.input.Update(key)
	return f, cmd
}

func (f stepForm) updateSelect(key tea.KeyMsg) (stepForm, tea.Cmd) {
	s := f.steps[f.idx]
	switch key.String() {
	case "up", "k":
		if f.cursor > 0 {
			f.cursor--
		}
	case "down", "j":
		if f.cursor < len(s.options)-1 {
			f.cursor++
		}
	case "enter":
		f.selVals[s.key] = s.options[f.cursor]
		return f.advance()
	}
	return f, nil
}

func (f stepForm) updateMulti(key tea.KeyMsg) (stepForm, tea.Cmd) {
	s := f.steps[f.idx]
	switch key.String() {
	case "up", "k":
		if f.cursor > 0 {
			f.cursor--
		}
	case "down", "j":
		if f.cursor < len(s.options)-1 {
			f.cursor++
		}
	case " ":
		f.checked[f.cursor] = !f.checked[f.cursor]
	case "enter":
		var chosen []string
		for i, opt := range s.options {
			if f.checked[i] {
				chosen = append(chosen, opt)
			}
		}
		f.multiVals[s.key] = chosen
		return f.advance()
	}
	return f, nil
}

func (f stepForm) advance() (stepForm, tea.Cmd) {
	f.idx++
	if f.idx >= len(f.steps) {
		f.done = true
		return f, nil
	}
	f.prepareStep()
	return f, textinput.Blink
}

func (f stepForm) View() string {
	if f.done || f.canceled {
		return ""
	}
	s := f.steps[f.idx]

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("%s  (step %d/%d)", f.title, f.idx+1, len(f.steps))))
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
		b.WriteString(f.input.View())
	case kindSelect:
		for i, opt := range s.options {
			b.WriteString(renderRadio(i == f.cursor, opt))
			b.WriteString("\n")
		}
	case kindMulti:
		for i, opt := range s.options {
			b.WriteString(renderCheckbox(i == f.cursor, f.checked[i], opt))
			b.WriteString("\n")
		}
	}

	if f.err != "" {
		b.WriteString("\n")
		b.WriteString(errStyle.Render("! " + f.err))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(navHelp(s.kind)))
	return b.String()
}

// --- value accessors ---

func (f stepForm) textVal(key string) string    { return f.textVals[key] }
func (f stepForm) selVal(key string) string     { return f.selVals[key] }
func (f stepForm) multiVal(key string) []string { return f.multiVals[key] }

// floatVal parses a numeric text value; empty or invalid yields fallback.
func (f stepForm) floatVal(key string, fallback float64) float64 {
	v := strings.TrimSpace(f.textVals[key])
	if v == "" {
		return fallback
	}
	out, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return out
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
