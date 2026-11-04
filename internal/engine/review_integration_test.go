package engine

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/review"
)

func TestReviewDeepSynthesizesVerifiedArchitectureFindings(t *testing.T) {
	dir := reviewRepo(t, true)
	writeReviewFile(t, dir, ".tija/rules/architecture/shared.md", "---\nname: shared\nappliesTo: ['*.ts']\n---\n## shared-bug: Shared rule\nCheck this problem in both passes.\n")
	writeReviewFile(t, dir, "a.ts", "const bad = true\n")
	env, _ := fakeAgentEnv()
	var calls atomic.Int32
	env.RunInput = func(_ context.Context, _, input, _ string, args ...string) (string, string, int, error) {
		severity := "Medium"
		if calls.Add(1) == 1 {
			severity = "High"
		}
		if !strings.Contains(input, "## shared-bug:") {
			t.Error("architecture rule missing from pass")
		}
		return `{"findings":[{"file":"a.ts","line":1,"severity":"` + severity + `","rule":"shared-bug","title":"Shared bug","explanation":"Verified issue","evidence":"const bad = true"}]}`, "", 0, nil
	}
	r := runReviewTest(t, dir, env, ReviewRequest{Mode: "deep", Agent: "claude", NoCheck: true, NoPRComments: true})
	if calls.Load() != 2 || len(r.Findings) != 1 || r.Exit != 1 || len(r.Discarded) != 0 {
		t.Fatalf("calls=%d report=%+v", calls.Load(), r)
	}
	f := r.Findings[0]
	if f.Severity != review.High || !f.Disputed || len(f.ConfirmedBy) != 2 || len(f.Severities) != 2 || len(f.Passes) != 2 {
		t.Fatalf("%+v", f)
	}
}

func TestReviewFailedAgentRetainsDeterministicFindings(t *testing.T) {
	dir := reviewRepo(t, true)
	writeReviewFile(t, dir, ".tija/rules/architecture/check.md", "---\nname: check\nchecks:\n  - id: seeded-check\n    severity: High\n    files: '*.ts'\n    forbid: bad\n    message: Seeded violation\n---\n")
	writeReviewFile(t, dir, "a.ts", "const bad = true\n")
	env, calls := fakeAgentEnv(agentReply{out: "bad JSON"}, agentReply{out: "still bad JSON"})
	r := runReviewTest(t, dir, env, ReviewRequest{Agent: "claude", NoCheck: true, NoPRComments: true})
	if len(*calls) != 2 || r.Exit != 2 || r.Chunks.Failed != 1 || len(r.Findings) != 1 || r.Findings[0].Source != "check" || r.Findings[0].Rule != "seeded-check" {
		t.Fatalf("%+v", r)
	}
	saved, ok, err := review.LoadSession(dir)
	if err != nil || !ok || saved.Exit != 2 || len(saved.Findings) != 1 {
		t.Fatalf("saved=%+v ok=%v err=%v", saved, ok, err)
	}
}
