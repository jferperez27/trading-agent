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

// Market category values.
const (
	MarketFutures = "futures"
	MarketStocks  = "stocks"
	MarketForex   = "forex"
	MarketCrypto  = "crypto"
	MarketOther   = "other"
)

// Markets is the ordered set of valid market categories.
var Markets = []string{MarketFutures, MarketStocks, MarketForex, MarketCrypto, MarketOther}

// Session is a single backtesting session.
type Session struct {
	ID             int64
	UID            string // unique id generated at creation
	Name           string
	Market         string // one of Markets
	StartedAt      time.Time
	EndedAt        *time.Time // nil until closed
	Instrument     string
	InitialBalance float64
	Notes          string
	ClosedMeta     string // JSON (ClosedMeta); empty until the session is closed
}

// SessionParams are the user-supplied fields for creating a session.
type SessionParams struct {
	Name           string
	Market         string
	Instrument     string
	InitialBalance float64
	Notes          string
}

// Trade is one logged trade within a session.
type Trade struct {
	ID             int64
	SessionID      int64
	EntryPrice     float64
	ExitPrice      float64
	StopLoss       float64
	Size           float64 // contracts x point value; cash pnl multiplier
	Direction      string
	SetupType      string
	PnL            float64 // price points
	PnLCash        float64 // PnL x Size
	RMultiple      float64
	ScreenshotPath string // relative to data_dir; empty if none
	LeakTags       []string
	Notes          string
	CreatedAt      time.Time
}

// Stats are live aggregates for a session, computed from its trades. Used by
// the TUI dashboard header, the sessions menu, and close-time metadata.
type Stats struct {
	TradeCount     int
	Wins           int // trades with positive pnl
	Losses         int // trades with negative pnl (breakeven counts as neither)
	TotalPnLPoints float64
	TotalPnLCash   float64
	WinRate        float64 // wins / trade count
	AvgRMultiple   float64
	InitialBalance float64
	CurrentBalance float64 // initial + total cash pnl
}

// ClosedMeta is the metadata generated and stored (as JSON) when a session is
// closed.
type ClosedMeta struct {
	ClosedAt        string  `json:"closed_at"` // RFC3339
	DurationSeconds int64   `json:"duration_seconds"`
	TradeCount      int     `json:"trade_count"`
	Wins            int     `json:"wins"`
	Losses          int     `json:"losses"`
	TotalPnLPoints  float64 `json:"total_pnl_points"`
	TotalPnLCash    float64 `json:"total_pnl_cash"`
	WinRate         float64 `json:"win_rate"`
	AvgRMultiple    float64 `json:"avg_r_multiple"`
	FinalBalance    float64 `json:"final_balance"`
}

// ComputeStats aggregates session stats from the trades. Pure function so the
// math is unit-testable independent of the database.
func ComputeStats(initialBalance float64, trades []Trade) Stats {
	s := Stats{
		TradeCount:     len(trades),
		InitialBalance: initialBalance,
		CurrentBalance: initialBalance,
	}
	if len(trades) == 0 {
		return s
	}
	var rSum float64
	for _, t := range trades {
		switch {
		case t.PnL > 0:
			s.Wins++
		case t.PnL < 0:
			s.Losses++
		}
		s.TotalPnLPoints += t.PnL
		s.TotalPnLCash += t.PnLCash
		rSum += t.RMultiple
	}
	n := float64(len(trades))
	s.WinRate = float64(s.Wins) / n
	s.AvgRMultiple = rSum / n
	s.CurrentBalance = initialBalance + s.TotalPnLCash
	return s
}

// SessionListRow is a session augmented with trade aggregates for list output.
type SessionListRow struct {
	Session
	TradeCount   int
	TotalPnLCash float64
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
