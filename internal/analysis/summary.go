// Package analysis builds the compact structured summary stored alongside each
// session-scope analysis. The numeric stats are computed from the trades
// (ground truth) rather than the model; the verdict comes from the model. This
// is the JSON that Sprint 2 will feed into future session analyses for
// longitudinal context — never the full critique text.
package analysis

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"tradectl/internal/store"
)

// maxTopLeaks caps how many leak tags appear in a summary.
const maxTopLeaks = 3

// Summary is the structured session-analysis summary (see MASTER doc).
type Summary struct {
	WinRate      float64  `json:"win_rate"`
	AvgRMultiple float64  `json:"avg_r_multiple"`
	TopLeaks     []string `json:"top_leaks"`
	Verdict      string   `json:"verdict"`
}

// BuildSummary computes win rate, average R-multiple, and the most frequent
// leak tags from the trades, pairs them with the model's verdict, and returns
// the JSON encoding. It returns an error only on JSON marshaling failure.
func BuildSummary(trades []store.Trade, verdict string) (string, error) {
	s := Compute(trades, verdict)
	b, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("marshaling session summary: %w", err)
	}
	return string(b), nil
}

// Compute derives the Summary struct from the trades and verdict. Exposed
// separately from BuildSummary so callers (e.g. Sprint 2 aggregations) can use
// the typed value without re-parsing JSON.
func Compute(trades []store.Trade, verdict string) Summary {
	if len(trades) == 0 {
		return Summary{TopLeaks: []string{}, Verdict: verdict}
	}

	wins := 0
	var rSum float64
	leakCounts := map[string]int{}
	for _, t := range trades {
		if t.PnL > 0 {
			wins++
		}
		rSum += t.RMultiple
		for _, tag := range t.LeakTags {
			leakCounts[tag]++
		}
	}
	n := float64(len(trades))

	return Summary{
		WinRate:      round4(float64(wins) / n),
		AvgRMultiple: round4(rSum / n),
		TopLeaks:     topLeaks(leakCounts, maxTopLeaks),
		Verdict:      verdict,
	}
}

// topLeaks returns up to limit leak tags ordered by frequency (then name for
// stability). Returns an empty (non-nil) slice when no leaks were tagged.
func topLeaks(counts map[string]int, limit int) []string {
	type kv struct {
		tag   string
		count int
	}
	pairs := make([]kv, 0, len(counts))
	for tag, c := range counts {
		pairs = append(pairs, kv{tag, c})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].tag < pairs[j].tag
	})
	out := []string{}
	for i, p := range pairs {
		if i >= limit {
			break
		}
		out = append(out, p.tag)
	}
	return out
}

func round4(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return math.Round(f*10000) / 10000
}
