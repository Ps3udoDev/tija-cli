package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/testutil"
	"github.com/Ps3udoDev/tija-cli/internal/workflows"
)

func TestAllowedResolvesInCobra(t *testing.T) {
	root := NewRoot(cmdDeps(false), strings.NewReader(""))
	for _, entry := range workflows.Allowed {
		args := strings.Fields(entry)
		cmd, _, err := root.Find(args)
		if err != nil {
			t.Errorf("entry %q failed to resolve in Cobra: %v", entry, err)
			continue
		}
		if cmd == nil || cmd == root {
			t.Errorf("entry %q resolved to root command instead of a subcommand", entry)
		}
	}
}

func setupWorkflowRepo(t *testing.T, wfName, wfYAML string, opts ...testutil.RepoOpt) string {
	t.Helper()
	repo := testutil.NewRepo(t, "", opts...)
	wfDir := filepath.Join(repo, ".tija", "workflows")
	if err := os.MkdirAll(wfDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, wfName+".yaml"), []byte(wfYAML), 0644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestRunStopsAtFirstFailure(t *testing.T) {
	wf := `name: stop-on-fail
steps:
  - run: status --strict
  - run: check
`
	repo := setupWorkflowRepo(t, "stop-on-fail", wf, testutil.WithBranch("changes"))
	code, out, _ := runIn(t, repo, cmdDeps(false), "", "run", "stop-on-fail")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out, "> status --strict") {
		t.Errorf("expected out to contain '> status --strict', got: %s", out)
	}
	if strings.Contains(out, "> check") {
		t.Errorf("check should not have run after first failure, got: %s", out)
	}
}

func TestRunContinueOnError(t *testing.T) {
	wf := `name: cont-on-fail
steps:
  - run: status --strict
    continueOnError: true
  - run: status
`
	repo := setupWorkflowRepo(t, "cont-on-fail", wf, testutil.WithBranch("changes"))
	code, out, _ := runIn(t, repo, cmdDeps(false), "", "run", "cont-on-fail")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out, "> status --strict") {
		t.Errorf("expected out to contain '> status --strict', got: %s", out)
	}
	if !strings.Contains(out, "> status") {
		t.Errorf("expected out to contain '> status', got: %s", out)
	}
}

func TestRunJSONEmbedsStepOutput(t *testing.T) {
	wf := `name: json-test
steps:
  - run: status
  - run: status
`
	repo := setupWorkflowRepo(t, "json-test", wf)
	code, out, _ := runIn(t, repo, cmdDeps(false), "", "run", "json-test", "--json")
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	var rep WorkflowReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v, raw: %s", err, out)
	}
	if rep.Name != "json-test" {
		t.Errorf("rep.Name = %q, want json-test", rep.Name)
	}
	if rep.Exit != 0 {
		t.Errorf("rep.Exit = %d, want 0", rep.Exit)
	}
	if len(rep.Steps) != 2 {
		t.Fatalf("rep.Steps count = %d, want 2", len(rep.Steps))
	}
	if len(rep.Steps[0].Output) == 0 {
		t.Errorf("step 0 output should not be empty")
	}
	if len(rep.Steps[1].Output) == 0 {
		t.Errorf("step 1 output should not be empty")
	}
}

func TestRunList(t *testing.T) {
	wf := `name: list-wf
description: A workflow to test list
steps:
  - run: check
`
	repo := setupWorkflowRepo(t, "list-wf", wf)

	// Test plain text list
	code, out, _ := runIn(t, repo, cmdDeps(false), "", "run", "--list")
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "list-wf") || !strings.Contains(out, "A workflow to test list") {
		t.Errorf("out should contain name and description, got: %s", out)
	}

	// Test JSON list
	code, outJSON, _ := runIn(t, repo, cmdDeps(false), "", "run", "--list", "--json")
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	var list []workflows.Workflow
	if err := json.Unmarshal([]byte(outJSON), &list); err != nil {
		t.Fatalf("expected valid JSON list, got: %v", err)
	}
	if len(list) != 1 || list[0].Name != "list-wf" {
		t.Errorf("unexpected list: %+v", list)
	}
}
