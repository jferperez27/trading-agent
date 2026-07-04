package analysis

import (
	"math"
	"reflect"
	"testing"

	"tradectl/internal/store"
)

func TestCompute(t *testing.T) {
	trades := []store.Trade{
		{PnL: 10, RMultiple: 2, LeakTags: []string{store.LeakMovedEntry}},
		{PnL: -5, RMultiple: -1, LeakTags: []string{store.LeakMovedEntry, store.LeakReEntryAfterStop}},
		{PnL: 5, RMultiple: 1},
	}
	got := Compute(trades, "stop chasing entries")

	if math.Abs(got.WinRate-0.6667) > 1e-4 {
		t.Errorf("win rate = %v, want ~0.6667", got.WinRate)
	}
	if math.Abs(got.AvgRMultiple-0.6667) > 1e-4 {
		t.Errorf("avg r = %v, want ~0.6667", got.AvgRMultiple)
	}
	wantLeaks := []string{store.LeakMovedEntry, store.LeakReEntryAfterStop}
	if !reflect.DeepEqual(got.TopLeaks, wantLeaks) {
		t.Errorf("top leaks = %v, want %v", got.TopLeaks, wantLeaks)
	}
	if got.Verdict != "stop chasing entries" {
		t.Errorf("verdict = %q", got.Verdict)
	}
}

func TestComputeEmpty(t *testing.T) {
	got := Compute(nil, "no trades")
	if got.WinRate != 0 || got.AvgRMultiple != 0 {
		t.Errorf("expected zero stats, got %+v", got)
	}
	if got.TopLeaks == nil || len(got.TopLeaks) != 0 {
		t.Errorf("top leaks = %v, want empty non-nil slice", got.TopLeaks)
	}
}

func TestBuildSummaryJSON(t *testing.T) {
	js, err := BuildSummary([]store.Trade{{PnL: 1, RMultiple: 1}}, "v")
	if err != nil {
		t.Fatalf("BuildSummary: %v", err)
	}
	want := `{"win_rate":1,"avg_r_multiple":1,"top_leaks":[],"verdict":"v"}`
	if js != want {
		t.Errorf("json = %s, want %s", js, want)
	}
}
