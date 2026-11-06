package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestWorkflowRunPinsStepsToRoot runs workflow_run with the executor that
// mcp serve builds while the process sits outside the repository: the
// steps must still act on the server root, never on the working directory.
func TestWorkflowRunPinsStepsToRoot(t *testing.T) {
	repo := setupWorkflowRepo(t, "look", "name: look\nsteps:\n  - run: status\n")
	t.Chdir(t.TempDir())

	deps := cmdDeps(false)
	stepDeps := Deps{Env: deps.Env, Dir: repo, IsTTY: func(any) bool { return false }}
	s := mcpserver.New(repo, mcpserver.Deps{Env: deps.Env, RunStep: StepExecutor(stepDeps, strings.NewReader(""))})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	a, b := mcp.NewInMemoryTransports()
	go func() { _ = s.Serve(ctx, a) }()
	c, err := mcp.NewClient(&mcp.Implementation{Name: "pin", Version: "1"}, nil).Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	r, err := c.CallTool(ctx, &mcp.CallToolParams{Name: "workflow_run", Arguments: map[string]any{"name": "look"}})
	if err != nil || r.IsError {
		t.Fatalf("workflow_run: %#v %v", r, err)
	}
	var out struct {
		Status string `json:"status"`
		Steps  []struct {
			Exit   int             `json:"exit"`
			Output json.RawMessage `json:"output"`
		} `json:"steps"`
	}
	raw, _ := json.Marshal(r.StructuredContent)
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "completed" || len(out.Steps) != 1 || out.Steps[0].Exit != 0 {
		t.Fatalf("out = %s", raw)
	}
	var status struct {
		Root string `json:"root"`
	}
	if err := json.Unmarshal(out.Steps[0].Output, &status); err != nil {
		t.Fatalf("status output is not JSON: %s", out.Steps[0].Output)
	}
	want, _ := filepath.EvalSymlinks(repo)
	got, _ := filepath.EvalSymlinks(filepath.FromSlash(status.Root))
	if !strings.EqualFold(got, want) {
		t.Errorf("step ran in %q, want %q", status.Root, repo)
	}
}
