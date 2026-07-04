// Package cost computes the USD cost of an Anthropic API call from token usage
// and a hardcoded rate card.
//
// IMPORTANT: the rate card below is hardcoded as of the values in
// Documentation/01-sprint-1.md. If Anthropic changes pricing, update Rates.
package cost

import "fmt"

// Rate is per-million-token pricing for a model.
type Rate struct {
	InputPerMTok  float64
	OutputPerMTok float64
}

// Pricing multipliers relative to the standard input rate.
const (
	// Cache reads are billed at ~10% of the standard input rate.
	cacheReadMultiplier = 0.10
	// Cache writes (creation) are billed at ~1.25x the standard input rate
	// (5-minute TTL).
	cacheWriteMultiplier = 1.25
)

// Rates maps model strings to their rate card. Both the dated and aliased
// Haiku identifiers are included so a --model override resolves either way.
//
// Update this table if Anthropic changes pricing.
var Rates = map[string]Rate{
	"claude-haiku-4-5-20251001": {InputPerMTok: 1.0, OutputPerMTok: 5.0},
	"claude-haiku-4-5":          {InputPerMTok: 1.0, OutputPerMTok: 5.0},
	"claude-sonnet-4-6":         {InputPerMTok: 3.0, OutputPerMTok: 15.0},
}

// Usage is the token breakdown needed to compute cost. It mirrors the relevant
// fields of the Anthropic SDK's Usage type without importing it, so the cost
// package stays dependency-light.
type Usage struct {
	InputTokens              int64 // uncached input tokens (full price)
	OutputTokens             int64
	CacheReadInputTokens     int64
	CacheCreationInputTokens int64
}

// TotalInputTokens is the full prompt size: uncached + cache reads + cache
// writes. Stored in analyses.input_tokens for informational/observability use.
func (u Usage) TotalInputTokens() int64 {
	return u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
}

// Compute returns the USD cost for a call against the given model, accounting
// for differential pricing of uncached input, cache reads, and cache writes.
// It returns an error for an unknown model so cost is never silently zero.
func Compute(model string, u Usage) (float64, error) {
	rate, ok := Rates[model]
	if !ok {
		return 0, fmt.Errorf("no rate card entry for model %q (update internal/cost.Rates)", model)
	}
	inputCost := float64(u.InputTokens)*rate.InputPerMTok +
		float64(u.CacheReadInputTokens)*rate.InputPerMTok*cacheReadMultiplier +
		float64(u.CacheCreationInputTokens)*rate.InputPerMTok*cacheWriteMultiplier
	outputCost := float64(u.OutputTokens) * rate.OutputPerMTok
	return (inputCost + outputCost) / 1_000_000, nil
}
