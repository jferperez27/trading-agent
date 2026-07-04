package tests

import (
	"testing"

	"tradectl/internal/tui"
)

func TestParseCommand(t *testing.T) {
	cases := []struct {
		in      string
		want    tui.Command
		wantErr bool
	}{
		{"new", tui.Command{Kind: tui.CmdNew}, false},
		{"  NEW  ", tui.Command{Kind: tui.CmdNew}, false},
		{"open 3", tui.Command{Kind: tui.CmdOpen, ID: 3}, false},
		{"close 12", tui.Command{Kind: tui.CmdClose, ID: 12}, false},
		{"help", tui.Command{Kind: tui.CmdHelp}, false},
		{"?", tui.Command{Kind: tui.CmdHelp}, false},
		{"quit", tui.Command{Kind: tui.CmdQuit}, false},
		{"q", tui.Command{Kind: tui.CmdQuit}, false},
		{"exit", tui.Command{Kind: tui.CmdQuit}, false},

		{"", tui.Command{}, true},
		{"   ", tui.Command{}, true},
		{"open", tui.Command{}, true},     // missing id
		{"open abc", tui.Command{}, true}, // bad id
		{"open 0", tui.Command{}, true},   // non-positive id
		{"open 1 2", tui.Command{}, true}, // too many args
		{"close", tui.Command{}, true},    // missing id
		{"new 5", tui.Command{}, true},    // new takes no args
		{"banana", tui.Command{}, true},   // unknown verb
	}
	for _, c := range cases {
		got, err := tui.ParseCommand(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseCommand(%q): expected error, got %+v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseCommand(%q): unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseCommand(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}
