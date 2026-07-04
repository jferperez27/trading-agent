package tui

import (
	"fmt"
	"math"

	"tradectl/internal/store"
)

// newSessionForm builds the session-creation form: name, market category,
// instrument, initial money, optional notes.
func newSessionForm() stepForm {
	steps := []step{
		{key: "name", label: "Session name", help: "e.g. \"NQ morning ORB practice\"", kind: kindText},
		{key: "market", label: "Market", kind: kindSelect, options: store.Markets},
		{key: "instrument", label: "Instrument", help: "specific ticker, e.g. NQ, ES, EURUSD", kind: kindText},
		{key: "balance", label: "Initial money", help: "starting balance for this session, e.g. 50000", kind: kindText, numeric: true},
		{key: "notes", label: "Notes", help: "optional, free text", kind: kindText, optional: true},
	}
	return newStepForm("New session", steps, nil)
}

// sessionParams extracts store.SessionParams from a completed session form.
func sessionParams(f stepForm) store.SessionParams {
	return store.SessionParams{
		Name:           f.textVal("name"),
		Market:         f.selVal("market"),
		Instrument:     f.textVal("instrument"),
		InitialBalance: f.floatVal("balance", 0),
		Notes:          f.textVal("notes"),
	}
}

// formatMoney renders a cash value like "+$1,234.50" / "-$97.25" / "$0.00".
// (Comma grouping kept simple: none — personal tool, values stay readable.)
func formatMoney(v float64) string {
	sign := ""
	switch {
	case v > 0:
		sign = "+"
	case v < 0:
		sign = "-"
	}
	return fmt.Sprintf("%s$%.2f", sign, math.Abs(v))
}
