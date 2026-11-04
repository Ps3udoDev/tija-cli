package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/review"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func reviewRepo(t *testing.T, initialized bool) string {
	t.Helper()
	dir := testutil.NewRepo(t, "", testutil.WithBranch("feat/review"))
	if initialized {
		m := manifest.FromDetection(detect.Detection{ProjectName: "test", Stack: detect.StackUnknown}, nil, "main")
		if e := manifest.Save(dir, m); e != nil {
			t.Fatal(e)
		}
		writeReviewFile(t, dir, ".gitignore", ".tija/session/\n.tija/tmp/\n")
		testutil.Git(t, dir, "add", ".")
		testutil.Git(t, dir, "commit", "-m", "setup")
		// Keep the target at setup so the manifest is not part of the review.
		testutil.Git(t, dir, "branch", "-f", "main", "HEAD")
	}
	return dir
}

func TestReviewQualityReuseOrRun(t *testing.T) {
	dir := prRepo(t)
	head := testutil.Git(t, dir, "rev-parse", "HEAD")
	if e := quality.SaveSession(dir, []quality.StepResult{{Kinds: []quality.Kind{quality.Lint}, Provider: "biome", Command: "biome ci", Commit: head, Status: quality.StatusPassed}}); e != nil {
		t.Fatal(e)
	}
	env, calls := fakeQualityEnv(nil)
	r := runReviewTest(t, dir, env, ReviewRequest{NoAgent: true, NoPRComments: true})
	if !r.Quality.Reused || len(*calls) != 0 || r.Quality.Status != "passed" {
		t.Fatalf("%+v calls %d", r.Quality, len(*calls))
	}
	writeReviewFile(t, dir, "new.ts", "dirty\n")
	createFakeNodeBin(t, dir, "biome")
	env, calls = fakeQualityEnv(func(ctx context.Context, dir string, w io.Writer, name string, args ...string) (int, error) {
		fmt.Fprintln(w, "seeded check failure")
		return 1, nil
	})
	r = runReviewTest(t, dir, env, ReviewRequest{NoAgent: true, NoPRComments: true})
	if r.Quality.Reused || r.Quality.Status != "failed" || len(*calls) == 0 || r.Exit != 1 {
		t.Fatalf("%+v calls %d", r, len(*calls))
	}
	var log string
	for _, f := range r.Findings {
		if f.Rule == "quality-check-failed" {
			log = f.File
		}
	}
	if log == "" || strings.HasPrefix(log, ".tija/") {
		t.Fatalf("invalid log %q", log)
	}
	if data, e := os.ReadFile(filepath.FromSlash(log)); e != nil || !strings.Contains(string(data), "seeded check failure") {
		t.Fatalf("log %s: %v", log, e)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(filepath.FromSlash(log))) })
	s, e := quality.LoadSession(dir)
	if e != nil || s.Results[quality.Lint].Status != quality.StatusPassed {
		t.Fatal("review rewrote quality session")
	}
	env, calls = fakeQualityEnv(nil)
	r = runReviewTest(t, dir, env, ReviewRequest{NoAgent: true, NoCheck: true, NoPRComments: true})
	if len(*calls) != 0 || r.Quality.Status != "skipped" {
		t.Fatalf("%+v", r.Quality)
	}
}

func TestReviewNoAgent(t *testing.T) {
	dir := reviewRepo(t, false)
	writeReviewFile(t, dir, "a.ts", "hello\n")
	r := runReviewTest(t, dir, fakeEnv(), ReviewRequest{NoCheck: true, NoPRComments: true})
	if r.Exit != 0 || len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "no agent CLI found") {
		t.Fatalf("%+v", r)
	}
}

func TestReviewPRComments(t *testing.T) {
	dir := reviewRepo(t, false)
	writeReviewFile(t, dir, "a.ts", "const bad = true\n")
	env, _ := fakeAgentEnv(agentReply{out: validReviewFinding}, agentReply{out: validReviewFinding})
	look, run := env.LookPath, env.Run
	apiCalls := 0
	env.LookPath = func(name string) (string, error) {
		if name == "gh" {
			return "gh", nil
		}
		return look(name)
	}
	env.Run = func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
		if name != "gh" {
			return run(ctx, dir, name, args...)
		}
		if args[0] == "auth" {
			return "", "", 0, nil
		}
		if args[0] == "pr" {
			return `[{"url":"https://github.com/o/r/pull/7","baseRefName":"main"}]`, "", 0, nil
		}
		apiCalls++
		if args[1] != "repos/o/r/pulls/7/comments" {
			t.Error(args)
		}
		return `[[{"path":"a.ts","line":null,"original_line":1,"body":"confirmed","html_url":"https://example.test/1","user":{"login":"reviewer"}},{"path":"other.ts","line":4}]]`, "", 0, nil
	}
	r := runReviewTest(t, dir, env, ReviewRequest{Agent: "claude", NoCheck: true})
	if !r.PRComments.Read || len(r.PRComments.Uncovered) != 1 || len(r.Findings[0].PRComments) != 1 || r.Findings[0].PRComments[0].Author != "reviewer" || apiCalls != 1 {
		t.Fatalf("%+v", r)
	}
	r = runReviewTest(t, dir, env, ReviewRequest{Agent: "claude", NoCheck: true, NoPRComments: true})
	if r.PRComments.Read || apiCalls != 1 || r.Exit != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestReviewNoMergeBase(t *testing.T) {
	dir := reviewRepo(t, true)
	m, e := manifest.Load(dir)
	if e != nil {
		t.Fatal(e)
	}
	m.PullRequest.Target = "missing"
	if e := manifest.Save(dir, m); e != nil {
		t.Fatal(e)
	}
	_, e = RunReview(context.Background(), dir, fakeEnv(), ReviewRequest{NoAgent: true, NoCheck: true}, nil)
	if !errors.Is(e, ErrNoMergeBase) {
		t.Fatal(e)
	}
}
func writeReviewFile(t *testing.T, root, p, s string) {
	t.Helper()
	p = filepath.Join(root, filepath.FromSlash(p))
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(s), 0644); e != nil {
		t.Fatal(e)
	}
}
func runReviewTest(t *testing.T, dir string, env sys.Env, req ReviewRequest) ReviewReport {
	t.Helper()
	r, e := RunReview(context.Background(), dir, env, req, nil)
	if e != nil {
		t.Fatal(e)
	}
	return r
}

const validReviewFinding = `{"findings":[{"file":"a.ts","line":1,"severity":"High","rule":"breaking-functionality","title":"Bug","explanation":"Failure","evidence":"const bad = true"},{"file":"a.ts","line":1,"severity":"High","rule":"breaking-functionality","evidence":"invented"}]}`

func TestReviewNothingToReview(t *testing.T) {
	dir := reviewRepo(t, false)
	var message string
	r, e := RunReview(context.Background(), dir, fakeEnv(), ReviewRequest{NoAgent: true, NoCheck: true, NoPRComments: true}, func(ev ReviewEvent) { message = ev.Message })
	if e != nil || r.Exit != 0 || message != "nothing to review" || len(r.Findings) != 0 {
		t.Fatalf("%+v %v %s", r, e, message)
	}
}
func TestReviewVerifiedAndDiscarded(t *testing.T) {
	dir := reviewRepo(t, true)
	writeReviewFile(t, dir, "a.ts", "const bad = true\n")
	env, calls := fakeAgentEnv(agentReply{out: validReviewFinding})
	r := runReviewTest(t, dir, env, ReviewRequest{Agent: "claude", NoCheck: true, NoPRComments: true})
	if r.Exit != 1 || len(r.Findings) != 1 || len(r.Discarded) != 1 || r.Discarded[0].Reason != "evidence-not-found" {
		t.Fatalf("%+v", r)
	}
	if len(*calls) != 1 || !strings.Contains(strings.Join((*calls)[0].args, " "), "--allowedTools Read,Grep,Glob") {
		t.Fatalf("not read only: %+v", calls)
	}
	if _, ok, e := review.LoadSession(dir); e != nil || !ok {
		t.Fatalf("session: %v %v", ok, e)
	}
	if _, e := os.Stat(filepath.Join(dir, ".tija", "tmp")); !os.IsNotExist(e) {
		t.Fatal("review wrote repo temporary files")
	}
}
func TestReviewUsesWorkingTreeAndUntracked(t *testing.T) {
	dir := reviewRepo(t, false)
	writeReviewFile(t, dir, "committed.ts", "first\n")
	testutil.Git(t, dir, "add", ".")
	testutil.Git(t, dir, "commit", "-m", "change")
	writeReviewFile(t, dir, "README.md", "changed\n")
	writeReviewFile(t, dir, "new.ts", "new\n")
	writeReviewFile(t, dir, "package-lock.json", "{}\n")
	env, calls := fakeAgentEnv(agentReply{out: `{"findings":[]}`})
	r := runReviewTest(t, dir, env, ReviewRequest{Agent: "claude", NoCheck: true, NoPRComments: true})
	for _, p := range []string{"committed.ts", "README.md", "new.ts"} {
		if !strings.Contains((*calls)[0].stdin, p) {
			t.Errorf("missing %s", p)
		}
	}
	if strings.Contains((*calls)[0].stdin, "package-lock.json") {
		t.Fatal("excluded lockfile sent")
	}
	if r.Exit != 0 {
		t.Fatal(r.Exit)
	}
	if _, e := os.Stat(filepath.Join(dir, ".tija")); !os.IsNotExist(e) {
		t.Fatal("uninitialized review wrote .tija")
	}
}
func TestReviewRetryOnce(t *testing.T) {
	for _, success := range []bool{false, true} {
		t.Run(fmt.Sprint(success), func(t *testing.T) {
			dir := reviewRepo(t, false)
			writeReviewFile(t, dir, "a.ts", "const bad = true\n")
			second := "invalid"
			if success {
				second = `{"findings":[]}`
			}
			env, calls := fakeAgentEnv(agentReply{out: "invalid"}, agentReply{out: second})
			r := runReviewTest(t, dir, env, ReviewRequest{Agent: "claude", NoCheck: true, NoPRComments: true})
			if len(*calls) != 2 || !strings.Contains((*calls)[1].stdin, "JSON") {
				t.Fatalf("retry %+v", calls)
			}
			if success && r.Exit != 0 || !success && (r.Exit != 2 || r.Chunks.Failed != 1) {
				t.Fatalf("%+v", r)
			}
		})
	}
}
func TestReviewChecksAndCeiling(t *testing.T) {
	dir := reviewRepo(t, true)
	writeReviewFile(t, dir, ".tija/rules/architecture/test.md", "---\nname: test\nchecks:\n  - id: forbid-bad\n    severity: High\n    files: '*.ts'\n    forbid: bad\n    message: forbidden\n---\n")
	writeReviewFile(t, dir, "a.ts", "bad\n"+strings.Repeat("ok\n", 1000))
	for _, mode := range []string{"review", "quality", "deep"} {
		r := runReviewTest(t, dir, fakeEnv(), ReviewRequest{Mode: mode, NoAgent: true, NoCheck: true, NoPRComments: true})
		want := 1
		if mode != "review" {
			want = 2
		}
		if len(r.Findings) != want || r.Exit != 1 {
			t.Fatalf("%s %+v", mode, r)
		}
	}
	writeReviewFile(t, dir, ".tija/rules/architecture/test.md", "---\nname: broken\nchecks:\n  - id: broken\n    severity: High\n    forbid: '['\n---\n")
	_, e := RunReview(context.Background(), dir, fakeEnv(), ReviewRequest{NoAgent: true, NoCheck: true}, nil)
	if !errors.Is(e, ErrInvalidRules) {
		t.Fatal(e)
	}
}
func TestReviewDeepTwoPassesParallel(t *testing.T) {
	dir := reviewRepo(t, true)
	m, e := manifest.Load(dir)
	if e != nil {
		t.Fatal(e)
	}
	m.Review.MaxLines = 1
	if e := manifest.Save(dir, m); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"one/a.ts", "two/b.ts", "three/c.ts"} {
		writeReviewFile(t, dir, p, "hello\n")
	}
	env, _ := fakeAgentEnv()
	var active, peak, calls atomic.Int32
	var mu sync.Mutex
	dirs := map[string]bool{}
	env.RunInput = func(ctx context.Context, dir, input, name string, args ...string) (string, string, int, error) {
		n := active.Add(1)
		for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
		}
		defer active.Add(-1)
		calls.Add(1)
		if !strings.Contains(strings.Join(args, " "), "--model opus") {
			t.Error("deep tier missing")
		}
		time.Sleep(15 * time.Millisecond)
		mu.Lock()
		dirs[input] = true
		mu.Unlock()
		return `{"findings":[]}`, "", 0, nil
	}
	r := runReviewTest(t, dir, env, ReviewRequest{Mode: "deep", Agent: "claude", NoCheck: true, NoPRComments: true})
	if r.Chunks.Total < 3 || calls.Load() != int32(r.Chunks.Total*2) || peak.Load() > 3 || peak.Load() < 2 || r.Exit != 0 {
		t.Fatalf("chunks %d calls %d peak %d exit %d", r.Chunks.Total, calls.Load(), peak.Load(), r.Exit)
	}
}
