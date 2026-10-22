package sys_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("TIJA_TEST_HELPER") != "1" {
		return
	}
	for _, arg := range os.Args {
		if arg == "-sleep" {
			time.Sleep(10 * time.Second)
			os.Exit(0)
		}
	}
	os.Exit(0)
}

func TestExecRunCapturesStderr(t *testing.T) {
	env := sys.Default()
	out, stderr, code, err := env.Run(context.Background(), t.TempDir(), "git", "--no-such-option")
	if err != nil {
		t.Fatal(err)
	}
	if code == 0 {
		t.Fatalf("code = 0, want non-zero (stdout %q)", out)
	}
	if !strings.Contains(stderr, "no-such-option") {
		t.Errorf("stderr = %q, want git's message", stderr)
	}
}

func TestStreamCombinesOutputAndExitCode(t *testing.T) {
	env := sys.Default()
	var buf bytes.Buffer
	code, err := env.Stream(context.Background(), t.TempDir(), &buf, "go", "env", "GOROOT")
	if err != nil {
		t.Fatalf("Stream unexpected err: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if buf.Len() == 0 {
		t.Fatal("expected non-empty output")
	}

	buf.Reset()
	code, err = env.Stream(context.Background(), t.TempDir(), &buf, "go", "nosuchcmd")
	if err != nil {
		t.Fatalf("Stream unexpected err on non-zero exit: %v", err)
	}
	if code == 0 {
		t.Fatalf("expected non-zero exit code for go nosuchcmd, got 0")
	}
}

func TestStreamNotFound(t *testing.T) {
	env := sys.Default()
	var buf bytes.Buffer
	code, err := env.Stream(context.Background(), t.TempDir(), &buf, "nonexistent-binary-123456")
	if err == nil {
		t.Fatal("expected error for nonexistent binary, got nil")
	}
	if code != -1 {
		t.Fatalf("code = %d, want -1", code)
	}
}

func TestStreamCancel(t *testing.T) {
	env := sys.Default()
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	t.Setenv("TIJA_TEST_HELPER", "1")
	start := time.Now()
	_, _ = env.Stream(ctx, t.TempDir(), &buf, os.Args[0], "-test.run=TestHelperProcess", "--", "-sleep")
	duration := time.Since(start)
	if duration >= 5*time.Second {
		t.Fatalf("Stream cancellation took too long: %v (want < 5s)", duration)
	}
}
