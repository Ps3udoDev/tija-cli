package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

// prBranchRepo is a react-vite repo with an origin remote, on a feature
// branch with one commit ahead of main.
func prBranchRepo(t *testing.T) string {
	t.Helper()
	repo := testutil.NewRepo(t, fixture("react-vite"), testutil.WithRemote("", true))
	testutil.Git(t, repo, "checkout", "-b", "feat/auth-mask")
	if err := os.MkdirAll(filepath.Join(repo, "src", "auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "src", "auth", "password.ts"), []byte("export const mask = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-m", "feat(auth): mask passwords in logs")
	return repo
}

// ghDeps routes gh to a fake that records its argv; git runs for real.
func ghDeps(isTTY bool, calls *[][]string) Deps {
	d := cmdDeps(isTTY)
	run, look := d.Env.Run, d.Env.LookPath
	d.Env = sys.Env{
		Getenv: d.Env.Getenv,
		LookPath: func(name string) (string, error) {
			if name == "gh" {
				return "/fake/gh", nil
			}
			return look(name)
		},
		Run: func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
			if name != "gh" {
				return run(ctx, dir, name, args...)
			}
			*calls = append(*calls, args)
			switch {
			case args[0] == "pr" && args[1] == "list":
				return "[]\n", "", 0, nil
			case args[0] == "pr" && args[1] == "create":
				return "https://github.com/o/r/pull/7\n", "", 0, nil
			}
			return "ok", "", 0, nil
		},
	}
	return d
}

func hasGHCall(calls [][]string, sub string) bool {
	for _, c := range calls {
		if len(c) >= 2 && c[0] == "pr" && c[1] == sub {
			return true
		}
	}
	return false
}

func TestPRPrepareNoAgentJSON(t *testing.T) {
	repo := prBranchRepo(t)
	code, out, errOut := runIn(t, repo, cmdDeps(false), "", "pr", "prepare", "--no-agent", "--no-check", "--json")
	if code != 0 {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, out, errOut)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if v["source"] != "deterministic" {
		t.Errorf("source = %v", v["source"])
	}
}

func TestPRPreparePrintPrompt(t *testing.T) {
	repo := prBranchRepo(t)
	code, out, errOut := runIn(t, repo, cmdDeps(false), "", "pr", "prepare", "--print-prompt")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(out, "<<<TITLE") {
		t.Errorf("prompt lacks <<<TITLE:\n%s", out)
	}
}

func TestPRPrepareOnBaseBranch(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"), testutil.WithRemote("", true))
	code, _, _ := runIn(t, repo, cmdDeps(false), "", "pr", "prepare", "--no-agent", "--no-check")
	if code != 2 {
		t.Errorf("code = %d, want 2", code)
	}
}

func TestPRCreateNeedsYesWithoutTTY(t *testing.T) {
	repo := prBranchRepo(t)
	var calls [][]string
	code, out, errOut := runIn(t, repo, ghDeps(false, &calls), "", "pr", "create", "--no-agent", "--no-check")
	if code != 2 {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(out, "Pull request  feat/auth-mask") {
		t.Errorf("stdout lacks the preview:\n%s", out)
	}
	if !strings.Contains(errOut, "confirmation required: rerun with --yes") {
		t.Errorf("stderr = %q", errOut)
	}
	if hasGHCall(calls, "create") {
		t.Error("gh pr create ran without confirmation")
	}
}

func TestPRCreateYes(t *testing.T) {
	repo := prBranchRepo(t)
	var calls [][]string
	code, out, errOut := runIn(t, repo, ghDeps(false, &calls), "", "pr", "create", "--no-agent", "--no-check", "--yes")
	if code != 0 {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, out, errOut)
	}
	if !strings.Contains(out, "https://github.com/o/r/pull/7") {
		t.Errorf("stdout lacks the URL:\n%s", out)
	}
	if !hasGHCall(calls, "create") {
		t.Error("gh pr create did not run")
	}
}

func TestPRCreateEditNeedsTTY(t *testing.T) {
	repo := prBranchRepo(t)
	var calls [][]string
	code, _, _ := runIn(t, repo, ghDeps(false, &calls), "", "pr", "create", "--no-agent", "--no-check", "--edit", "--yes")
	if code != 2 {
		t.Errorf("code = %d, want 2", code)
	}
	if hasGHCall(calls, "create") {
		t.Error("gh pr create ran")
	}
}

func TestPRPreparePrintPromptJSON(t *testing.T) {
	repo := prBranchRepo(t)
	code, out, errOut := runIn(t, repo, cmdDeps(false), "", "pr", "prepare", "--print-prompt", "--json")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
	var v struct {
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if !strings.Contains(v.Prompt, "<<<TITLE") {
		t.Errorf("prompt = %q", v.Prompt)
	}
}
