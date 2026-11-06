package mcpserver_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/agents"
	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeGH answers gh and records every call; git runs for real.
type fakeGH struct {
	mu     sync.Mutex
	calls  [][]string
	bodies []string
}

func (g *fakeGH) env() sys.Env {
	env := sys.Default()
	run := env.Run
	env.LookPath = func(name string) (string, error) {
		switch name {
		case "git":
			return exec.LookPath("git")
		case "gh":
			return "/fake/bin/gh", nil
		}
		return "", errors.New("test: not found: " + name)
	}
	env.Run = func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
		if name != "gh" {
			return run(ctx, dir, name, args...)
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		g.calls = append(g.calls, args)
		if i := slices.Index(args, "--body-file"); i >= 0 && i+1 < len(args) {
			data, _ := os.ReadFile(args[i+1])
			g.bodies = append(g.bodies, string(data))
		}
		switch {
		case len(args) >= 2 && args[0] == "auth":
			return "Logged in to github.com", "", 0, nil
		case len(args) >= 2 && args[0] == "pr" && args[1] == "list":
			return "[]\n", "", 0, nil
		case len(args) >= 2 && args[0] == "pr" && args[1] == "create":
			return "https://github.com/o/r/pull/42\n", "", 0, nil
		}
		return "", "unexpected gh call", 1, nil
	}
	return env
}

func (g *fakeGH) published() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, c := range g.calls {
		if len(c) >= 2 && c[0] == "pr" && (c[1] == "create" || c[1] == "edit") {
			return true
		}
	}
	return false
}

// prRemoteRepo is an initialized react-vite repo whose main is pushed to a
// bare remote, on a feature branch with one unpushed commit.
func prRemoteRepo(t *testing.T) (string, string) {
	t.Helper()
	bare := t.TempDir()
	repo := testutil.NewRepo(t, filepath.Join("..", "detect", "testdata", "react-vite"), testutil.WithRemote(bare, true))
	ctx := context.Background()
	plan, err := engine.PlanInit(ctx, repo, sys.Default())
	if err != nil {
		t.Fatal(err)
	}
	approved := map[string]bool{}
	for _, a := range plan.Actions {
		approved[a.ID] = true
	}
	if _, err := engine.ApplyInit(ctx, plan, nil, approved); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-m", "chore: init tija")
	testutil.Git(t, repo, "push", "origin", "main")
	testutil.Git(t, repo, "checkout", "-b", "feat/auth-mask")
	localWrite(t, repo, "src/auth/password.ts", "export const mask = (s: string) => '*'.repeat(s.length)\n")
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-m", "feat(auth): mask passwords in logs")
	return repo, bare
}

func remoteHasBranch(t *testing.T, bare, branch string) bool {
	t.Helper()
	return strings.TrimSpace(testutil.Git(t, bare, "branch", "--list", branch)) != ""
}

var qaptureRef = map[string]any{
	"source":     "qapture",
	"project":    map[string]any{"id": "p-1", "name": "Portal"},
	"cases":      []any{map[string]any{"code": "EJ-014", "title": "Inicio de sesión", "status": "Aprobado"}},
	"exportPath": `C:\Users\someone\matriz-login.xlsx`,
}

func TestPRCreateWithoutWriteDoesNotPublish(t *testing.T) {
	repo, bare := prRemoteRepo(t)
	g := &fakeGH{}
	c := paritySession(t, repo, g.env())
	callLocal(t, c, "pr_prepare", map[string]any{"agent": "none", "noCheck": true})

	out := structured(t, callLocal(t, c, "pr_create", map[string]any{}))
	if out["status"] != "needs-write" || !strings.Contains(out["question"].(string), "feat/auth-mask") || out["push"] != true {
		t.Errorf("plan = %v", out)
	}
	if g.published() || remoteHasBranch(t, bare, "feat/auth-mask") {
		t.Errorf("pr_create without write published or pushed: %v", g.calls)
	}
}

func TestPRCreateRefusesDraftWithProblems(t *testing.T) {
	repo, bare := prRemoteRepo(t)
	g := &fakeGH{}
	c := paritySession(t, repo, g.env())
	callLocal(t, c, "pr_prepare", map[string]any{"agent": "none", "noCheck": true})
	// The saved draft is edited into one that breaks the rules.
	d, meta, err := engine.LoadDraft(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SaveDraft(repo, agents.Draft{Title: "no conventional title", Body: d.Body}, meta); err != nil {
		t.Fatal(err)
	}

	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "pr_create", Arguments: map[string]any{"write": true}})
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsError || r.Content[0].(*mcp.TextContent).Text != "pull request draft has problems; nothing was published" {
		t.Fatalf("result = %#v", r)
	}
	out := structured(t, r)
	report, _ := out["report"].(map[string]any)
	if out["status"] != "draft-has-problems" || report == nil || len(report["problems"].([]any)) == 0 {
		t.Errorf("structured = %v", out)
	}
	if g.published() || remoteHasBranch(t, bare, "feat/auth-mask") {
		t.Errorf("a draft with problems was published or pushed: %v", g.calls)
	}
}

func TestPRCreatePublishesWithEvidence(t *testing.T) {
	repo, bare := prRemoteRepo(t)
	g := &fakeGH{}
	c := paritySession(t, repo, g.env())
	callLocal(t, c, "pr_prepare", map[string]any{"agent": "none", "noCheck": true, "evidenceRefs": []any{qaptureRef}})

	out := structured(t, callLocal(t, c, "pr_create", map[string]any{"write": true}))
	if out["status"] != "published" || out["number"] != float64(42) || out["pushed"] != true {
		t.Fatalf("report = %v", out)
	}
	evidence, _ := out["evidence"].([]any)
	if len(evidence) != 1 || evidence[0].(map[string]any)["exportName"] != "matriz-login.xlsx" {
		t.Errorf("evidence = %v", out["evidence"])
	}
	if !remoteHasBranch(t, bare, "feat/auth-mask") {
		t.Errorf("branch was not pushed")
	}
	if len(g.bodies) != 1 || !strings.Contains(g.bodies[0], "EJ-014") || strings.Contains(g.bodies[0], `C:\Users`) {
		t.Errorf("published body = %q", g.bodies)
	}
}

func TestPRCreateAnnotations(t *testing.T) {
	c := paritySession(t, commentsRepo(t), sys.Default())
	r, err := c.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range r.Tools {
		if tool.Name != "pr_create" {
			continue
		}
		a := tool.Annotations
		if a == nil || a.ReadOnlyHint || a.DestructiveHint == nil || !*a.DestructiveHint || a.OpenWorldHint == nil || !*a.OpenWorldHint {
			t.Errorf("pr_create annotations = %#v", a)
		}
		return
	}
	t.Fatal("pr_create is not listed")
}
