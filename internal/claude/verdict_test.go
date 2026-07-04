package claude

import "testing"

func TestExtractVerdict(t *testing.T) {
	cases := []struct {
		name             string
		in               string
		verdict, cleaned string
	}{
		{
			name:    "tag present",
			in:      "Great discipline overall.\n<verdict>Stop chasing entries.</verdict>",
			verdict: "Stop chasing entries.",
			cleaned: "Great discipline overall.",
		},
		{
			name:    "no tag",
			in:      "  Just a critique with no verdict tag.  ",
			verdict: "",
			cleaned: "Just a critique with no verdict tag.",
		},
		{
			name:    "multiline verdict",
			in:      "Body.\n<verdict>line one\nline two</verdict>",
			verdict: "line one\nline two",
			cleaned: "Body.",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, cl := ExtractVerdict(c.in)
			if v != c.verdict {
				t.Errorf("verdict = %q, want %q", v, c.verdict)
			}
			if cl != c.cleaned {
				t.Errorf("cleaned = %q, want %q", cl, c.cleaned)
			}
		})
	}
}
