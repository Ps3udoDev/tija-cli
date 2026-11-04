package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/review"
)

func TestRenderReview(t *testing.T) {
	r := engine.ReviewReport{Result: review.Result{Mode: "deep", FailOn: "high", Quality: review.QualitySummary{Status: "passed"}, Findings: []review.Finding{
		{Source: "agent", File: "src/a.ts", Line: 42, Severity: review.High, Rule: "breaking-functionality", Explanation: "Bug", Suggestion: "Fix", ConfirmedBy: []string{"review", "quality"}, Disputed: true, Severities: []review.Severity{review.High, review.Medium}},
		{Source: "check", File: "a.ts", Line: 4, Severity: review.Medium, Rule: "architecture"},
		{Source: "quality", File: "check.log", Severity: review.High, Rule: "quality-check-failed"},
	}, Warnings: []string{"warning"}}}
	r.Finalize()
	var buf bytes.Buffer
	Review(Printer{W: &buf}, r)
	out := buf.String()
	for _, s := range []string{"✗ High", "! Medium", "src/a.ts:42", "[review, quality]", "(disputed: High/Medium)", "check.log  quality-check-failed", "Summary: 0 critical, 2 high, 1 medium, 0 low", "quality: passed · mode: deep", "! warning"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q in %s", s, out)
		}
	}
	if strings.Contains(out, ":0") || strings.Contains(out, "\x1b") {
		t.Fatal(out)
	}
}
