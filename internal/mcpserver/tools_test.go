package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var updateTools = flag.Bool("update", false, "update the MCP tools contract snapshot")

func connectTools(t *testing.T, s *Server, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	a, b := mcp.NewInMemoryTransports()
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, a) }()
	c, err := mcp.NewClient(&mcp.Implementation{Name: "tool-tests", Version: "1"}, opts).Connect(ctx, b, nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = c.Close()
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("server did not stop")
		}
	})
	return c
}

func TestReadToolsListedWithAnnotations(t *testing.T) {
	s := New(t.TempDir(), Deps{Env: sys.Default()})
	c := connectTools(t, s, nil)
	r, err := c.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range r.Tools {
		names = append(names, tool.Name)
		readTool := slices.Contains([]string{"doctor", "repo_context", "repo_rules", "repo_status", "skills_list"}, tool.Name)
		if readTool && (tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint) {
			t.Errorf("annotations %s = %#v", tool.Name, tool.Annotations)
		}
		b, _ := json.Marshal(tool.InputSchema)
		var schema map[string]any
		if err := json.Unmarshal(b, &schema); err != nil {
			t.Fatal(err)
		}
		if schema["additionalProperties"] != false {
			t.Errorf("input %s is not closed: %s", tool.Name, b)
		}
		if props, ok := schema["properties"].(map[string]any); ok {
			if _, ok := props["dir"]; ok {
				t.Errorf("%s accepts dir", tool.Name)
			}
		}
		if tool.OutputSchema == nil {
			t.Errorf("%s has no output schema", tool.Name)
		}
	}
	slices.Sort(names)
	for _, want := range []string{"doctor", "repo_context", "repo_rules", "repo_status", "skills_list"} {
		if !slices.Contains(names, want) {
			t.Errorf("missing %s: %v", want, names)
		}
	}
}

func TestToolsGolden(t *testing.T) {
	s := New(t.TempDir(), Deps{Env: sys.Default()})
	c := connectTools(t, s, nil)
	r, err := c.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	type entry struct {
		Name        string               `json:"name"`
		Input       any                  `json:"inputSchema"`
		Output      any                  `json:"outputSchema,omitempty"`
		Annotations *mcp.ToolAnnotations `json:"annotations"`
	}
	entries := []entry{}
	for _, tool := range r.Tools {
		e := entry{Name: tool.Name, Input: tool.InputSchema, Annotations: tool.Annotations}
		if slices.Contains([]string{"repo_status", "review_run", "pr_prepare", "pr_create"}, tool.Name) {
			e.Output = tool.OutputSchema
		}
		entries = append(entries, e)
	}
	slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.Name, b.Name) })
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, '\n')
	p := filepath.Join("testdata", "tools.golden.json")
	if *updateTools {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	if !bytes.Equal(b, want) {
		t.Fatalf("tools contract changed; inspect diff and regenerate with go test ./internal/mcpserver -run ToolsGolden -update\ngot:\n%s", b)
	}
}

func TestToolErrorIsResultNotRPC(t *testing.T) {
	root := testutil.NewRepo(t, "")
	env := sys.Default()
	run := env.Run
	env.Run = func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
		if name == "git" && slices.Contains(args, "--show-toplevel") {
			return filepath.Join(root, "unavailable"), "", 0, nil
		}
		return run(ctx, dir, name, args...)
	}
	s := New(root, Deps{Env: env})
	c := connectTools(t, s, nil)
	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "doctor", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("environment failure escaped as RPC error: %v", err)
	}
	if !r.IsError {
		t.Fatalf("doctor succeeded: %#v", r)
	}
	b, _ := json.Marshal(r.Content)
	if !strings.Contains(string(b), "is not a directory") {
		t.Fatalf("error lost environment context: %s", b)
	}
}

func TestWriteMutexRespectsCancel(t *testing.T) {
	s := New(t.TempDir(), Deps{Env: sys.Default()})
	started, release, firstDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	h := toolHandler(s, "first", EffectLocal, func(context.Context, struct{}, Progress) (struct{}, int, error) {
		close(started)
		<-release
		return struct{}{}, 0, nil
	})
	go func() { _, _, err := h(context.Background(), nil, struct{}{}); firstDone <- err }()
	<-started
	defer func() {
		close(release)
		if err := <-firstDone; err != nil {
			t.Error(err)
		}
	}()
	called := false
	second := toolHandler(s, "second", EffectLocal, func(context.Context, struct{}, Progress) (struct{}, int, error) {
		called = true
		return struct{}{}, 0, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err := second(ctx, nil, struct{}{})
	if err == nil || err.Error() != "cancelled; no files were changed" {
		t.Fatalf("cancelled wait = %v", err)
	}
	if called {
		t.Fatal("cancelled write handler ran")
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancelled write waited for preceding writer")
	}
	// The SDK turns ordinary handler errors into isError results, as verified
	// with the real doctor failure in TestToolErrorIsResultNotRPC.
}

func TestWriteMutexCancellationIsErrorResult(t *testing.T) {
	s := New(t.TempDir(), Deps{Env: sys.Default()})
	started, release := make(chan struct{}), make(chan struct{})
	addTool(s, "blocking_write", "Prueba de escritura", EffectLocal, func(context.Context, struct{}, Progress) (struct{}, int, error) {
		close(started)
		<-release
		return struct{}{}, 0, nil
	})
	called := false
	addTool(s, "waiting_write", "Prueba de espera", EffectLocal, func(context.Context, struct{}, Progress) (struct{}, int, error) {
		called = true
		return struct{}{}, 0, nil
	})
	s.MCP.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if call, ok := req.(*mcp.CallToolRequest); ok && method == "tools/call" && call.Params.Name == "waiting_write" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 50*time.Millisecond)
				defer cancel()
			}
			return next(ctx, method, req)
		}
	})
	c := connectTools(t, s, nil)
	firstDone := make(chan error, 1)
	go func() {
		_, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "blocking_write", Arguments: map[string]any{}})
		firstDone <- err
	}()
	<-started
	defer func() {
		close(release)
		if err := <-firstDone; err != nil {
			t.Error(err)
		}
	}()
	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "waiting_write", Arguments: map[string]any{}})
	if err != nil || !r.IsError {
		t.Fatalf("cancelled wait: result=%#v error=%v", r, err)
	}
	b, _ := json.Marshal(r.Content)
	if !strings.Contains(string(b), "cancelled; no files were changed") || called {
		t.Fatalf("cancelled handler ran=%v result=%s", called, b)
	}
}

func TestReadToolsNeverWrite(t *testing.T) {
	root := testutil.NewRepo(t, "", testutil.WithBranch("feat/read-tools"))
	if err := os.WriteFile(filepath.Join(root, "feature.txt"), []byte("feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, root, "add", "feature.txt")
	testutil.Git(t, root, "commit", "-m", "feat(read): exercise context")
	env := sys.Default()
	env.LookPath = func(string) (string, error) { return "", errors.New("test: no external tool") }
	s := New(root, Deps{Env: env})
	c := connectTools(t, s, nil)
	before := testutil.Git(t, root, "status", "--porcelain", "--untracked-files=all")
	for _, name := range []string{"repo_status", "repo_context", "repo_rules", "skills_list", "doctor"} {
		r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: map[string]any{}})
		if err != nil || r.IsError {
			t.Fatalf("%s: result=%#v error=%v", name, r, err)
		}
	}
	if got := testutil.Git(t, root, "status", "--porcelain", "--untracked-files=all"); got != before {
		t.Fatalf("read tools changed tree: before=%q after=%q", before, got)
	}
}

func TestRepoRulesIncludesArchitecture(t *testing.T) {
	root := testutil.NewRepo(t, "")
	path := filepath.Join(root, ".tija", "rules", "architecture", "domain.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	text := "---\nname: domain\nchecks:\n  - id: no-infrastructure\n    severity: high\n    files: 'src/**/*.cs'\n    forbid: 'Infrastructure'\n    message: Keep domain independent\n---\n## domain-boundaries: Domain boundaries\nKeep adapters outside domain.\n"
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	env := sys.Default()
	env.LookPath = func(string) (string, error) { return "", errors.New("test: no external tool") }
	c := connectTools(t, New(root, Deps{Env: env}), nil)
	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "repo_rules", Arguments: map[string]any{}})
	if err != nil || r.IsError {
		t.Fatalf("repo_rules: result=%#v error=%v", r, err)
	}
	b, err := json.Marshal(r.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var report RulesReport
	if err = json.Unmarshal(b, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Architecture) != 1 || len(report.Architecture[0].Checks) != 1 || report.Architecture[0].Checks[0].Forbid != "Infrastructure" || len(report.Architecture[0].Rules) != 1 || report.Architecture[0].Path != ".tija/rules/architecture/domain.md" {
		t.Fatalf("architecture result: %#v", report.Architecture)
	}
	if report.PullRequestRules == "" || !slices.Contains(report.AllowedTypes, "feat") {
		t.Fatalf("missing PR rules: %#v", report)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(text, "'Infrastructure'", "'['")), 0644); err != nil {
		t.Fatal(err)
	}
	r, err = c.CallTool(context.Background(), &mcp.CallToolParams{Name: "repo_rules", Arguments: map[string]any{}})
	if err != nil || !r.IsError {
		t.Fatalf("invalid architecture accepted: result=%#v error=%v", r, err)
	}
}

func TestProgressOnlyWithToken(t *testing.T) {
	s := New(t.TempDir(), Deps{Env: sys.Default()})
	addTool(s, "progress_test", "Prueba de progreso", EffectRead, func(_ context.Context, _ struct{}, p Progress) (struct{}, int, error) {
		p("quality", 0, 0)
		p("checks", 0, 0)
		return struct{}{}, 0, nil
	})
	notifications := make(chan *mcp.ProgressNotificationParams, 4)
	c := connectTools(t, s, &mcp.ClientOptions{ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) { notifications <- r.Params }})
	params := &mcp.CallToolParams{Name: "progress_test", Arguments: map[string]any{}}
	if _, err := c.CallTool(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-notifications:
		t.Fatalf("unexpected progress without token: %#v", p)
	default:
	}
	params.SetProgressToken("token-1")
	if _, err := c.CallTool(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	var previous float64
	for _, stage := range []string{"quality", "checks"} {
		select {
		case p := <-notifications:
			if p.ProgressToken != "token-1" || p.Message != stage || p.Progress <= previous {
				t.Fatalf("progress = %#v", p)
			}
			previous = p.Progress
		case <-time.After(time.Second):
			t.Fatal("missing progress notification")
		}
	}
}

func TestToolExitCodesAndSchemaErrors(t *testing.T) {
	s := New(t.TempDir(), Deps{Env: sys.Default()})
	for _, e := range []struct {
		name string
		exit int
		err  error
	}{{"one", 1, nil}, {"two", 2, nil}, {"failed", 0, errors.New("environment unavailable")}} {
		addTool(s, e.name, "Prueba de resultado", EffectRead, func(context.Context, struct{}, Progress) (struct{}, int, error) { return struct{}{}, e.exit, e.err })
	}
	c := connectTools(t, s, nil)
	for _, name := range []string{"one", "two", "failed"} {
		r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: map[string]any{}})
		if err != nil {
			t.Fatal(err)
		}
		if r.IsError != (name != "one") {
			t.Fatalf("%s IsError=%v", name, r.IsError)
		}
		if name == "one" && r.Meta["tija/exitCode"] != float64(1) {
			t.Fatalf("exit metadata: %#v", r.Meta)
		}
	}
	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "repo_status", Arguments: map[string]any{"dir": "other"}})
	if err != nil || !r.IsError {
		t.Fatalf("invalid dir must be rejected by SDK: result=%#v error=%v", r, err)
	}
	if _, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "unknown_tool", Arguments: map[string]any{}}); err == nil {
		t.Fatal("unknown tool must return a JSON-RPC error")
	}
}
