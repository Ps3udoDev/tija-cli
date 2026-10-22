package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/render"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

// cmdDeps finds only git on PATH and reports every stream as TTY or not.
func cmdDeps(isTTY bool) Deps {
	return Deps{
		Env: sys.Env{
			Run: sys.Default().Run,
			LookPath: func(name string) (string, error) {
				if name == "git" {
					return exec.LookPath("git")
				}
				return "", errors.New("not found: " + name)
			},
			Getenv: func(string) string { return "" },
		},
		IsTTY: func(any) bool { return isTTY },
	}
}

// runIn runs the CLI with cwd set to dir and stdin fed from in.
func runIn(t *testing.T, dir string, deps Deps, in string, args ...string) (int, string, string) {
	t.Helper()
	t.Chdir(dir)
	var out, errOut bytes.Buffer
	code := ExecuteWith(deps, args, strings.NewReader(in), &out, &errOut)
	return code, out.String(), errOut.String()
}

func fixture(name string) string {
	p, err := filepath.Abs(filepath.Join("..", "detect", "testdata", name))
	if err != nil {
		panic(err)
	}
	return p
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return string(b)
}

func TestInitCommandNonTTY(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	before := readFile(t, filepath.Join(repo, ".gitignore"))
	code, out, errOut := runIn(t, repo, cmdDeps(false), "", "init")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(out, "Pending") {
		t.Errorf("out lacks Pending:\n%s", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("ANSI in non-TTY output")
	}
	if after := readFile(t, filepath.Join(repo, ".gitignore")); after != before {
		t.Errorf(".gitignore changed without consent: %q", after)
	}
}

func TestInitCommandYes(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	code, out, errOut := runIn(t, repo, cmdDeps(false), "", "init", "--yes")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(readFile(t, filepath.Join(repo, ".gitignore")), ".tija/cache/") {
		t.Error(".gitignore lacks .tija/cache/")
	}
	if strings.Contains(out, "Pending") {
		t.Errorf("--yes left pending actions:\n%s", out)
	}
}

func TestInitCommandInteractive(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	plan, err := engine.PlanInit(context.Background(), repo, cmdDeps(true).Env)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Choices) != 0 {
		t.Fatalf("plan has choices %+v; answers below assume none", plan.Choices)
	}
	consent := 0
	for _, a := range plan.Actions {
		if a.NeedsConsent {
			consent++
		}
	}
	if consent == 0 {
		t.Fatal("plan has no consent actions")
	}
	// Answer yes to exactly the consent questions.
	code, out, errOut := runIn(t, repo, cmdDeps(true), strings.Repeat("y\n", consent), "init", "--no-color")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(out, "[y/N]") {
		t.Errorf("no consent prompt:\n%s", out)
	}
	if !strings.Contains(readFile(t, filepath.Join(repo, ".gitignore")), ".tija/cache/") {
		t.Error(".gitignore not updated after approval")
	}
}

func TestInitCommandJSONNeverPrompts(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	code, out, errOut := runIn(t, repo, cmdDeps(true), "y\ny\ny\n", "init", "--json")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if p, _ := v["pending"].([]any); len(p) == 0 {
		t.Errorf("pending = %v, want the consent actions", v["pending"])
	}
}

func TestInitOutsideRepoExit2(t *testing.T) {
	code, _, errOut := runIn(t, t.TempDir(), cmdDeps(false), "", "init")
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "not a git repository") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestDoctorJSON(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	code, out, errOut := runIn(t, repo, cmdDeps(true), "", "doctor", "--json")
	// gh is not on the fake PATH, so doctor fails its check.
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want empty", errOut)
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("ANSI in JSON output")
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if _, ok := v["checks"].([]any); !ok {
		t.Errorf("no checks array: %s", out)
	}
	if d, _ := v["detection"].(map[string]any); d["framework"] != "React 19 + Vite 7" {
		t.Errorf("detection = %v", v["detection"])
	}
}

func TestDoctorPlainShowsDetection(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	_, out, _ := runIn(t, repo, cmdDeps(false), "", "doctor")
	for _, want := range []string{"Detected project\n", "Framework:      React 19 + Vite 7\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("out lacks %q:\n%s", want, out)
		}
	}
}

func TestStatusStrictExit1(t *testing.T) {
	repo := testutil.NewRepo(t, "", testutil.WithBranch("changes"))
	code, out, errOut := runIn(t, repo, cmdDeps(false), "", "status", "--strict")
	if code != 1 {
		t.Errorf("--strict code = %d, want 1", code)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want no extra line", errOut)
	}
	if !strings.Contains(out, `! branch "changes"`) {
		t.Errorf("out = %q", out)
	}
	if code, _, _ := runIn(t, repo, cmdDeps(false), "", "status"); code != 0 {
		t.Errorf("code = %d, want 0", code)
	}
}

func TestStatusOutsideRepoExit2(t *testing.T) {
	code, _, _ := runIn(t, t.TempDir(), cmdDeps(false), "", "status")
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
}

func TestStdinPrompter(t *testing.T) {
	c := engine.Choice{ID: "target", Question: "Target?", Options: []string{"main", "dev"}, Default: "main"}
	cases := []struct{ in, want string }{
		{"2\n", "dev"}, {"dev\n", "dev"}, {"\n", "main"}, {"", "main"}, {"x\n1\n", "main"},
	}
	for _, tc := range cases {
		var out bytes.Buffer
		pr := newStdinPrompter(strings.NewReader(tc.in), render.Printer{W: &out, Mode: render.Plain})
		if got := pr.Choose(c); got != tc.want {
			t.Errorf("Choose(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	a := engine.Action{Kind: engine.KindUpdate, Path: ".gitignore", Diff: "+.tija/cache/\n"}
	for in, want := range map[string]bool{"y\n": true, "YES\n": true, "n\n": false, "\n": false, "": false} {
		var out bytes.Buffer
		pr := newStdinPrompter(strings.NewReader(in), render.Printer{W: &out, Mode: render.Plain})
		if got := pr.Confirm(a); got != want {
			t.Errorf("Confirm(%q) = %v, want %v", in, got, want)
		}
		if !strings.Contains(out.String(), "+.tija/cache/") {
			t.Errorf("diff not shown: %q", out.String())
		}
	}
}

func TestGitFailureExits2WithMessage(t *testing.T) {
	deps := cmdDeps(false)
	deps.Env.Run = func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
		return "", "fatal: detected dubious ownership in repository at 'C:/x'\n", 128, nil
	}
	for _, cmd := range []string{"init", "status"} {
		code, _, errOut := runIn(t, t.TempDir(), deps, "", cmd)
		if code != 2 || !strings.Contains(errOut, "detected dubious ownership") {
			t.Errorf("%s: code = %d, stderr = %q", cmd, code, errOut)
		}
	}
}


func TestCheckCommandJSON(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	// Initialize manifest
	data := []byte("version: 1\nproject:\n  name: gmx\n  type: react-vite\nquality:\n  - kind: lint\n    provider: biome\n")
	if err := os.MkdirAll(filepath.Join(repo, ".tija"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".tija", "manifest.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	deps := cmdDeps(false)
	deps.Env.LookPath = func(name string) (string, error) {
		if name == "git" {
			return exec.LookPath("git")
		}
		if name == "biome" || name == "bun" {
			return "/bin/" + name, nil
		}
		return "", errors.New("not found: " + name)
	}
	deps.Env.Stream = func(ctx context.Context, dir string, out io.Writer, name string, args ...string) (int, error) {
		return 0, nil
	}

	code, out, errOut := runIn(t, repo, deps, "", "check", "--json")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
	var rep engine.QualityReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("unmarshal error: %v\nout:\n%s", err, out)
	}
	if rep.Mode != "check" {
		t.Errorf("rep.Mode = %q, want 'check'", rep.Mode)
	}
	if code != rep.ExitCode() {
		t.Errorf("exit code = %d, want rep.ExitCode() = %d", code, rep.ExitCode())
	}
}

func TestLintFixRunsFixMode(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	data := []byte("version: 1\nproject:\n  name: gmx\n  type: react-vite\nquality:\n  - kind: lint\n    provider: biome\n")
	_ = os.MkdirAll(filepath.Join(repo, ".tija"), 0o755)
	_ = os.WriteFile(filepath.Join(repo, ".tija", "manifest.yaml"), data, 0o644)

	var streamCalls [][]string
	deps := cmdDeps(false)
	deps.Env.LookPath = func(name string) (string, error) {
		if name == "git" {
			return exec.LookPath("git")
		}
		if name == "biome" || name == "bun" {
			return "/bin/" + name, nil
		}
		return "", errors.New("not found: " + name)
	}
	deps.Env.Stream = func(ctx context.Context, dir string, out io.Writer, name string, args ...string) (int, error) {
		streamCalls = append(streamCalls, append([]string{name}, args...))
		return 0, nil
	}

	code, out, errOut := runIn(t, repo, deps, "", "lint", "--fix")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q, out = %q", code, errOut, out)
	}
	foundFix := false
	for _, call := range streamCalls {
		if slices.Contains(call, "check") && slices.Contains(call, "--write") {
			foundFix = true
			break
		}
	}
	if !foundFix {
		t.Errorf("stream calls did not include biome check --write: %v", streamCalls)
	}
}

func TestTestCommandOnlyTests(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("dotnet-api"))
	data := []byte("version: 1\nproject:\n  name: Claims\n  type: dotnet-api\nquality:\n  - kind: build\n    provider: dotnet\n  - kind: tests\n    provider: dotnet-test\n")
	_ = os.MkdirAll(filepath.Join(repo, ".tija"), 0o755)
	_ = os.WriteFile(filepath.Join(repo, ".tija", "manifest.yaml"), data, 0o644)

	var streamCalls [][]string
	deps := cmdDeps(false)
	deps.Env.LookPath = func(name string) (string, error) {
		if name == "git" {
			return exec.LookPath("git")
		}
		if name == "dotnet" {
			return "/bin/dotnet", nil
		}
		return "", errors.New("not found: " + name)
	}
	deps.Env.Stream = func(ctx context.Context, dir string, out io.Writer, name string, args ...string) (int, error) {
		streamCalls = append(streamCalls, append([]string{name}, args...))
		return 0, nil
	}

	code, _, errOut := runIn(t, repo, deps, "", "test")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
	for _, call := range streamCalls {
		if slices.Contains(call, "build") {
			t.Errorf("unexpected build call: %v", call)
		}
		if !slices.Contains(call, "test") {
			t.Errorf("call does not contain test: %v", call)
		}
	}
	if len(streamCalls) == 0 {
		t.Errorf("expected test call, got none")
	}
}

func TestCheckTimeoutFlag(t *testing.T) {
	code, _, _ := runIn(t, t.TempDir(), cmdDeps(false), "", "check", "--timeout", "x")
	if code != 2 {
		t.Errorf("code = %d, want 2", code)
	}
}
