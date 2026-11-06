package mcpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/cli"
	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const localAgentDraft = `<<<TITLE
feat(local): agregar comprobaciones locales
TITLE>>>
<<<BODY
## Resumen

Se agregan comprobaciones locales para revisar los cambios.

## Objetivo

Verificar el comportamiento de las herramientas del repositorio.

## Cambios realizados

- Se agrega una función funcional en feature.ts.

## Pruebas

No se ejecutaron pruebas para este commit.

## Evidencias

N/A
BODY>>>`

func localParityRepo(t *testing.T, forbid bool) string {
	t.Helper()
	root := testutil.NewRepo(t, "")
	m := manifest.FromDetection(detect.Detection{ProjectName: "local-tools", Stack: detect.StackUnknown}, nil, "main")
	m.Quality = map[string]manifest.QualityEntry{"build": {Provider: "none"}, "lint": {Provider: "none"}, "format": {Provider: "none"}, "tests": {Provider: "script", Script: "test"}}
	if err := manifest.Save(root, m); err != nil {
		t.Fatal(err)
	}
	localWrite(t, root, ".gitignore", ".tija/session/\n.tija/tmp/\n")
	localWrite(t, root, "package.json", `{"name":"local-tools","scripts":{"test":"echo captured stdout"}}`)
	if forbid {
		localWrite(t, root, ".tija/rules/architecture/local.md", "---\nname: local\nchecks:\n  - id: local-forbid-bad\n    severity: High\n    files: '*.ts'\n    forbid: bad\n    message: forbidden marker\n---\n")
	}
	testutil.Git(t, root, "add", ".")
	testutil.Git(t, root, "commit", "-m", "chore: initialize local tools")
	testutil.Git(t, root, "checkout", "-b", "feat/local-tools")
	localWrite(t, root, "feature.ts", "export const bad = true\n")
	testutil.Git(t, root, "add", ".")
	testutil.Git(t, root, "commit", "-m", "feat(local): add feature")
	return root
}

func localWrite(t *testing.T, root, path, text string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

func localEnv() (sys.Env, *atomic.Int32) {
	env := sys.Default()
	look := env.LookPath
	env.LookPath = func(name string) (string, error) {
		if name == "git" {
			return look(name)
		}
		if name == "npm" || name == "claude" {
			return "/fake/bin/" + name, nil
		}
		return "", errors.New("test: tool unavailable")
	}
	env.Stream = func(_ context.Context, _ string, out io.Writer, _ string, _ ...string) (int, error) {
		fmt.Fprintln(out, "captured stdout")
		return 0, nil
	}
	calls := &atomic.Int32{}
	env.RunInput = func(_ context.Context, _, input, _ string, _ ...string) (string, string, int, error) {
		calls.Add(1)
		if strings.Contains(input, "<<<TITLE") {
			return localAgentDraft, "", 0, nil
		}
		return `{"findings":[]}`, "", 0, nil
	}
	return env, calls
}

func callLocal(t *testing.T, c *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil || r.IsError {
		t.Fatalf("%s: result=%#v error=%v", name, r, err)
	}
	return r
}

func cliLocal(t *testing.T, env sys.Env, expected int, args ...string) []byte {
	t.Helper()
	var out, errOut bytes.Buffer
	args = append(args, "--json")
	if code := cli.ExecuteWith(cli.Deps{Env: env}, args, strings.NewReader(""), &out, &errOut); code != expected {
		t.Fatalf("CLI %v: exit=%d want=%d out=%s err=%s", args, code, expected, out.String(), errOut.String())
	}
	return out.Bytes()
}

func TestQualityCheckParity(t *testing.T) {
	root := localParityRepo(t, false)
	t.Chdir(root)
	env, _ := localEnv()
	c := paritySession(t, root, env)
	r := callLocal(t, c, "quality_check", map[string]any{})
	cliJSON := cliLocal(t, env, 0, "check")
	// Only wall-clock fields differ between two real invocations. Keep all
	// other fields, including commands, paths, status and empty-array shapes.
	b, err := json.Marshal(r.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var got, want map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(cliJSON, &want); err != nil {
		t.Fatal(err)
	}
	for _, result := range []map[string]any{got, want} {
		for _, step := range result["steps"].([]any) {
			s := step.(map[string]any)
			s["durationMs"], s["startedAt"] = float64(0), "<clock>"
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("quality differs: MCP=%s CLI=%s", b, cliJSON)
	}
	if r.Meta["tija/exitCode"] != float64(0) {
		t.Fatalf("exit metadata=%v", r.Meta)
	}
}

func TestReviewRunParity(t *testing.T) {
	root := localParityRepo(t, false)
	t.Chdir(root)
	env, calls := localEnv()
	c := paritySession(t, root, env)
	r := callLocal(t, c, "review_run", map[string]any{"agent": "claude", "noCheck": true, "noPRComments": true, "mode": "deep"})
	assertJSONParity(t, r.StructuredContent, cliLocal(t, env, 0, "review", "--agent", "claude", "--no-check", "--no-pr-comments", "--deep"))
	b, _ := json.Marshal(r.StructuredContent)
	var report struct {
		Chunks struct{ Total, Failed int } `json:"chunks"`
	}
	if err := json.Unmarshal(b, &report); err != nil || report.Chunks.Total == 0 || report.Chunks.Failed != 0 {
		t.Fatalf("fake review agent did not complete: %s (%v)", b, err)
	}
	if calls.Load() < 2 {
		t.Fatalf("fake agent not exercised on both paths: %d", calls.Load())
	}
}

func TestPRPrepareParity(t *testing.T) {
	root := localParityRepo(t, false)
	t.Chdir(root)
	env, calls := localEnv()
	c := paritySession(t, root, env)
	evidence := map[string]any{"source": "qapture", "project": map[string]any{"id": "local-project"}, "exportPath": `C:\private\matrix.xlsx`, "notes": "Verified local flow"}
	r := callLocal(t, c, "pr_prepare", map[string]any{"agent": "claude", "noCheck": true, "evidenceRefs": []any{evidence}})
	evidenceFile := filepath.Join(t.TempDir(), "evidence.json")
	b, _ := json.Marshal(map[string]any{"evidenceRefs": []any{evidence}})
	if err := os.WriteFile(evidenceFile, b, 0644); err != nil {
		t.Fatal(err)
	}
	assertJSONParity(t, r.StructuredContent, cliLocal(t, env, 0, "pr", "prepare", "--agent", "claude", "--no-check", "--evidence", evidenceFile))
	var report struct {
		Source   string `json:"source"`
		Attempts int    `json:"attempts"`
	}
	b, _ = json.Marshal(r.StructuredContent)
	if err := json.Unmarshal(b, &report); err != nil || report.Source != "claude" || report.Attempts != 1 {
		t.Fatalf("fake drafting agent did not complete: %s (%v)", b, err)
	}
	if calls.Load() < 2 {
		t.Fatalf("fake agent not exercised on both paths: %d", calls.Load())
	}
}

func TestReviewRunExitOneIsNotError(t *testing.T) {
	root := localParityRepo(t, true)
	t.Chdir(root)
	env, _ := localEnv()
	r := callLocal(t, paritySession(t, root, env), "review_run", map[string]any{"agent": "none", "noCheck": true, "noPRComments": true})
	b, _ := json.Marshal(r.StructuredContent)
	var report struct {
		Exit     int `json:"exit"`
		Findings []struct {
			Severity string `json:"severity"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(b, &report); err != nil {
		t.Fatal(err)
	}
	if r.Meta["tija/exitCode"] != float64(1) || report.Exit != 1 || len(report.Findings) != 1 || report.Findings[0].Severity != "High" {
		t.Fatalf("not a normal High result: meta=%v report=%s", r.Meta, b)
	}
}

func TestLocalToolsNeverTouchTrackedFiles(t *testing.T) {
	root := localParityRepo(t, false)
	env, _ := localEnv()
	c := paritySession(t, root, env)
	before := testutil.Git(t, root, "status", "--porcelain", "--untracked-files=all")
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"quality_check", map[string]any{}},
		{"review_run", map[string]any{"agent": "none", "noCheck": true, "noPRComments": true}},
		{"pr_prepare", map[string]any{"agent": "none", "noCheck": true}},
	} {
		callLocal(t, c, tc.name, tc.args)
	}
	if got := testutil.Git(t, root, "status", "--porcelain", "--untracked-files=all"); got != before {
		t.Fatalf("local tools changed visible worktree: before=%q after=%q", before, got)
	}
	for _, path := range []string{".tija/session/quality.json", ".tija/session/review.json", ".tija/session/pr.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("expected local artifact %s: %v", path, err)
		}
	}
}

func TestPRPrepareEvidenceValidationBlock(t *testing.T) {
	root := localParityRepo(t, false)
	env, _ := localEnv()
	r := callLocal(t, paritySession(t, root, env), "pr_prepare", map[string]any{"agent": "none", "noCheck": true, "evidenceRefs": []any{map[string]any{"source": "qapture", "project": map[string]any{"id": "local-project"}, "notes": "Functional flow verified"}}})
	b, _ := json.Marshal(r.StructuredContent)
	var report struct {
		EvidenceValidation map[string]string `json:"evidenceValidation"`
	}
	if err := json.Unmarshal(b, &report); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report.EvidenceValidation, map[string]string{"structural": "passed", "externalExistence": "not_checked"}) {
		t.Fatalf("validation=%s", b)
	}
}

func TestLocalToolInputValidation(t *testing.T) {
	root := localParityRepo(t, false)
	env, calls := localEnv()
	c := paritySession(t, root, env)
	before := testutil.Git(t, root, "status", "--porcelain", "--untracked-files=all")
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"quality_check", map[string]any{"kinds": []string{"build"}}},
		{"quality_check", map[string]any{"kinds": []string{}}},
		{"quality_check", map[string]any{"timeoutSec": 0}},
		{"quality_check", map[string]any{"timeoutSec": -1}},
		{"review_run", map[string]any{"mode": "invalid"}},
		{"review_run", map[string]any{"failOn": "invalid"}},
		{"review_run", map[string]any{"agent": "invalid"}},
		{"pr_prepare", map[string]any{"agent": "invalid"}},
		{"pr_prepare", map[string]any{"evidenceRefs": []any{map[string]any{"source": "jira"}}}},
		{"quality_check", map[string]any{"dir": "other"}},
	} {
		r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
		if err != nil || !r.IsError {
			t.Fatalf("invalid %s %v accepted: result=%#v err=%v", tc.name, tc.args, r, err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid input invoked agent: %d", calls.Load())
	}
	if got := testutil.Git(t, root, "status", "--porcelain", "--untracked-files=all"); got != before {
		t.Fatalf("invalid input changed tree: %q", got)
	}
}
