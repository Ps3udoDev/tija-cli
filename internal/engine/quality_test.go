package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

type streamCall struct {
	dir  string
	name string
	args []string
}

func fakeQualityEnv(streamHandler func(ctx context.Context, dir string, out io.Writer, name string, args ...string) (int, error)) (sys.Env, *[]streamCall) {
	calls := &[]streamCall{}
	real := sys.Default()
	env := sys.Env{
		Run: real.Run,
		LookPath: func(name string) (string, error) {
			if name == "git" {
				return real.LookPath("git")
			}
			return "/bin/" + name, nil
		},
		Getenv: func(string) string { return "" },
		Stream: func(ctx context.Context, dir string, out io.Writer, name string, args ...string) (int, error) {
			*calls = append(*calls, streamCall{dir: dir, name: name, args: args})
			if streamHandler != nil {
				return streamHandler(ctx, dir, out, name, args...)
			}
			return 0, nil
		},
	}
	return env, calls
}

func createFakeNodeBin(t *testing.T, root, name string) {
	t.Helper()
	binDir := filepath.Join(root, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{"", ".cmd", ".exe"} {
		p := filepath.Join(binDir, name+ext)
		if err := os.WriteFile(p, []byte("echo ok"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunQualityFrontendBiome(t *testing.T) {
	root := initRepo(t, "react-vite")
	createFakeNodeBin(t, root, "biome")

	env, calls := fakeQualityEnv(nil)
	req := QualityRequest{Mode: quality.CheckMode}

	report, err := RunQuality(context.Background(), root, env, req, nil)
	if err != nil {
		t.Fatalf("RunQuality: %v", err)
	}
	if report.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0", report.ExitCode())
	}
	if len(*calls) != 1 {
		t.Fatalf("stream calls = %d, want 1", len(*calls))
	}
	call := (*calls)[0]
	if !slices.Contains(call.args, "ci") || !slices.Contains(call.args, ".") {
		t.Errorf("expected biome ci ., got %v", call.args)
	}

	session, err := quality.LoadSession(root)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if session.Results[quality.Lint].Status != "passed" || session.Results[quality.Format].Status != "passed" {
		t.Errorf("session results: %+v", session.Results)
	}
}

func TestRunQualityDotnetBuildFailSkipsRest(t *testing.T) {
	root := initRepo(t, "dotnet-api")
	env, calls := fakeQualityEnv(func(ctx context.Context, dir string, out io.Writer, name string, args ...string) (int, error) {
		if len(args) > 0 && args[0] == "build" {
			return 1, nil
		}
		return 0, nil
	})

	req := QualityRequest{Mode: quality.CheckMode}
	report, err := RunQuality(context.Background(), root, env, req, nil)
	if err != nil {
		t.Fatalf("RunQuality: %v", err)
	}
	if report.ExitCode() != 1 {
		t.Fatalf("ExitCode = %d, want 1", report.ExitCode())
	}
	if len(*calls) != 1 {
		t.Fatalf("calls = %d, want 1 (subsequent steps should be skipped)", len(*calls))
	}
	// Check that format and tests are marked skipped
	for _, step := range report.Steps {
		if slices.Contains(step.Kinds, quality.Build) {
			if step.Status != quality.StatusFailed {
				t.Errorf("build status = %s, want failed", step.Status)
			}
		} else {
			if step.Status != quality.StatusSkipped {
				t.Errorf("step %v status = %s, want skipped", step.Kinds, step.Status)
			}
		}
	}
}

func TestRunQualityTestCounts(t *testing.T) {
	root := initRepo(t, "dotnet-api")
	env, _ := fakeQualityEnv(func(ctx context.Context, dir string, out io.Writer, name string, args ...string) (int, error) {
		// If dotnet test with results-directory, write a sample trx
		for i, a := range args {
			if a == "--results-directory" && i+1 < len(args) {
				trxDir := args[i+1]
				_ = os.MkdirAll(trxDir, 0o755)
				trxContent := `<?xml version="1.0" encoding="utf-8"?>
<TestRun id="1" name="r" xmlns="http://microsoft.com/schemas/VisualStudio/TeamTest/2010">
  <ResultSummary outcome="Completed">
    <Counters total="3" executed="3" passed="3" failed="0" error="0" timeout="0" notExecuted="0" />
  </ResultSummary>
</TestRun>`
				_ = os.WriteFile(filepath.Join(trxDir, "test.trx"), []byte(trxContent), 0o644)
			}
		}
		return 0, nil
	})

	req := QualityRequest{Mode: quality.CheckMode, Kinds: []quality.Kind{quality.Tests}}
	report, err := RunQuality(context.Background(), root, env, req, nil)
	if err != nil {
		t.Fatalf("RunQuality: %v", err)
	}
	if len(report.Steps) != 1 || report.Steps[0].Tests == nil {
		t.Fatalf("expected 1 step with Tests, got %+v", report.Steps)
	}
	tests := report.Steps[0].Tests
	if tests.Total != 3 || tests.Passed != 3 || tests.Failed != 0 || tests.Skipped != 0 {
		t.Errorf("Tests = %+v, want {3,3,0,0}", tests)
	}
}

func TestRunQualityUnavailableTool(t *testing.T) {
	root := initRepo(t, "react-vite")
	// No node_modules created, and LookPath returns not found for biome
	real := sys.Default()
	env := sys.Env{
		Run: real.Run,
		LookPath: func(name string) (string, error) {
			if name == "git" {
				return real.LookPath("git")
			}
			return "", os.ErrNotExist
		},
		Stream: func(ctx context.Context, dir string, out io.Writer, name string, args ...string) (int, error) {
			t.Fatalf("Stream should not be called when tool is unavailable")
			return 0, nil
		},
	}

	req := QualityRequest{Mode: quality.CheckMode}
	report, err := RunQuality(context.Background(), root, env, req, nil)
	if err != nil {
		t.Fatalf("RunQuality: %v", err)
	}
	if report.ExitCode() != 2 {
		t.Fatalf("ExitCode = %d, want 2", report.ExitCode())
	}
	if len(report.Steps) != 1 || report.Steps[0].Status != quality.StatusUnavailable {
		t.Fatalf("Step = %+v, want unavailable", report.Steps)
	}
	if !strings.Contains(report.Steps[0].Hint, "bun install") {
		t.Errorf("Hint = %q, want bun install", report.Steps[0].Hint)
	}
}

func TestRunQualityUninitializedWritesNothing(t *testing.T) {
	// A repo without .tija
	root := testutil.NewRepo(t, fixture("react-vite"))
	createFakeNodeBin(t, root, "biome")

	env, _ := fakeQualityEnv(nil)
	req := QualityRequest{Mode: quality.CheckMode}

	report, err := RunQuality(context.Background(), root, env, req, nil)
	if err != nil {
		t.Fatalf("RunQuality: %v", err)
	}
	if report.Initialized {
		t.Error("Initialized should be false")
	}
	foundWarning := false
	for _, w := range report.Warnings {
		if strings.Contains(w, "not initialized") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("Warnings missing 'not initialized': %v", report.Warnings)
	}

	// Verify .tija does not exist
	if _, err := os.Stat(filepath.Join(root, ".tija")); !os.IsNotExist(err) {
		t.Fatalf(".tija exists after running on uninitialized repo")
	}
}

func TestRunQualityTimeout(t *testing.T) {
	root := initRepo(t, "dotnet-api")
	env, _ := fakeQualityEnv(func(ctx context.Context, dir string, out io.Writer, name string, args ...string) (int, error) {
		<-ctx.Done()
		return -1, ctx.Err()
	})

	req := QualityRequest{
		Mode:    quality.CheckMode,
		Timeout: 50 * time.Millisecond,
	}

	report, err := RunQuality(context.Background(), root, env, req, nil)
	if err != nil {
		t.Fatalf("RunQuality: %v", err)
	}
	if report.ExitCode() != 1 {
		t.Fatalf("ExitCode = %d, want 1", report.ExitCode())
	}
	if len(report.Steps) == 0 || !report.Steps[0].TimedOut || report.Steps[0].Status != quality.StatusFailed {
		t.Fatalf("first step = %+v, want TimedOut and Failed", report.Steps[0])
	}
	if len(report.Steps) > 1 && report.Steps[1].Status != quality.StatusSkipped {
		t.Errorf("subsequent step = %+v, want skipped", report.Steps[1])
	}
}

func TestRunQualityLogsAndTail(t *testing.T) {
	root := initRepo(t, "dotnet-api")
	env, _ := fakeQualityEnv(func(ctx context.Context, dir string, out io.Writer, name string, args ...string) (int, error) {
		for i := 0; i < 1000; i++ {
			_, _ = fmt.Fprintf(out, "build output line %d\n", i)
		}
		return 0, nil
	})

	req := QualityRequest{Mode: quality.CheckMode, Kinds: []quality.Kind{quality.Build}}
	report, err := RunQuality(context.Background(), root, env, req, nil)
	if err != nil {
		t.Fatalf("RunQuality: %v", err)
	}
	if len(report.Steps) != 1 {
		t.Fatalf("steps len = %d", len(report.Steps))
	}
	step := report.Steps[0]
	if len(step.Tail) != 40 {
		t.Errorf("len(Tail) = %d, want 40", len(step.Tail))
	}
	wantLog := ".tija/session/logs/build.log"
	if step.Log != wantLog {
		t.Errorf("step.Log = %q, want %q", step.Log, wantLog)
	}

	logData, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wantLog)))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
	if len(lines) != 1000 {
		t.Errorf("log line count = %d, want 1000", len(lines))
	}
}

func TestRunQualityFixReportsChangedFiles(t *testing.T) {
	root := initRepo(t, "react-vite")
	createFakeNodeBin(t, root, "biome")

	env, _ := fakeQualityEnv(func(ctx context.Context, dir string, out io.Writer, name string, args ...string) (int, error) {
		// Modify a file in repo
		srcFile := filepath.Join(dir, "src", "app.js")
		_ = os.MkdirAll(filepath.Dir(srcFile), 0o755)
		_ = os.WriteFile(srcFile, []byte("formatted content"), 0o644)
		return 0, nil
	})

	req := QualityRequest{Mode: quality.FixMode}
	report, err := RunQuality(context.Background(), root, env, req, nil)
	if err != nil {
		t.Fatalf("RunQuality: %v", err)
	}
	if !slices.Contains(report.ChangedFiles, "src/app.js") {
		t.Errorf("ChangedFiles = %v, want src/app.js", report.ChangedFiles)
	}

	// Session must not be written in FixMode
	sessionPath := filepath.Join(root, filepath.FromSlash(quality.SessionPath))
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Errorf("session file exists after FixMode")
	}
}

func TestRunQualityNothingConfigured(t *testing.T) {
	root := testutil.NewRepo(t, fixture("unknown"))
	env, _ := fakeQualityEnv(nil)
	req := QualityRequest{Mode: quality.CheckMode}

	report, err := RunQuality(context.Background(), root, env, req, nil)
	if err != nil {
		t.Fatalf("RunQuality: %v", err)
	}
	if len(report.Steps) != 0 {
		t.Fatalf("expected 0 steps, got %d", len(report.Steps))
	}
	if report.ExitCode() != 2 {
		t.Fatalf("ExitCode = %d, want 2", report.ExitCode())
	}
}

func TestRunQualityEvents(t *testing.T) {
	root := initRepo(t, "dotnet-api")
	env, _ := fakeQualityEnv(nil)

	var events []QualityEvent
	onEvent := func(ev QualityEvent) {
		events = append(events, ev)
	}

	req := QualityRequest{
		Mode:  quality.CheckMode,
		Kinds: []quality.Kind{quality.Build, quality.Format},
	}
	report, err := RunQuality(context.Background(), root, env, req, onEvent)
	if err != nil {
		t.Fatalf("RunQuality: %v", err)
	}
	if len(report.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(report.Steps))
	}
	// Expected 4 events: start(0), done(0), start(1), done(1)
	if len(events) != 4 {
		t.Fatalf("events len = %d, want 4", len(events))
	}
	if events[0].Type != "start" || events[0].Index != 0 || events[0].Total != 2 {
		t.Errorf("event 0 = %+v", events[0])
	}
	if events[1].Type != "done" || events[1].Index != 0 || events[1].Total != 2 {
		t.Errorf("event 1 = %+v", events[1])
	}
	if events[2].Type != "start" || events[2].Index != 1 || events[2].Total != 2 {
		t.Errorf("event 2 = %+v", events[2])
	}
	if events[3].Type != "done" || events[3].Index != 1 || events[3].Total != 2 {
		t.Errorf("event 3 = %+v", events[3])
	}
}
