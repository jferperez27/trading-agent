package store

import (
	"math"
	"strings"
	"time"
)

// Direction values.
const (
	DirectionLong  = "long"
	DirectionShort = "short"
)

// Setup type values.
const (
	SetupORB   = "ORB"
	SetupFVG   = "FVG"
	SetupOther = "other"
)

// Leak tag enum values.
const (
	LeakMovedEntry        = "moved_entry"
	LeakReEntryAfterStop  = "re_entry_after_stop"
	LeakSetAndForgetBreak = "set_and_forget_break"
	LeakOther             = "other"
)

// LeakTags is the ordered set of valid leak tags.
var LeakTags = []string{LeakMovedEntry, LeakReEntryAfterStop, LeakSetAndForgetBreak, LeakOther}

// Analysis scopes.
const (
	ScopeTrade   = "trade"
	ScopeSession = "session"
)

// Session is a single backtesting session.
type Session struct {
	ID         int64
	StartedAt  time.Time
	EndedAt    *time.Time // nil until closed
	Instrument string
	Notes      string
}

// Trade is one logged trade within a session.
type Trade struct {
	ID             int64
	SessionID      int64
	EntryPrice     float64
	ExitPrice      float64
	StopLoss       float64
	Direction      string
	SetupType      string
	PnL            float64
	RMultiple      float64
	ScreenshotPath string // relative to data_dir; empty if none
	LeakTags       []string
	Notes          string
	CreatedAt      time.Time
}

// Analysis is a logged Claude analysis call.
type Analysis struct {
	ID           int64
	Scope        string
	TargetID     int64
	ModelUsed    string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	ResultText   string
	Summary      string // JSON; empty means NULL
	CreatedAt    time.Time
}

// SessionListRow is a session augmented with its trade count for list output.
type SessionListRow struct {
	Session
	TradeCount int
}

// ComputeMetrics derives pnl (in price points) and the R-multiple from the
// entry/exit/stop and direction. Math is done here so manual entry is never
// trusted. Risk is the absolute distance between entry and stop; R-multiple is
// pnl divided by that risk (so a stopped-out trade yields a negative R).
func ComputeMetrics(direction string, entry, exit, stop float64) (pnl, rMultiple float64) {
	switch direction {
	case DirectionShort:
		pnl = entry - exit
	default: // long
		pnl = exit - entry
	}
	risk := math.Abs(entry - stop)
	if risk == 0 {
		return pnl, 0
	}
	return pnl, pnl / risk
}

// encodeLeakTags joins leak tags into the comma-separated DB representation.
func encodeLeakTags(tags []string) string {
	return strings.Join(tags, ",")
}

// decodeLeakTags splits the comma-separated DB representation, dropping empties.
func decodeLeakTags(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
