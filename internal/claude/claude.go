// Package claude wraps the Anthropic Go SDK for tradectl's per-trade and
// per-session analysis. The static system prompt (trading rules, leak
// definitions, ORB/FVG definitions) is marked for prompt caching so the
// repeated portion is billed at ~10% on subsequent calls.
package claude

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"tradectl/internal/cost"
	"tradectl/internal/store"
)

// maxTokens caps output for both analysis modes. Critiques are short.
const maxTokens = 1500

// systemPrompt is the static, cached block: stable trading context shared by
// every call. Only the per-trade / per-session data varies per request, so this
// whole block is a cache prefix.
const systemPrompt = `You are a disciplined trading coach reviewing FxReplay backtest sessions for a
single trader. You give honest, specific, no-fluff critique. You are reviewing
backtests (not live trades), so there is no risk-of-ruin lecture needed — focus
on execution discipline and recurring behavioral leaks.

SETUP DEFINITIONS
- ORB (Opening Range Breakout): a trade taken on a break of the high or low of
  the defined opening range, in the direction of the break.
- FVG (Fair Value Gap): a trade taken into or from a three-candle imbalance
  (the gap between candle 1's wick and candle 3's wick), used as an entry zone
  or target.
- other: any setup that is neither a clean ORB nor FVG.

KNOWN LEAK CATEGORIES (these are the behavioral mistakes we track over time)
- moved_entry: the trader moved their entry limit order after placing it,
  chasing price instead of letting the setup come to them.
- re_entry_after_stop: the trader re-entered the same idea immediately after
  being stopped out, often revenge-trading rather than waiting for a fresh setup.
- set_and_forget_break: the trader interfered with a trade after entry (moved
  the stop, took profit early, or added) instead of letting the original plan
  play out.
- other: a leak that does not fit the categories above.

PERFORMANCE CONVENTIONS
- pnl is expressed in price points (entry/exit/stop are raw prices; position
  size is not tracked).
- r_multiple = pnl / |entry - stop|. A trade that hit its stop has a negative
  R-multiple. A win rate is the fraction of trades with positive pnl.

Be concrete. Reference the actual trades and tags provided. Never invent trades
or tags that were not given to you.`

// Analyzer holds an Anthropic client.
type Analyzer struct {
	client anthropic.Client
}

// New constructs an Analyzer with the given API key.
func New(apiKey string) *Analyzer {
	return &Analyzer{client: anthropic.NewClient(option.WithAPIKey(apiKey))}
}

// Result is the outcome of an analysis call.
type Result struct {
	Text  string     // human-readable critique
	Model string     // resolved model string used
	Usage cost.Usage // token usage for cost computation
}

// AnalyzeTrade runs a lightweight per-trade pattern check.
func (a *Analyzer) AnalyzeTrade(ctx context.Context, model string, t store.Trade) (Result, error) {
	prompt := "Review this single backtest trade. In 3-5 sentences: does it match any known leak " +
		"signature, and what is your quick observation? Be direct.\n\n" + formatTrade(t)
	return a.call(ctx, model, prompt)
}

// AnalyzeSession runs a full per-session critique. The instruction asks the
// model to end with a one-line verdict wrapped in <verdict>...</verdict> tags,
// which the caller extracts for the structured summary.
func (a *Analyzer) AnalyzeSession(ctx context.Context, model string, sess store.Session, trades []store.Trade) (Result, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Critique this full backtest session. Cover: aggregate performance, the leak "+
		"patterns you actually observe in these trades, and concrete recommendations for the next "+
		"session.\n\nEnd your response with a single one-line takeaway wrapped exactly like this: "+
		"<verdict>your one-line takeaway</verdict>\n\n")
	fmt.Fprintf(&b, "SESSION: id=%d instrument=%s started=%s trades=%d\n",
		sess.ID, sess.Instrument, sess.StartedAt.Format("2006-01-02 15:04"), len(trades))
	if sess.Notes != "" {
		fmt.Fprintf(&b, "Session notes: %s\n", sess.Notes)
	}
	b.WriteString("\nTRADES:\n")
	for _, t := range trades {
		b.WriteString(formatTrade(t))
		b.WriteString("\n")
	}
	return a.call(ctx, model, b.String())
}

func (a *Analyzer) call(ctx context.Context, model, prompt string) (Result, error) {
	resp, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		System: []anthropic.TextBlockParam{{
			Text:         systemPrompt,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return Result{}, fmt.Errorf("anthropic request failed: %w", err)
	}

	var text strings.Builder
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(tb.Text)
		}
	}

	return Result{
		Text:  strings.TrimSpace(text.String()),
		Model: string(resp.Model),
		Usage: cost.Usage{
			InputTokens:              resp.Usage.InputTokens,
			OutputTokens:             resp.Usage.OutputTokens,
			CacheReadInputTokens:     resp.Usage.CacheReadInputTokens,
			CacheCreationInputTokens: resp.Usage.CacheCreationInputTokens,
		},
	}, nil
}

// formatTrade renders a trade as a compact, model-friendly line block.
func formatTrade(t store.Trade) string {
	leaks := "none"
	if len(t.LeakTags) > 0 {
		leaks = strings.Join(t.LeakTags, ", ")
	}
	notes := t.Notes
	if notes == "" {
		notes = "(none)"
	}
	return fmt.Sprintf(
		"Trade #%d: %s %s | entry=%.2f exit=%.2f stop=%.2f | pnl=%.2f pts r=%.2fR | leaks=[%s] | notes: %s",
		t.ID, t.Direction, t.SetupType, t.EntryPrice, t.ExitPrice, t.StopLoss,
		t.PnL, t.RMultiple, leaks, notes)
}
