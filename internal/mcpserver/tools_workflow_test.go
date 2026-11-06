package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type countingExec struct {
	mu    sync.Mutex
	calls [][]string
	exit  int
}

func (c *countingExec) run(_ context.Context, args []string, out, _ io.Writer) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, args)
	_, _ = io.WriteString(out, `{"ok":true}`)
	return c.exit
}

func workflowRepo(t *testing.T, name, yaml string) string {
	t.Helper()
	root := testutil.NewRepo(t, "")
	dir := filepath.Join(root, ".tija", "workflows")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func callWorkflow(t *testing.T, root string, exec *countingExec, args map[string]any) (*mcp.CallToolResult, map[string]any) {
	t.Helper()
	s := New(root, Deps{Env: sys.Default(), RunStep: exec.run})
	c := connectTools(t, s, nil)
	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "workflow_run", Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if r.StructuredContent != nil {
		b, _ := json.Marshal(r.StructuredContent)
		_ = json.Unmarshal(b, &out)
	}
	return r, out
}

const prePR = "name: pre-pr\nsteps:\n  - run: clean comments --write\n  - run: check\n  - run: review\n  - run: pr prepare\n"

func TestWorkflowRunNeedsPermissionRunsNothing(t *testing.T) {
	exec := &countingExec{}
	r, out := callWorkflow(t, workflowRepo(t, "pre-pr", prePR), exec, map[string]any{"name": "pre-pr"})
	if r.IsError || out["status"] != "needs-permission" {
		t.Fatalf("result = %#v %v", r, out)
	}
	if needs, _ := out["needs"].([]any); len(needs) != 1 || needs[0] != "write" {
		t.Errorf("needs = %v", out["needs"])
	}
	planned, _ := out["plannedSteps"].([]any)
	if len(planned) != 4 || planned[0].(map[string]any)["effect"] != "repo" || planned[1].(map[string]any)["effect"] != "local" {
		t.Errorf("plannedSteps = %v", out["plannedSteps"])
	}
	if len(exec.calls) != 0 {
		t.Errorf("steps ran without permission: %v", exec.calls)
	}
}

func TestWorkflowRunPublishRequired(t *testing.T) {
	exec := &countingExec{}
	_, out := callWorkflow(t, workflowRepo(t, "ship", "name: ship\nsteps:\n  - run: check\n  - run: pr create\n"), exec,
		map[string]any{"name": "ship", "write": true})
	if needs, _ := out["needs"].([]any); out["status"] != "needs-permission" || len(needs) != 1 || needs[0] != "publish" {
		t.Errorf("out = %v", out)
	}
	if len(exec.calls) != 0 {
		t.Errorf("steps ran without publish: %v", exec.calls)
	}
}

func TestWorkflowRunUnsafeRequired(t *testing.T) {
	exec := &countingExec{}
	_, out := callWorkflow(t, workflowRepo(t, "deep", "name: deep\nsteps:\n  - run: clean comments --write --unsafe\n"), exec,
		map[string]any{"name": "deep", "write": true})
	if needs, _ := out["needs"].([]any); len(needs) != 1 || needs[0] != "unsafe" || len(exec.calls) != 0 {
		t.Errorf("out = %v, calls = %v", out, exec.calls)
	}
}

func TestWorkflowRunReadOnlyNeedsNothing(t *testing.T) {
	exec := &countingExec{}
	r, out := callWorkflow(t, workflowRepo(t, "look", "name: look\nsteps:\n  - run: status\n  - run: doctor\n"), exec, map[string]any{"name": "look"})
	if r.IsError || out["status"] != "completed" || len(exec.calls) != 2 {
		t.Fatalf("result = %v, calls = %v", out, exec.calls)
	}
	for _, c := range exec.calls {
		if !strings.Contains(strings.Join(c, " "), "--json") {
			t.Errorf("step %v ran without --json", c)
		}
	}
	if steps, _ := out["steps"].([]any); len(steps) != 2 {
		t.Errorf("steps = %v", out["steps"])
	}
}

func TestWorkflowRunAuthorizedRunsAll(t *testing.T) {
	exec := &countingExec{}
	_, out := callWorkflow(t, workflowRepo(t, "pre-pr", prePR), exec, map[string]any{"name": "pre-pr", "write": true})
	if out["status"] != "completed" || len(exec.calls) != 4 {
		t.Errorf("out = %v, calls = %v", out, exec.calls)
	}
}

func TestWorkflowRunRejectsEditorStep(t *testing.T) {
	exec := &countingExec{}
	r, _ := callWorkflow(t, workflowRepo(t, "edit", "name: edit\nsteps:\n  - run: pr create --edit\n"), exec,
		map[string]any{"name": "edit", "write": true, "publish": true})
	if !r.IsError || !strings.Contains(r.Content[0].(*mcp.TextContent).Text, "needs a terminal") || len(exec.calls) != 0 {
		t.Errorf("result = %#v, calls = %v", r, exec.calls)
	}
}

func TestWorkflowRunStepErrorKeepsReport(t *testing.T) {
	exec := &countingExec{exit: 2}
	r, out := callWorkflow(t, workflowRepo(t, "look", "name: look\nsteps:\n  - run: status\n  - run: doctor\n"), exec, map[string]any{"name": "look"})
	if !r.IsError || out["status"] != "completed" || len(exec.calls) != 1 {
		t.Errorf("result = %#v, out = %v, calls = %v", r, out, exec.calls)
	}
	if steps, _ := out["steps"].([]any); len(steps) != 1 {
		t.Errorf("steps = %v", out["steps"])
	}
}

func TestWorkflowRunUnknown(t *testing.T) {
	r, _ := callWorkflow(t, workflowRepo(t, "look", "name: look\nsteps:\n  - run: status\n"), &countingExec{}, map[string]any{"name": "nope"})
	if !r.IsError || !strings.Contains(r.Content[0].(*mcp.TextContent).Text, `unknown workflow "nope"`) {
		t.Errorf("result = %#v", r)
	}
}
