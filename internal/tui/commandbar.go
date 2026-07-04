package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// The command bar is the small in-app terminal: pressing ':' on any browse
// screen opens a one-line prompt for quick scripting. Every command is also
// reachable through plain navigation.

// CommandKind identifies a command-bar verb.
type CommandKind int

const (
	CmdNew CommandKind = iota
	CmdOpen
	CmdClose
	CmdHelp
	CmdQuit
)

// Command is a parsed command-bar input.
type Command struct {
	Kind CommandKind
	ID   int64 // session id for open/close
}

const commandHelp = "commands: new · open <id> · close <id> · help · quit"

// ParseCommand parses one command-bar line. Pure function; exported so the
// black-box tests under tests/ can exercise it.
func ParseCommand(input string) (Command, error) {
	fields := strings.Fields(strings.TrimSpace(input))
	if len(fields) == 0 {
		return Command{}, fmt.Errorf("empty command (%s)", commandHelp)
	}
	verb, args := strings.ToLower(fields[0]), fields[1:]

	needID := func(kind CommandKind) (Command, error) {
		if len(args) != 1 {
			return Command{}, fmt.Errorf("usage: %s <session-id>", verb)
		}
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || id <= 0 {
			return Command{}, fmt.Errorf("invalid session id %q", args[0])
		}
		return Command{Kind: kind, ID: id}, nil
	}
	noArgs := func(kind CommandKind) (Command, error) {
		if len(args) != 0 {
			return Command{}, fmt.Errorf("%s takes no arguments", verb)
		}
		return Command{Kind: kind}, nil
	}

	switch verb {
	case "new":
		return noArgs(CmdNew)
	case "open":
		return needID(CmdOpen)
	case "close":
		return needID(CmdClose)
	case "help", "?":
		return noArgs(CmdHelp)
	case "quit", "q", "exit":
		return noArgs(CmdQuit)
	default:
		return Command{}, fmt.Errorf("unknown command %q (%s)", verb, commandHelp)
	}
}

// cmdbar is the bottom-of-screen command input.
type cmdbar struct {
	input  textinput.Model
	active bool
}

func newCmdbar() cmdbar {
	ti := textinput.New()
	ti.Prompt = ":"
	return cmdbar{input: ti}
}

func (c *cmdbar) open() tea.Cmd {
	c.active = true
	c.input.SetValue("")
	return c.input.Focus()
}

func (c *cmdbar) close() {
	c.active = false
	c.input.Blur()
}

// update handles a key while the bar is active. When the user submits a line,
// submitted is true and line holds the raw input.
func (c cmdbar) update(msg tea.Msg) (bar cmdbar, submitted bool, line string, cmd tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			c.close()
			return c, false, "", nil
		case "enter":
			line = c.input.Value()
			c.close()
			return c, true, line, nil
		}
	}
	c.input, cmd = c.input.Update(msg)
	return c, false, "", cmd
}

func (c cmdbar) view() string {
	if !c.active {
		return ""
	}
	return c.input.View()
}
