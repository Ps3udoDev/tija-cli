package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestReportActivityDates(t *testing.T) {
	env := sys.Default()
	env.Run = func(context.Context, string, string, ...string) (string, string, int, error) {
		t.Fatal("invalid period reached the engine")
		return "", "", 0, nil
	}
	c := connectTools(t, New(t.TempDir(), Deps{Env: env}), nil)
	for _, period := range [][2]string{{"2026-13-01", "2026-10-31"}, {"2026-02-30", "2026-03-01"}, {"2026-10-31", "2026-10-01"}, {"2026-1-01", "2026-10-31"}, {"2026-10-01", "9999-01-01"}} {
		r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "report_activity", Arguments: map[string]any{"since": period[0], "until": period[1]}})
		if err != nil || !r.IsError || !strings.Contains(reportResultText(r), "invalid dates:") {
			t.Fatalf("period %v: result=%#v error=%v", period, r, err)
		}
	}
}

func TestReportActivityTimeouts(t *testing.T) {
	in := reportActivityInput{Since: "2026-10-01", Until: "2026-10-31", Agent: "none"}
	req, err := activityReportRequest(in)
	if err != nil || req.AgentTimeout != 300*time.Second || req.Agent != "none" {
		t.Fatalf("defaults: request=%+v error=%v", req, err)
	}
	custom := 42
	in.AgentTimeoutSec = &custom
	req, err = activityReportRequest(in)
	if err != nil || req.AgentTimeout != 42*time.Second {
		t.Fatalf("custom timeout: request=%+v error=%v", req, err)
	}
	for _, invalid := range []int{0, -1, int((time.Duration(1<<63-1))/time.Second) + 1} {
		in.AgentTimeoutSec = &invalid
		if _, err := activityReportRequest(in); err == nil || !strings.Contains(err.Error(), "agentTimeoutSec") {
			t.Fatalf("invalid timeout %d: %v", invalid, err)
		}
	}
	in.AgentTimeoutSec = nil
	in.Agent = "unknown"
	if _, err := activityReportRequest(in); err == nil || !strings.Contains(err.Error(), "unknown agent") {
		t.Fatalf("invalid agent: %v", err)
	}
}

func TestReportActivityAnnotations(t *testing.T) {
	c := connectTools(t, New(t.TempDir(), Deps{Env: sys.Default()}), nil)
	r, err := c.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range r.Tools {
		if tool.Name != "report_activity" {
			continue
		}
		a := tool.Annotations
		if a == nil || a.ReadOnlyHint || a.DestructiveHint == nil || *a.DestructiveHint || a.OpenWorldHint == nil || !*a.OpenWorldHint || tool.OutputSchema == nil {
			t.Fatalf("incorrect report annotations or schema: %#v", tool)
		}
		b, _ := json.Marshal(tool.InputSchema)
		var schema map[string]any
		if err := json.Unmarshal(b, &schema); err != nil {
			t.Fatal(err)
		}
		if schema["additionalProperties"] != false || !reflect.DeepEqual(schema["required"], []any{"since", "until"}) {
			t.Fatalf("input schema: %s", b)
		}
		return
	}
	t.Fatal("report_activity is not registered")
}

func reportTestRepo(t *testing.T) string {
	t.Helper()
	root := testutil.NewRepo(t, "")
	for _, rel := range []string{engine.ActivityGuide, engine.ActivitySchema, engine.ActivityCollectScript, engine.ActivityReportScript} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("local report fixture\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	testutil.Git(t, root, "add", ".")
	testutil.Git(t, root, "commit", "-m", "test: install report fixtures")
	return root
}

func reportTestEnv(t *testing.T, noPython, failCollect bool) sys.Env {
	t.Helper()
	env := sys.Default()
	realRun := env.Run
	realLook := env.LookPath
	env.LookPath = func(name string) (string, error) {
		if name == "git" {
			return realLook(name)
		}
		if !noPython && (name == "py" || name == "python" || name == "python3") {
			return name, nil
		}
		return "", errors.New("fixture: unavailable")
	}
	env.Run = func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
		if name != "py" && name != "python" && name != "python3" {
			return realRun(ctx, dir, name, args...)
		}
		if len(args) >= 2 && args[0] == "-c" {
			return "", "", 0, nil
		}
		if failCollect {
			return "", "fixture: collect failed", 1, nil
		}
		if len(args) < 4 || args[2] != engine.ActivityCollectScript {
			t.Fatalf("unexpected Python invocation: %s %v", name, args)
		}
		folder := filepath.Dir(filepath.Join(dir, filepath.FromSlash(args[3])))
		activity := `{"grupos":[{"nombre":"fixture","repos":[{"nombre":"fixture","commits":[{}],"prs":[],"sin_commitear":[]}]}]}`
		for name, contents := range map[string]string{"actividad.json": activity, "actividad.md": "# Local activity\n"} {
			if err := os.WriteFile(filepath.Join(folder, name), []byte(contents), 0644); err != nil {
				return "", "", 1, err
			}
		}
		return "", "", 0, nil
	}
	env.RunInput = func(context.Context, string, string, string, ...string) (string, string, int, error) {
		t.Fatal("agent none invoked an agent")
		return "", "", 1, nil
	}
	return env
}

func reportFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func reportResultText(r *mcp.CallToolResult) string {
	var parts []string
	for _, c := range r.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func TestReportActivityWritesOnlyReportes(t *testing.T) {
	root := reportTestRepo(t)
	before := reportFiles(t, root)
	c := connectTools(t, New(root, Deps{Env: reportTestEnv(t, false, false)}), nil)
	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "report_activity", Arguments: map[string]any{"since": "2026-10-01", "until": "2026-10-31", "agent": "none"}})
	if err != nil || r.IsError {
		t.Fatalf("report: %#v error=%v", r, err)
	}
	b, _ := json.Marshal(r.StructuredContent)
	var result engine.ReportResult
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatal(err)
	}
	if result.Commits != 1 || result.PDF != "" || result.Report != "" || result.Folder != "docs/reportes/actividades-2026-10-01_2026-10-31" || result.Activity != result.Folder+"/actividad.md" {
		t.Fatalf("report result: %s", b)
	}
	after := reportFiles(t, root)
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("modified existing fixture: %s", path)
		}
	}
	newFiles := 0
	for path := range after {
		if _, found := before[path]; !found {
			newFiles++
			if !strings.HasPrefix(path, "docs/reportes/") {
				t.Errorf("wrote outside reportes: %s", path)
			}
		}
	}
	if newFiles == 0 || testutil.Git(t, root, "status", "--porcelain", "--untracked-files=all") != "" {
		t.Fatalf("new files not ignored or absent: %d", newFiles)
	}
}

func TestReportActivityMissingPython(t *testing.T) {
	root := reportTestRepo(t)
	before := reportFiles(t, root)
	c := connectTools(t, New(root, Deps{Env: reportTestEnv(t, true, false)}), nil)
	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "report_activity", Arguments: map[string]any{"since": "2026-10-01", "until": "2026-10-31", "agent": "none"}})
	if err != nil || !r.IsError || !strings.Contains(reportResultText(r), "Python 3 was not found on PATH") {
		t.Fatalf("missing Python: %#v error=%v", r, err)
	}
	if !reflect.DeepEqual(before, reportFiles(t, root)) {
		t.Fatal("setup failure wrote files")
	}
}

func TestReportActivityMissingAgent(t *testing.T) {
	root := reportTestRepo(t)
	before := reportFiles(t, root)
	c := connectTools(t, New(root, Deps{Env: reportTestEnv(t, false, false)}), nil)
	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "report_activity", Arguments: map[string]any{"since": "2026-10-01", "until": "2026-10-31", "agent": "claude"}})
	if err != nil || !r.IsError || !strings.Contains(reportResultText(r), "claude") || !strings.Contains(reportResultText(r), "not installed") {
		t.Fatalf("missing agent: %#v error=%v", r, err)
	}
	if !reflect.DeepEqual(before, reportFiles(t, root)) {
		t.Fatal("missing agent setup failure wrote files")
	}
}

func TestReportActivityCollectFailureKeepsPaths(t *testing.T) {
	root := reportTestRepo(t)
	c := connectTools(t, New(root, Deps{Env: reportTestEnv(t, false, true)}), nil)
	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "report_activity", Arguments: map[string]any{"since": "2026-10-01", "until": "2026-10-31", "agent": "none"}})
	if err != nil || !r.IsError || !strings.Contains(reportResultText(r), "collect activity:") || strings.Contains(reportResultText(r), "no files were changed") {
		t.Fatalf("collect failure: %#v error=%v", r, err)
	}
	b, _ := json.Marshal(r.StructuredContent)
	var result engine.ReportResult
	if err := json.Unmarshal(b, &result); err != nil || result.Folder == "" {
		t.Fatalf("missing partial result: %s error=%v", b, err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(result.Folder), "manifest.json")); err != nil {
		t.Fatalf("engine did not write the reported artifact: %v", err)
	}
}
