package review

import (
	"strings"
	"testing"
)

func TestBuildPrompt(t *testing.T) {
	p := BuildPrompt(PromptInput{PackBody: "PACK BODY", Rules: []PromptRule{{ID: "thin", Title: "Thin controllers", Text: "Keep controllers thin."}}, Quality: []string{"`go test`: passed"}, Files: []FileDiff{{Path: "src/a.ts", Text: "+```\n"}}, Handled: []string{"1000-lines-ceiling", "domain"}, Language: "es", Retry: "findings must be an array"})
	last := -1
	for _, s := range []string{"PACK BODY", "## thin: Thin controllers", "`go test`: passed", "src/a.ts", "````diff\n+```\n````", "do not report", "Spanish", `"findings"`} {
		i := strings.Index(p, s)
		if i < 0 || i < last {
			t.Errorf("missing/out of order %q in %s", s, p)
		}
		last = i
	}
	for _, s := range []string{"1000-lines-ceiling", "domain", "only JSON", "Do not modify", "findings must be an array", "Correct the response format", `"endLine"`, `"suggestion"`, `"evidence"`} {
		if !strings.Contains(p, s) {
			t.Errorf("missing %q", s)
		}
	}
	if p = BuildPrompt(PromptInput{}); strings.Contains(p, "Architecture rules") || strings.Contains(p, "Correct the response format") {
		t.Fatalf("optional section: %s", p)
	}
}

func TestFence(t *testing.T) {
	for _, c := range []struct{ s, want string }{{"plain", "```"}, {"```", "````"}, {"a``````b", "```````"}, {"`a``b", "```"}} {
		if got := Fence(c.s); got != c.want {
			t.Errorf("Fence(%q)=%q", c.s, got)
		}
	}
}
