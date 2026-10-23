//go:build windows

package sys_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

// pingCount is how many PING.EXE processes are running.
func pingCount(t *testing.T) int {
	t.Helper()
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq PING.EXE", "/NH").Output()
	if err != nil {
		t.Skipf("tasklist unavailable: %v", err)
	}
	return strings.Count(strings.ToUpper(string(out)), "PING.EXE")
}

// TestRunInputTimeoutKillsTree checks that a timeout kills the whole
// process tree: cmd.exe and the ping it started. Killing only cmd.exe
// would leave ping running and holding the output pipes.
func TestRunInputTimeoutKillsTree(t *testing.T) {
	before := pingCount(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	_, _, _, _ = sys.Default().RunInput(ctx, t.TempDir(), "", "cmd", "/c", "ping -n 30 127.0.0.1 >NUL")
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("RunInput returned after %s, want < 5s", d)
	}
	deadline := time.Now().Add(3 * time.Second)
	n := pingCount(t)
	for n > before && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		n = pingCount(t)
	}
	if n > before {
		t.Errorf("PING.EXE processes: %d before, %d after the timeout — the child survived", before, n)
	}
}

func TestStreamTimeoutKillsTree(t *testing.T) {
	before := pingCount(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	var out strings.Builder
	_, _ = sys.Default().Stream(ctx, t.TempDir(), &out, "cmd", "/c", "ping -n 30 127.0.0.1")
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("Stream returned after %s, want < 5s", d)
	}
	deadline := time.Now().Add(3 * time.Second)
	n := pingCount(t)
	for n > before && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		n = pingCount(t)
	}
	if n > before {
		t.Errorf("PING.EXE processes: %d before, %d after the timeout — the child survived", before, n)
	}
}
