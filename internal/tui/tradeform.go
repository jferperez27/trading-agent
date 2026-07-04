package tui

import (
	"strconv"

	"tradectl/internal/store"
)

// LogResult holds the raw values captured by the trade form. Numeric fields
// are pre-parsed; pnl/r_multiple/pnl_cash are computed downstream by the store.
type LogResult struct {
	SessionID      int64
	Direction      string
	Entry          float64
	Exit           float64
	Stop           float64
	Size           float64
	Setup          string
	Leaks          []string
	Notes          string
	ScreenshotPath string // optional source path, as typed by the user
}

// tradeSteps builds the trade form's step list. The session step is only
// included for the standalone `tradectl log` flow; inside the session
// dashboard the session is implied by context.
func tradeSteps(includeSession bool) []step {
	steps := []step{
		{key: "direction", label: "Direction", kind: kindSelect, options: []string{store.DirectionLong, store.DirectionShort}},
		{key: "entry", label: "Entry price", kind: kindText, numeric: true},
		{key: "exit", label: "Exit price", kind: kindText, numeric: true},
		{key: "stop", label: "Stop loss", kind: kindText, numeric: true},
		{key: "size", label: "Size", help: "contracts × point value; cash pnl = points × size (default 1)", kind: kindText, numeric: true, optional: true},
		{key: "setup", label: "Setup type", kind: kindSelect, options: []string{store.SetupORB, store.SetupFVG, store.SetupOther}},
		{key: "leaks", label: "Leak tags", help: "space to toggle, enter to confirm (none = leave all unchecked)", kind: kindMulti, options: store.LeakTags},
		{key: "notes", label: "Notes", help: "optional, free text", kind: kindText, optional: true},
		{key: "screenshot", label: "Screenshot path", help: "optional; leave blank for none", kind: kindText, optional: true},
	}
	if includeSession {
		steps = append([]step{
			{key: "session_id", label: "Session ID", help: "trade belongs to this session", kind: kindText, numeric: true},
		}, steps...)
	}
	return steps
}

// newTradeForm builds the embedded (dashboard) trade form: no session step.
// Size is intentionally not prefilled: an empty answer falls back to 1, and a
// prefilled "1" would corrupt typed input (typing "20" would yield "120").
func newTradeForm() stepForm {
	return newStepForm("Log trade", tradeSteps(false), nil)
}

// newStandaloneTradeForm builds the `tradectl log` form, which asks for the
// session ID first (prefilled with the latest open session when > 0).
func newStandaloneTradeForm(defaultSessionID int64) stepForm {
	defaults := map[string]string{}
	if defaultSessionID > 0 {
		defaults["session_id"] = strconv.FormatInt(defaultSessionID, 10)
	}
	return newStepForm("Log trade", tradeSteps(true), defaults)
}

// tradeResult extracts the LogResult from a completed trade form. sessionID
// overrides the form value when > 0 (the dashboard supplies it from context).
func tradeResult(f stepForm, sessionID int64) LogResult {
	if sessionID == 0 {
		sessionID, _ = strconv.ParseInt(f.textVal("session_id"), 10, 64)
	}
	return LogResult{
		SessionID:      sessionID,
		Direction:      f.selVal("direction"),
		Entry:          f.floatVal("entry", 0),
		Exit:           f.floatVal("exit", 0),
		Stop:           f.floatVal("stop", 0),
		Size:           f.floatVal("size", 1),
		Setup:          f.selVal("setup"),
		Leaks:          f.multiVal("leaks"),
		Notes:          f.textVal("notes"),
		ScreenshotPath: f.textVal("screenshot"),
	}
}
