package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	// The DB file and screenshots dir should exist after Open.
	if _, err := os.Stat(filepath.Join(dir, DBFileName)); err != nil {
		t.Fatalf("db file not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ScreenshotsDir)); err != nil {
		t.Fatalf("screenshots dir not created: %v", err)
	}

	// Session lifecycle.
	sid, err := st.CreateSession("NQ", "morning session")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, ok, _ := st.LatestOpenSessionID(); !ok {
		t.Fatal("expected an open session")
	}

	// Insert a trade and verify computed metrics persisted.
	tradeID, err := st.InsertTrade(Trade{
		SessionID:  sid,
		EntryPrice: 100, ExitPrice: 110, StopLoss: 95,
		Direction: DirectionLong, SetupType: SetupORB,
		LeakTags: []string{LeakMovedEntry, LeakOther},
		Notes:    "chased the entry",
	})
	if err != nil {
		t.Fatalf("InsertTrade: %v", err)
	}

	got, err := st.GetTrade(tradeID)
	if err != nil {
		t.Fatalf("GetTrade: %v", err)
	}
	if got.PnL != 10 || got.RMultiple != 2 {
		t.Errorf("metrics = pnl %v r %v, want 10 / 2", got.PnL, got.RMultiple)
	}
	if len(got.LeakTags) != 2 {
		t.Errorf("leak tags = %v, want 2", got.LeakTags)
	}

	// Screenshot path round-trip.
	if err := st.SetTradeScreenshot(tradeID, "screenshots/1_1_x.png"); err != nil {
		t.Fatalf("SetTradeScreenshot: %v", err)
	}
	if got, _ := st.GetTrade(tradeID); got.ScreenshotPath != "screenshots/1_1_x.png" {
		t.Errorf("screenshot path = %q", got.ScreenshotPath)
	}

	// List reflects the trade count.
	rows, err := st.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(rows) != 1 || rows[0].TradeCount != 1 {
		t.Fatalf("list = %+v, want 1 session with 1 trade", rows)
	}

	// Close lifecycle: closing twice should error.
	if err := st.CloseSession(sid); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if err := st.CloseSession(sid); err == nil {
		t.Error("expected error closing an already-closed session")
	}
	if err := st.CloseSession(9999); err == nil {
		t.Error("expected error closing a nonexistent session")
	}

	// Analysis insert with NULL summary.
	if _, err := st.InsertAnalysis(Analysis{
		Scope: ScopeTrade, TargetID: tradeID, ModelUsed: "claude-haiku-4-5-20251001",
		InputTokens: 100, OutputTokens: 50, CostUSD: 0.00035, ResultText: "ok",
	}); err != nil {
		t.Fatalf("InsertAnalysis: %v", err)
	}
}
