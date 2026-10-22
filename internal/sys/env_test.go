package sys

import (
	"context"
	"strings"
	"testing"
)

func TestExecRunCapturesStderr(t *testing.T) {
	out, stderr, code, err := execRun(context.Background(), t.TempDir(), "git", "--no-such-option")
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
