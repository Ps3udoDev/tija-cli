package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/review"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func TestPRContextFreshReview(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"), testutil.WithBranch("feat/review-citation"))
	commitFile(t, repo, "src/new.ts", "export const x = 1\n", "feat(review): add source")
	p := plan(t, repo)
	apply(t, p, nil, approveAll(p))
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-m", "chore: initialize")
	head := testutil.Git(t, repo, "rev-parse", "HEAD")
	for _, tc := range []struct {
		head        string
		dirty, want bool
	}{{head, false, true}, {"stale", false, false}, {head, true, false}} {
		if err := review.SaveSession(repo, review.Result{Commit: tc.head, Dirty: tc.dirty, Mode: "deep"}); err != nil {
			t.Fatal(err)
		}
		c := collect(t, repo)
		if (c.Review != nil) != tc.want {
			t.Fatalf("session %+v: review %+v", tc, c.Review)
		}
	}
}

func reviewContextForPR() PRContext {
	return PRContext{Branch: "feat/review-citation", Target: "main", Manifest: manifest.Manifest{PullRequest: manifest.PullRequest{Title: manifest.Title{MaxLength: 100}, RequiredSections: []string{"Pruebas"}}}, Template: "## Pruebas\n", Review: &review.Result{Mode: "deep", Summary: map[review.Severity]int{review.Medium: 2}}}
}

func TestPRPromptCitesReview(t *testing.T) {
	c := reviewContextForPR()
	prompt := BuildPRPrompt(c, nil)
	if !strings.Contains(prompt, "`tija review --deep`: 0 critical, 0 high, 2 medium, 0 low") || !strings.Contains(prompt, "Do not copy findings") {
		t.Fatal(prompt)
	}
	c.Review = nil
	if strings.Contains(BuildPRPrompt(c, nil), "Verified review for this commit") {
		t.Fatal("absent session cited")
	}
}

func TestDeterministicDraftCitesReview(t *testing.T) {
	c := reviewContextForPR()
	for _, mode := range []string{"review", "quality", "deep"} {
		c.Review.Mode = mode
		body := DeterministicDraft(c).Body
		if !strings.Contains(body, reviewCitation(c.Review)) {
			t.Fatal(body)
		}
	}
}

func TestAgentDraftCitesReviewAfterEnforcement(t *testing.T) {
	c := reviewContextForPR()
	body := enforceVerifiedTests("## Pruebas\n\nInvented tests\n\n## Evidencias\n\nN/A", "es", nil, c.Review)
	if !strings.Contains(body, reviewCitation(c.Review)) || strings.Contains(body, "Invented") {
		t.Fatal(body)
	}
}

func TestPreparePRAgentCitesReview(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"), testutil.WithBranch("feat/review-citation"))
	commitFile(t, repo, "src/new.ts", "export const x = 1\n", "feat(review): add source")
	p := plan(t, repo)
	apply(t, p, nil, approveAll(p))
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-m", "chore: initialize")
	r := review.Result{Mode: "deep", Commit: testutil.Git(t, repo, "rev-parse", "HEAD"), Summary: map[review.Severity]int{review.Medium: 2}}
	if err := review.SaveSession(repo, r); err != nil {
		t.Fatal(err)
	}
	env, _ := fakeAgentEnv(agentReply{out: validAgentDraft})
	report, err := PreparePR(context.Background(), repo, env, PRRequest{NoCheck: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.Body, reviewCitation(&r)) {
		t.Fatal(report.Body)
	}
}
