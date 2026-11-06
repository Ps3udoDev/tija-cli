package mcpserver_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A real SDK client sends notifications/cancelled while an actual subprocess
// is blocked. Agent output is synthetic; no model credentials are used.
func TestMCPClientCancellationKillsNestedAgent(t *testing.T) {
	root := localParityRepo(t, false)
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "started")
	t.Setenv("TIJA_QA_CANCEL_MARKER", marker)
	source := filepath.Join(tmp, "agent.go")
	program := "package main\nimport(\"os\";\"time\")\nfunc main(){os.WriteFile(os.Getenv(\"TIJA_QA_CANCEL_MARKER\"),[]byte(\"running\"),0600);time.Sleep(10*time.Minute)}\n"
	if err := os.WriteFile(source, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	name := "claude"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(tmp, name)
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer buildCancel()
	if out, err := exec.CommandContext(buildCtx, "go", "build", "-o", bin, source).CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, out)
	}
	env := sys.Default()
	look, runInput := env.LookPath, env.RunInput
	env.LookPath = func(name string) (string, error) {
		if name == "claude" {
			return bin, nil
		}
		if name == "git" {
			return look(name)
		}
		return "", errors.New("test: unavailable")
	}
	processDone := make(chan struct{}, 1)
	env.RunInput = func(ctx context.Context, dir, input, name string, args ...string) (string, string, int, error) {
		out, stderr, code, err := runInput(ctx, dir, input, name, args...)
		processDone <- struct{}{}
		return out, stderr, code, err
	}
	client := paritySession(t, root, env)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	callDone := make(chan error, 1)
	go func() {
		_, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "review_run", Arguments: map[string]any{"agent": "claude", "noCheck": true, "noPRComments": true}})
		callDone <- err
	}()
	deadline := time.Now().Add(8 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("nested agent never started")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-callDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("client error=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("client cancellation hung")
	}
	select {
	case <-processDone:
	case <-time.After(5 * time.Second):
		t.Fatal("nested agent process did not terminate")
	}
}
