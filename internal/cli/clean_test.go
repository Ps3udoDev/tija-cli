package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func TestCleanCmdFlagsValidation(t *testing.T) {
	repo := testutil.NewRepo(t, filepath.Join("..", "engine", "testdata", "comments-repo"))

	// --concurrency <= 0 returns exit 2
	code, _, errOut := runIn(t, repo, cmdDeps(false), "", "clean", "comments", "--concurrency", "0")
	if code != 2 {
		t.Errorf("code = %d, want 2 for --concurrency 0", code)
	}
	if !strings.Contains(errOut, "--concurrency must be > 0") {
		t.Errorf("expected '--concurrency must be > 0' in stderr, got: %q", errOut)
	}

	code, _, errOut = runIn(t, repo, cmdDeps(false), "", "clean", "comments", "--concurrency", "-1")
	if code != 2 {
		t.Errorf("code = %d, want 2 for --concurrency -1", code)
	}
	if !strings.Contains(errOut, "--concurrency must be > 0") {
		t.Errorf("expected '--concurrency must be > 0' in stderr, got: %q", errOut)
	}
}

func TestCleanCmdUnsafeWithoutWriteValid(t *testing.T) {
	repo := testutil.NewRepo(t, filepath.Join("..", "engine", "testdata", "comments-repo"))

	// --unsafe without --write is valid (broadens report scope, exit 1 if changes pending)
	code, out, errOut := runIn(t, repo, cmdDeps(false), "", "clean", "comments", "--all", "--no-agent", "--unsafe")
	if code != 1 {
		t.Errorf("code = %d, want 1 for pending changes, stderr: %q", code, errOut)
	}
	if !strings.Contains(out, "Scope: all") {
		t.Errorf("expected report output in stdout, got:\n%s", out)
	}
}

func TestCleanCmdOutsideRepoExit2(t *testing.T) {
	code, _, errOut := runIn(t, t.TempDir(), cmdDeps(false), "", "clean", "comments")
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "not a git repository") {
		t.Errorf("stderr = %q, want not a git repository", errOut)
	}
}

func TestCleanCmdExitCodes(t *testing.T) {
	repo := testutil.NewRepo(t, filepath.Join("..", "engine", "testdata", "comments-repo"))

	// 1. Report-only mode with deterministic rules finds comments to clean -> ExitCode 1
	code, out, errOut := runIn(t, repo, cmdDeps(false), "", "clean", "comments", "--all", "--no-agent")
	if code != 1 {
		t.Fatalf("clean comments report-only code = %d, want 1, errOut: %s", code, errOut)
	}
	if !strings.Contains(out, "run with --write to apply safe changes") {
		t.Errorf("expected hint to run with --write, got:\n%s", out)
	}

	// 2. Write safe changes -> ExitCode 0
	code, out, errOut = runIn(t, repo, cmdDeps(false), "", "clean", "comments", "--all", "--no-agent", "--write")
	if code != 0 {
		t.Fatalf("clean comments --write code = %d, want 0, errOut: %s", code, errOut)
	}
	if !strings.Contains(out, "✓ src/Component.jsx") {
		t.Errorf("expected Component.jsx written, got:\n%s", out)
	}

	// 3. Re-run after writing -> no safe changes pending -> ExitCode 0
	code, out, errOut = runIn(t, repo, cmdDeps(false), "", "clean", "comments", "--all", "--no-agent")
	if code != 0 {
		t.Fatalf("clean comments after write code = %d, want 0, errOut: %s", code, errOut)
	}
	if strings.Contains(out, "run with --write to apply safe changes") {
		t.Errorf("should not prompt to write when no safe changes pending:\n%s", out)
	}
}

func TestCleanCmdJSONOutput(t *testing.T) {
	repo := testutil.NewRepo(t, filepath.Join("..", "engine", "testdata", "comments-repo"))

	code, out, errOut := runIn(t, repo, cmdDeps(false), "", "clean", "comments", "--all", "--no-agent", "--json")
	if code != 1 {
		t.Fatalf("code = %d, want 1, errOut = %s", code, errOut)
	}
	if !strings.Contains(out, `"summary"`) || !strings.Contains(out, `"scope"`) {
		t.Errorf("expected JSON output, got:\n%s", out)
	}
}
