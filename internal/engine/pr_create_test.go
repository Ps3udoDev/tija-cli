package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/agents"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

// ghFake answers gh invocations and records their argv.
type ghFake struct {
	calls   [][]string
	bodies  []string // --body-file contents at call time
	list    string   // stdout of gh pr list
	create  string   // stdout of gh pr create
	authErr bool
}

func (g *ghFake) run(ctx context.Context, dir string, args ...string) (string, string, int, error) {
	g.calls = append(g.calls, args)
	if i := slices.Index(args, "--body-file"); i >= 0 && i+1 < len(args) {
		data, _ := os.ReadFile(args[i+1])
		g.bodies = append(g.bodies, string(data))
	}
	switch {
	case len(args) >= 2 && args[0] == "auth" && args[1] == "status":
		if g.authErr {
			return "", "You are not logged into any GitHub hosts.", 1, nil
		}
		return "Logged in to github.com", "", 0, nil
	case len(args) >= 2 && args[0] == "pr" && args[1] == "list":
		if g.list == "" {
			return "[]\n", "", 0, nil
		}
		return g.list, "", 0, nil
	case len(args) >= 2 && args[0] == "pr" && args[1] == "create":
		return g.create, "", 0, nil
	case len(args) >= 2 && args[0] == "pr" && args[1] == "edit":
		return args[2] + "\n", "", 0, nil
	}
	return "", "unexpected gh call", 1, nil
}

func (g *ghFake) called(sub string) [][]string {
	var out [][]string
	for _, c := range g.calls {
		if len(c) >= 2 && c[0] == "pr" && c[1] == sub {
			out = append(out, c)
		}
	}
	return out
}

// withGH makes gh findable and routes it to g; everything else (git) runs for real.
func withGH(env sys.Env, g *ghFake) sys.Env {
	run, look := env.Run, env.LookPath
	env.LookPath = func(name string) (string, error) {
		if name == "gh" {
			return "/fake/bin/gh", nil
		}
		return look(name)
	}
	env.Run = func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
		if name == "gh" {
			return g.run(ctx, dir, args...)
		}
		return run(ctx, dir, name, args...)
	}
	return env
}

// remotePRRepo is prRepo with an origin remote: an initialized react-vite
// repo whose main (with init committed) is pushed, on a new feature branch
// with one unpushed commit. It returns the repo and the bare remote.
func remotePRRepo(t *testing.T) (string, string) {
	t.Helper()
	bare := t.TempDir()
	repo := testutil.NewRepo(t, fixture("react-vite"), testutil.WithRemote(bare, true))
	p := plan(t, repo)
	apply(t, p, nil, approveAll(p))
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-m", "chore: init tija")
	testutil.Git(t, repo, "push", "origin", "main")
	testutil.Git(t, repo, "checkout", "-b", "feat/auth-mask")
	commitFile(t, repo, "src/auth/password.ts", "export const mask = (s: string) => '*'.repeat(s.length)\n", "feat(auth): mask passwords in logs")
	return repo, bare
}

func planCreate(t *testing.T, repo string, env sys.Env, req CreateRequest) CreatePlan {
	t.Helper()
	p, err := PlanCreatePR(context.Background(), repo, env, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

var noneReq = CreateRequest{Prepare: PRRequest{Agent: "none", NoCheck: true}}

func TestCreatePRPushesUnpublished(t *testing.T) {
	repo, bare := remotePRRepo(t)
	g := &ghFake{create: "Creating pull request for feat/auth-mask into main\n\nhttps://github.com/o/r/pull/7\n"}
	env := withGH(fakeEnv(), g)

	p := planCreate(t, repo, env, noneReq)
	if !p.Push || p.Existing != "" || !p.Assign || p.Report.ExitCode() != 0 {
		t.Fatalf("plan = %+v", p)
	}
	if len(g.called("create")) != 0 {
		t.Fatal("plan must not create the pull request")
	}
	rep, err := ApplyCreatePR(context.Background(), repo, env, p, noneReq)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Pushed || rep.Updated || !rep.Assigned || rep.URL != "https://github.com/o/r/pull/7" {
		t.Errorf("report = %+v", rep)
	}
	if got, want := testutil.Git(t, bare, "rev-parse", "refs/heads/feat/auth-mask"), testutil.Git(t, repo, "rev-parse", "HEAD"); got != want {
		t.Errorf("bare branch = %q, want %q", got, want)
	}
	if up := testutil.Git(t, repo, "rev-parse", "--abbrev-ref", "feat/auth-mask@{upstream}"); up != "origin/feat/auth-mask" {
		t.Errorf("upstream = %q", up)
	}
	creates := g.called("create")
	if len(creates) != 1 {
		t.Fatalf("create calls = %v", g.calls)
	}
	argv := strings.Join(creates[0], " ")
	for _, want := range []string{"--base main --head feat/auth-mask", "--assignee @me", "--title " + p.Report.Title, "--body-file "} {
		if !strings.Contains(argv, want) {
			t.Errorf("argv %q lacks %q", argv, want)
		}
	}
	if strings.Contains(argv, "--draft") {
		t.Errorf("argv %q has --draft", argv)
	}
	if len(g.bodies) != 1 || g.bodies[0] != p.Report.Body {
		t.Errorf("body file = %q, want %q", g.bodies, p.Report.Body)
	}
	bodyFile := creates[0][slices.Index(creates[0], "--body-file")+1]
	if filepath.Clean(bodyFile) != filepath.Join(repo, ".tija", "tmp", "pr", "body.md") {
		t.Errorf("body file path = %q", bodyFile)
	}
}

func TestCreatePRDraftAndNoAssign(t *testing.T) {
	repo, _ := remotePRRepo(t)
	m := filepath.Join(repo, ".tija", "manifest.yaml")
	data, err := os.ReadFile(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "autoAssign: true") {
		t.Fatalf("manifest lacks autoAssign:\n%s", data)
	}
	if err := os.WriteFile(m, []byte(strings.Replace(string(data), "autoAssign: true", "autoAssign: false", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &ghFake{create: "https://github.com/o/r/pull/8\n"}
	env := withGH(fakeEnv(), g)
	req := noneReq
	req.Draft = true

	p := planCreate(t, repo, env, req)
	if p.Assign {
		t.Errorf("Assign = true with autoAssign: false")
	}
	rep, err := ApplyCreatePR(context.Background(), repo, env, p, req)
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Join(g.called("create")[0], " ")
	if !strings.Contains(argv, "--draft") || strings.Contains(argv, "--assignee") || rep.Assigned {
		t.Errorf("argv = %q report = %+v", argv, rep)
	}
}

func TestCreatePRUpdatesExisting(t *testing.T) {
	repo, _ := remotePRRepo(t)
	testutil.Git(t, repo, "push", "-u", "origin", "feat/auth-mask")
	g := &ghFake{list: `[{"url":"https://github.com/o/r/pull/3","baseRefName":"main","isCrossRepository":false}]`}
	env := withGH(fakeEnv(), g)

	p := planCreate(t, repo, env, noneReq)
	if p.Existing != "https://github.com/o/r/pull/3" || p.Push {
		t.Fatalf("plan = %+v", p)
	}
	lists := g.called("list")
	if len(lists) != 1 || strings.Join(lists[0], " ") != "pr list --head feat/auth-mask --state open --json url,baseRefName,isCrossRepository --limit 5" {
		t.Errorf("list calls = %v", lists)
	}
	rep, err := ApplyCreatePR(context.Background(), repo, env, p, noneReq)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Updated || rep.Pushed || rep.URL != "https://github.com/o/r/pull/3" || !rep.Assigned {
		t.Errorf("report = %+v", rep)
	}
	if c := g.called("create"); len(c) != 0 {
		t.Errorf("gh pr create called: %v", c)
	}
	edits := g.called("edit")
	if len(edits) != 1 {
		t.Fatalf("edit calls = %v", g.calls)
	}
	argv := strings.Join(edits[0], " ")
	if !strings.HasPrefix(argv, "pr edit https://github.com/o/r/pull/3 --title ") || !strings.Contains(argv, "--body-file ") || !strings.Contains(argv, "--add-assignee @me") {
		t.Errorf("edit argv = %q", argv)
	}
}

func TestCreatePRPushesUnpushedCommits(t *testing.T) {
	repo, bare := remotePRRepo(t)
	testutil.Git(t, repo, "push", "-u", "origin", "feat/auth-mask")
	commitFile(t, repo, "src/auth/more.ts", "export const x = 1\n", "feat(auth): add more")
	g := &ghFake{create: "https://github.com/o/r/pull/9\n"}
	env := withGH(fakeEnv(), g)

	p := planCreate(t, repo, env, noneReq)
	if !p.Push {
		t.Fatal("Push = false with an unpushed commit")
	}
	if _, err := ApplyCreatePR(context.Background(), repo, env, p, noneReq); err != nil {
		t.Fatal(err)
	}
	if got, want := testutil.Git(t, bare, "rev-parse", "refs/heads/feat/auth-mask"), testutil.Git(t, repo, "rev-parse", "HEAD"); got != want {
		t.Errorf("bare branch = %q, want %q", got, want)
	}
}

func TestCreatePRReusesFreshDraft(t *testing.T) {
	repo, _ := remotePRRepo(t)
	if _, err := PreparePR(context.Background(), repo, fakeEnv(), PRRequest{Agent: "none", NoCheck: true}, nil); err != nil {
		t.Fatal(err)
	}
	d, meta, err := LoadDraft(repo)
	if err != nil {
		t.Fatal(err)
	}
	edited := "feat(auth): ocultar contraseñas en los registros"
	// Hand edit, as a user would, with Windows line endings.
	md := strings.ReplaceAll(edited+"\n\n"+d.Body+"\nNota añadida a mano.\n", "\n", "\r\n")
	if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(DraftPath)), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	aenv, calls := fakeAgentEnv()
	g := &ghFake{}
	req := CreateRequest{Prepare: PRRequest{Agent: "claude", NoCheck: true}}
	p := planCreate(t, repo, withGH(aenv, g), req)
	if len(*calls) != 0 {
		t.Fatalf("agent called %d times", len(*calls))
	}
	if p.Report.Title != edited || !strings.Contains(p.Report.Body, "Nota añadida a mano.") || strings.Contains(p.Report.Body, "\r") {
		t.Errorf("title=%q body=%q", p.Report.Title, p.Report.Body)
	}
	if p.Report.Source != meta.Source || p.Report.Branch != "feat/auth-mask" || p.Report.Target != "main" ||
		p.Report.DraftPath != DraftPath || p.Report.Commits != 1 || len(p.Report.Problems) != 0 {
		t.Errorf("report = %+v", p.Report)
	}
}

func TestCreatePRStaleDraftRegenerates(t *testing.T) {
	repo, _ := remotePRRepo(t)
	stale := agents.Draft{Title: "feat(auth): borrador viejo", Body: "## Resumen\n\nviejo"}
	if err := SaveDraft(repo, stale, DraftMeta{Head: "0000000000000000000000000000000000000000", Branch: "feat/auth-mask", Target: "main", Source: "claude"}); err != nil {
		t.Fatal(err)
	}
	aenv, calls := fakeAgentEnv(agentReply{out: validAgentDraft})
	req := CreateRequest{Prepare: PRRequest{Agent: "claude", NoCheck: true}}
	p := planCreate(t, repo, withGH(aenv, &ghFake{}), req)
	if len(*calls) != 1 {
		t.Fatalf("agent calls = %d", len(*calls))
	}
	if p.Report.Title != "feat(auth): enmascarar contraseñas en los logs" || p.Report.Source != "claude" || p.Report.Attempts != 1 {
		t.Errorf("report = %+v", p.Report)
	}
	_, meta, err := LoadDraft(repo)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Head != testutil.Git(t, repo, "rev-parse", "HEAD") {
		t.Errorf("saved meta head = %q", meta.Head)
	}
}

func TestCreatePRGhMissing(t *testing.T) {
	repo, _ := remotePRRepo(t)
	_, err := PlanCreatePR(context.Background(), repo, fakeEnv(), noneReq, nil)
	if !errors.Is(err, ErrGHUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "install GitHub CLI and run: gh auth login") {
		t.Errorf("message = %q", err)
	}
	if _, statErr := os.Stat(filepath.Join(repo, filepath.FromSlash(DraftPath))); statErr == nil {
		t.Error("a draft was prepared although gh is unavailable")
	}
}

func TestCreatePRGhNotAuthenticated(t *testing.T) {
	repo, _ := remotePRRepo(t)
	g := &ghFake{authErr: true}
	_, err := PlanCreatePR(context.Background(), repo, withGH(fakeEnv(), g), noneReq, nil)
	if !errors.Is(err, ErrGHUnavailable) || !strings.Contains(err.Error(), "not logged into") {
		t.Fatalf("err = %v", err)
	}
}

func TestCreatePRInvalidDraftDoesNotPush(t *testing.T) {
	repo, bare := remotePRRepo(t)
	if _, err := PreparePR(context.Background(), repo, fakeEnv(), PRRequest{Agent: "none", NoCheck: true}, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(DraftPath)), []byte("Arreglos varios\n\n## Resumen\n\nTBD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &ghFake{}
	env := withGH(fakeEnv(), g)
	p := planCreate(t, repo, env, noneReq)
	if p.Report.ExitCode() != 1 || len(p.Report.Problems) == 0 {
		t.Fatalf("ExitCode = %d problems = %+v", p.Report.ExitCode(), p.Report.Problems)
	}
	if _, err := ApplyCreatePR(context.Background(), repo, env, p, noneReq); err == nil {
		t.Error("ApplyCreatePR accepted a draft with problems")
	}
	if c := append(g.called("create"), g.called("edit")...); len(c) != 0 {
		t.Errorf("gh calls = %v", c)
	}
	if out := testutil.Git(t, bare, "branch", "--list", "feat/auth-mask"); out != "" {
		t.Errorf("branch was pushed: %q", out)
	}
}

func TestCreatePRBaseBranchErrors(t *testing.T) {
	repo, _ := remotePRRepo(t)
	testutil.Git(t, repo, "checkout", "main")
	_, err := PlanCreatePR(context.Background(), repo, withGH(fakeEnv(), &ghFake{}), noneReq, nil)
	if !errors.Is(err, ErrBaseBranch) {
		t.Fatalf("err = %v", err)
	}
}

func TestCreatePRGhFailureCarriesStderr(t *testing.T) {
	repo, _ := remotePRRepo(t)
	g := &ghFake{}
	env := withGH(fakeEnv(), g)
	p := planCreate(t, repo, env, noneReq)
	run := env.Run
	env.Run = func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
		if name == "gh" && len(args) >= 2 && args[1] == "create" {
			return "", "GraphQL: No commits between main and feat/auth-mask", 1, nil
		}
		return run(ctx, dir, name, args...)
	}
	_, err := ApplyCreatePR(context.Background(), repo, env, p, noneReq)
	if err == nil || !strings.Contains(err.Error(), "No commits between") {
		t.Fatalf("err = %v", err)
	}
}

func TestCreatePRUninitializedUsesTempBody(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"), testutil.WithRemote("", true), testutil.WithBranch("feat/auth-mask"))
	commitFile(t, repo, "src/auth/password.ts", "export const mask = 1\n", "feat(auth): mask passwords in logs")
	g := &ghFake{create: "https://github.com/o/r/pull/10\n"}
	env := withGH(fakeEnv(), g)
	p := planCreate(t, repo, env, noneReq)
	if _, err := ApplyCreatePR(context.Background(), repo, env, p, noneReq); err != nil {
		t.Fatal(err)
	}
	c := g.called("create")[0]
	bodyFile := c[slices.Index(c, "--body-file")+1]
	if strings.HasPrefix(filepath.Clean(bodyFile), repo) {
		t.Errorf("body file %q is inside the repo", bodyFile)
	}
	if _, err := os.Stat(filepath.Join(repo, ".tija")); err == nil {
		t.Error(".tija was created in an uninitialized repo")
	}
	if _, err := os.Stat(bodyFile); err == nil {
		t.Error("temporary body file was not removed")
	}
}

func TestCreatePlanQuestion(t *testing.T) {
	rep := PRReport{Branch: "feat/x", Target: "main"}
	url := "https://github.com/o/r/pull/3"
	cases := []struct {
		plan CreatePlan
		want string
	}{
		{CreatePlan{Report: rep, Push: true, Assign: true}, "Push feat/x and open a pull request to main and assign it to you?"},
		{CreatePlan{Report: rep, Push: true}, "Push feat/x and open a pull request to main?"},
		{CreatePlan{Report: rep}, "Open a pull request to main?"},
		{CreatePlan{Report: rep, Assign: true}, "Open a pull request to main and assign it to you?"},
		{CreatePlan{Report: rep, Existing: url}, "Update pull request " + url + "?"},
		{CreatePlan{Report: rep, Existing: url, Push: true, Assign: true}, "Push feat/x and update pull request " + url + " and assign it to you?"},
	}
	for _, c := range cases {
		if got := c.plan.Question(); got != c.want {
			t.Errorf("Question(%+v) = %q, want %q", c.plan, got, c.want)
		}
	}
}

func TestCreatePRExistingBaseMismatch(t *testing.T) {
	repo, _ := remotePRRepo(t)
	testutil.Git(t, repo, "push", "-u", "origin", "feat/auth-mask")
	g := &ghFake{list: `[{"url":"https://github.com/o/r/pull/3","baseRefName":"develop","isCrossRepository":false}]`}
	env := withGH(fakeEnv(), g)

	p := planCreate(t, repo, env, noneReq)
	if p.Existing != "https://github.com/o/r/pull/3" || p.ExistingBase != "develop" {
		t.Fatalf("plan = %+v", p)
	}
	if !hasWarning(p.Report, "existing pull request targets develop, not main — its diff differs from this description") {
		t.Errorf("warnings = %v", p.Report.Warnings)
	}
	if q := p.Question(); q != "Update pull request https://github.com/o/r/pull/3 (base: develop) and assign it to you?" {
		t.Errorf("question = %q", q)
	}
	if _, err := ApplyCreatePR(context.Background(), repo, env, p, noneReq); err != nil {
		t.Fatal(err)
	}
	argv := strings.Join(g.called("edit")[0], " ")
	if strings.Contains(argv, "--base") {
		t.Errorf("edit changed the base: %q", argv)
	}
}

func TestCreatePRIgnoresForkPullRequests(t *testing.T) {
	repo, _ := remotePRRepo(t)
	testutil.Git(t, repo, "push", "-u", "origin", "feat/auth-mask")
	g := &ghFake{
		list: `[{"url":"https://github.com/o/r/pull/2","baseRefName":"main","isCrossRepository":true},` +
			`{"url":"https://github.com/o/r/pull/4","baseRefName":"main","isCrossRepository":false}]`,
		create: "https://github.com/o/r/pull/9\n",
	}
	p := planCreate(t, repo, withGH(fakeEnv(), g), noneReq)
	if p.Existing != "https://github.com/o/r/pull/4" || p.ExistingBase != "main" || hasWarning(p.Report, "existing pull request targets") {
		t.Fatalf("plan = %+v", p)
	}

	g = &ghFake{list: `[{"url":"https://github.com/o/r/pull/2","baseRefName":"main","isCrossRepository":true}]`}
	p = planCreate(t, repo, withGH(fakeEnv(), g), noneReq)
	if p.Existing != "" {
		t.Errorf("a fork pull request was taken as existing: %+v", p)
	}
}

func TestCreatePRGhFailureAfterPushSaysPushed(t *testing.T) {
	repo, _ := remotePRRepo(t)
	env := withGH(fakeEnv(), &ghFake{})
	p := planCreate(t, repo, env, noneReq)
	if !p.Push {
		t.Fatal("Push = false")
	}
	run := env.Run
	env.Run = func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
		if name == "gh" && len(args) >= 2 && args[1] == "create" {
			return "", "HTTP 502", 1, nil
		}
		return run(ctx, dir, name, args...)
	}
	_, err := ApplyCreatePR(context.Background(), repo, env, p, noneReq)
	if err == nil || !strings.HasPrefix(err.Error(), "pushed feat/auth-mask; gh pr create failed: HTTP 502") {
		t.Fatalf("err = %v", err)
	}
}
