package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"tradectl/internal/analysis"
	"tradectl/internal/claude"
	"tradectl/internal/cost"
	"tradectl/internal/store"
)

func newAnalyzeCmd() *cobra.Command {
	var (
		tradeID   int64
		sessionID int64
		model     string
	)
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Run Claude analysis on a trade or a full session",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if (tradeID == 0) == (sessionID == 0) {
				return fmt.Errorf("specify exactly one of --trade or --session")
			}

			a, err := loadApp()
			if err != nil {
				return err
			}
			defer a.st.Close()

			// Resolve the API key up front so we fail clearly before doing work.
			apiKey, err := a.cfg.APIKey()
			if err != nil {
				return err
			}
			analyzer := claude.New(apiKey)
			ctx := context.Background()

			if tradeID != 0 {
				return analyzeTrade(ctx, a, analyzer, tradeID, model)
			}
			return analyzeSession(ctx, a, analyzer, sessionID, model)
		},
	}
	cmd.Flags().Int64Var(&tradeID, "trade", 0, "analyze a single trade by ID")
	cmd.Flags().Int64Var(&sessionID, "session", 0, "analyze a full session by ID")
	cmd.Flags().StringVar(&model, "model", "", "override the default model for this analysis")
	return cmd
}

func analyzeTrade(ctx context.Context, a *app, analyzer *claude.Analyzer, tradeID int64, modelOverride string) error {
	trade, err := a.st.GetTrade(tradeID)
	if err != nil {
		return err
	}
	model := firstNonEmpty(modelOverride, a.cfg.DefaultModelTrade)

	res, err := analyzer.AnalyzeTrade(ctx, model, trade)
	if err != nil {
		return err
	}

	costUSD, err := cost.Compute(res.Model, res.Usage)
	if err != nil {
		return err
	}

	// Per-trade summary is optional in Sprint 1; store NULL.
	analysisID, err := a.st.InsertAnalysis(store.Analysis{
		Scope:        store.ScopeTrade,
		TargetID:     tradeID,
		ModelUsed:    res.Model,
		InputTokens:  res.Usage.TotalInputTokens(),
		OutputTokens: res.Usage.OutputTokens,
		CostUSD:      costUSD,
		ResultText:   res.Text,
		Summary:      "",
	})
	if err != nil {
		return err
	}

	fmt.Printf("Trade %d analysis:\n\n%s\n", tradeID, res.Text)
	printUsage(res, costUSD, analysisID)
	return nil
}

func analyzeSession(ctx context.Context, a *app, analyzer *claude.Analyzer, sessionID int64, modelOverride string) error {
	sess, err := a.st.GetSession(sessionID)
	if err != nil {
		return err
	}
	trades, err := a.st.GetSessionTrades(sessionID)
	if err != nil {
		return err
	}
	if len(trades) == 0 {
		return fmt.Errorf("session %d has no trades to analyze", sessionID)
	}
	model := firstNonEmpty(modelOverride, a.cfg.DefaultModelSession)

	res, err := analyzer.AnalyzeSession(ctx, model, sess, trades)
	if err != nil {
		return err
	}

	costUSD, err := cost.Compute(res.Model, res.Usage)
	if err != nil {
		return err
	}

	// The model wraps its takeaway in <verdict>...</verdict>; pull it out for the
	// structured summary and strip it from the human-readable critique. Win rate
	// and avg R are computed from the trades (ground truth), not the model.
	verdict, critique := claude.ExtractVerdict(res.Text)
	summaryJSON, err := analysis.BuildSummary(trades, verdict)
	if err != nil {
		return err
	}

	analysisID, err := a.st.InsertAnalysis(store.Analysis{
		Scope:        store.ScopeSession,
		TargetID:     sessionID,
		ModelUsed:    res.Model,
		InputTokens:  res.Usage.TotalInputTokens(),
		OutputTokens: res.Usage.OutputTokens,
		CostUSD:      costUSD,
		ResultText:   critique,
		Summary:      summaryJSON,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Session %d analysis (%d trades):\n\n%s\n", sessionID, len(trades), critique)
	fmt.Printf("\nSummary: %s\n", summaryJSON)
	printUsage(res, costUSD, analysisID)
	return nil
}

func printUsage(res claude.Result, costUSD float64, analysisID int64) {
	fmt.Printf("\n— model=%s  input=%d output=%d (cache_read=%d cache_write=%d)  cost=$%.6f  [analysis #%d]\n",
		res.Model, res.Usage.TotalInputTokens(), res.Usage.OutputTokens,
		res.Usage.CacheReadInputTokens, res.Usage.CacheCreationInputTokens, costUSD, analysisID)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
