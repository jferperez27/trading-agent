package store

import (
	"math"
	"testing"
)

func TestComputeMetrics(t *testing.T) {
	const eps = 1e-9
	cases := []struct {
		name              string
		dir               string
		entry, exit, stop float64
		wantPnL, wantR    float64
	}{
		{"long winner 2R", DirectionLong, 100, 110, 95, 10, 2.0},
		{"long stopped out -1R", DirectionLong, 100, 95, 95, -5, -1.0},
		{"short winner 2R", DirectionShort, 100, 90, 105, 10, 2.0},
		{"short stopped out -1R", DirectionShort, 100, 105, 105, -5, -1.0},
		{"zero risk yields zero R", DirectionLong, 100, 110, 100, 10, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pnl, r := ComputeMetrics(c.dir, c.entry, c.exit, c.stop)
			if math.Abs(pnl-c.wantPnL) > eps {
				t.Errorf("pnl = %v, want %v", pnl, c.wantPnL)
			}
			if math.Abs(r-c.wantR) > eps {
				t.Errorf("r = %v, want %v", r, c.wantR)
			}
		})
	}
}

func TestLeakTagsRoundTrip(t *testing.T) {
	in := []string{LeakMovedEntry, LeakOther}
	got := decodeLeakTags(encodeLeakTags(in))
	if len(got) != 2 || got[0] != LeakMovedEntry || got[1] != LeakOther {
		t.Fatalf("round trip = %v, want %v", got, in)
	}
	if got := decodeLeakTags(""); got != nil {
		t.Errorf("empty decode = %v, want nil", got)
	}
}
