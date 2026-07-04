package claude

import (
	"regexp"
	"strings"
)

// verdictRE matches the one-line takeaway the session prompt asks the model to
// wrap in <verdict>...</verdict> (see AnalyzeSession). Kept in this package so
// the tag convention and its parser stay in sync.
var verdictRE = regexp.MustCompile(`(?s)<verdict>(.*?)</verdict>`)

// ExtractVerdict pulls the verdict out of a session-analysis response and
// returns it along with the response stripped of the tag. If no tag is present,
// verdict is empty and cleaned is the trimmed original.
func ExtractVerdict(text string) (verdict, cleaned string) {
	m := verdictRE.FindStringSubmatch(text)
	if m == nil {
		return "", strings.TrimSpace(text)
	}
	verdict = strings.TrimSpace(m[1])
	cleaned = strings.TrimSpace(verdictRE.ReplaceAllString(text, ""))
	return verdict, cleaned
}
