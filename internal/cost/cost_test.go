package cost

import (
	"math"
	"testing"
)

func TestCompute(t *testing.T) {
	const eps = 1e-12
	cases := []struct {
		name  string
		model string
		usage Usage
		want  float64
	}{
		{
			// (1000*$1 + 500*$5) / 1e6
			name:  "haiku uncached",
			model: "claude-haiku-4-5-20251001",
			usage: Usage{InputTokens: 1000, OutputTokens: 500},
			want:  0.0035,
		},
		{
			// (2000*$3 + 1000*$15) / 1e6
			name:  "sonnet uncached",
			model: "claude-sonnet-4-6",
			usage: Usage{InputTokens: 2000, OutputTokens: 1000},
			want:  0.021,
		},
		{
			// (100*$1 + 900*$1*0.1 + 100*$5) / 1e6 — cache reads at 10%
			name:  "haiku with cache read",
			model: "claude-haiku-4-5-20251001",
			usage: Usage{InputTokens: 100, CacheReadInputTokens: 900, OutputTokens: 100},
			want:  0.00069,
		},
		{
			// (100*$1*1.25 + 100*$5) / 1e6 — cache writes at 1.25x
			name:  "haiku with cache write",
			model: "claude-haiku-4-5",
			usage: Usage{CacheCreationInputTokens: 100, OutputTokens: 100},
			want:  0.000625,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Compute(c.model, c.usage)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(got-c.want) > eps {
				t.Errorf("Compute = %v, want %v", got, c.want)
			}
		})
	}
}

func TestComputeUnknownModel(t *testing.T) {
	if _, err := Compute("gpt-4", Usage{InputTokens: 10}); err == nil {
		t.Fatal("expected error for unknown model, got nil")
	}
}

func TestTotalInputTokens(t *testing.T) {
	u := Usage{InputTokens: 10, CacheReadInputTokens: 20, CacheCreationInputTokens: 5}
	if got := u.TotalInputTokens(); got != 35 {
		t.Errorf("TotalInputTokens = %d, want 35", got)
	}
}
