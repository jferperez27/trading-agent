// Package tests holds tradectl's test suite, exercising the internal packages
// through their exported APIs (black-box).
package tests

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"tradectl/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestComputeMetrics(t *testing.T) {
	const eps = 1e-9
	cases := []struct {
		name              string
		dir               string
		entry, exit, stop float64
		wantPnL, wantR    float64
	}{
		{"long winner 2R", store.DirectionLong, 100, 110, 95, 10, 2.0},
		{"long stopped out -1R", store.DirectionLong, 100, 95, 95, -5, -1.0},
		{"short winner 2R", store.DirectionShort, 100, 90, 105, 10, 2.0},
		{"short stopped out -1R", store.DirectionShort, 100, 105, 105, -5, -1.0},
		{"zero risk yields zero R", store.DirectionLong, 100, 110, 100, 10, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pnl, r := store.ComputeMetrics(c.dir, c.entry, c.exit, c.stop)
			if math.Abs(pnl-c.wantPnL) > eps {
				t.Errorf("pnl = %v, want %v", pnl, c.wantPnL)
			}
			if math.Abs(r-c.wantR) > eps {
				t.Errorf("r = %v, want %v", r, c.wantR)
			}
		})
	}
}

func TestComputeStats(t *testing.T) {
	const eps = 1e-9
	trades := []store.Trade{
		{PnL: 10, PnLCash: 200, RMultiple: 2},   // win
		{PnL: -5, PnLCash: -100, RMultiple: -1}, // loss
		{PnL: 0, PnLCash: 0, RMultiple: 0},      // breakeven: neither
		{PnL: 5, PnLCash: 100, RMultiple: 1},    // win
	}
	s := store.ComputeStats(50000, trades)

	if s.TradeCount != 4 || s.Wins != 2 || s.Losses != 1 {
		t.Errorf("counts = %d/%d/%d, want 4/2/1", s.TradeCount, s.Wins, s.Losses)
	}
	if math.Abs(s.WinRate-0.5) > eps {
		t.Errorf("win rate = %v, want 0.5", s.WinRate)
	}
	if math.Abs(s.TotalPnLPoints-10) > eps || math.Abs(s.TotalPnLCash-200) > eps {
		t.Errorf("totals = %v pts / %v cash, want 10 / 200", s.TotalPnLPoints, s.TotalPnLCash)
	}
	if math.Abs(s.AvgRMultiple-0.5) > eps {
		t.Errorf("avg r = %v, want 0.5", s.AvgRMultiple)
	}
	if math.Abs(s.CurrentBalance-50200) > eps {
		t.Errorf("balance = %v, want 50200", s.CurrentBalance)
	}
}

func TestComputeStatsEmpty(t *testing.T) {
	s := store.ComputeStats(1000, nil)
	if s.TradeCount != 0 || s.WinRate != 0 || s.CurrentBalance != 1000 {
		t.Errorf("empty stats = %+v", s)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	// The DB file and screenshots dir should exist after Open.
	if _, err := os.Stat(filepath.Join(dir, store.DBFileName)); err != nil {
		t.Fatalf("db file not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, store.ScreenshotsDir)); err != nil {
		t.Fatalf("screenshots dir not created: %v", err)
	}

	// Session lifecycle with detail fields.
	sid, err := st.CreateSession(store.SessionParams{
		Name:           "London open",
		Market:         store.MarketFutures,
		Instrument:     "NQ",
		InitialBalance: 50000,
		Notes:          "morning session",
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, ok, _ := st.LatestOpenSessionID(); !ok {
		t.Fatal("expected an open session")
	}

	sess, err := st.GetSession(sid)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess.Name != "London open" || sess.Market != store.MarketFutures || sess.InitialBalance != 50000 {
		t.Errorf("session fields = %+v", sess)
	}
	if len(sess.UID) != 32 {
		t.Errorf("uid = %q, want 32 hex chars", sess.UID)
	}
	if sess.ClosedMeta != "" {
		t.Errorf("closed_meta should be empty until close, got %q", sess.ClosedMeta)
	}

	// Insert a trade with size; verify computed metrics persisted.
	tradeID, err := st.InsertTrade(store.Trade{
		SessionID:  sid,
		EntryPrice: 100, ExitPrice: 110, StopLoss: 95,
		Size:      20, // e.g. 1 NQ contract at $20/pt
		Direction: store.DirectionLong, SetupType: store.SetupORB,
		LeakTags: []string{store.LeakMovedEntry, store.LeakOther},
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
	if got.Size != 20 || got.PnLCash != 200 {
		t.Errorf("cash = size %v pnl_cash %v, want 20 / 200", got.Size, got.PnLCash)
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

	// List reflects trade aggregates.
	rows, err := st.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(rows) != 1 || rows[0].TradeCount != 1 || rows[0].TotalPnLCash != 200 {
		t.Fatalf("list = %+v, want 1 session, 1 trade, 200 cash", rows)
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
}

func TestCloseSessionWritesMeta(t *testing.T) {
	st := openTestStore(t)

	sid, err := st.CreateSession(store.SessionParams{
		Name: "meta test", Market: store.MarketFutures, Instrument: "NQ", InitialBalance: 10000,
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// One win (+10 pts x 20 = +$200), one loss (-5 pts x 20 = -$100).
	mustInsert := func(entry, exit, stop float64) {
		t.Helper()
		if _, err := st.InsertTrade(store.Trade{
			SessionID: sid, EntryPrice: entry, ExitPrice: exit, StopLoss: stop,
			Size: 20, Direction: store.DirectionLong, SetupType: store.SetupORB,
		}); err != nil {
			t.Fatalf("InsertTrade: %v", err)
		}
	}
	mustInsert(100, 110, 95)
	mustInsert(100, 95, 95)

	if err := st.CloseSession(sid); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	sess, err := st.GetSession(sid)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess.EndedAt == nil {
		t.Fatal("ended_at not set")
	}
	if sess.ClosedMeta == "" {
		t.Fatal("closed_meta not written")
	}

	var meta store.ClosedMeta
	if err := json.Unmarshal([]byte(sess.ClosedMeta), &meta); err != nil {
		t.Fatalf("closed_meta not valid JSON: %v", err)
	}
	if meta.TradeCount != 2 || meta.Wins != 1 || meta.Losses != 1 {
		t.Errorf("meta counts = %+v", meta)
	}
	if meta.TotalPnLCash != 100 || meta.FinalBalance != 10100 {
		t.Errorf("meta cash = total %v final %v, want 100 / 10100", meta.TotalPnLCash, meta.FinalBalance)
	}
	if meta.WinRate != 0.5 {
		t.Errorf("meta win rate = %v, want 0.5", meta.WinRate)
	}
	if meta.DurationSeconds < 0 {
		t.Errorf("duration = %d, want >= 0", meta.DurationSeconds)
	}
	if meta.ClosedAt == "" {
		t.Error("closed_at empty")
	}
}

func TestInsertTradeDefaultSize(t *testing.T) {
	st := openTestStore(t)
	sid, _ := st.CreateSession(store.SessionParams{Name: "s", Market: store.MarketOther, Instrument: "X"})

	id, err := st.InsertTrade(store.Trade{
		SessionID: sid, EntryPrice: 100, ExitPrice: 105, StopLoss: 95,
		Direction: store.DirectionLong, SetupType: store.SetupOther, // Size omitted
	})
	if err != nil {
		t.Fatalf("InsertTrade: %v", err)
	}
	got, _ := st.GetTrade(id)
	if got.Size != 1 || got.PnLCash != 5 {
		t.Errorf("size %v pnl_cash %v, want 1 / 5 (points == cash at size 1)", got.Size, got.PnLCash)
	}
}

func TestUIDsUnique(t *testing.T) {
	st := openTestStore(t)
	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		id, err := st.CreateSession(store.SessionParams{Name: "u", Market: store.MarketOther, Instrument: "X"})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		sess, _ := st.GetSession(id)
		if seen[sess.UID] {
			t.Fatalf("duplicate uid %q", sess.UID)
		}
		seen[sess.UID] = true
	}
}
