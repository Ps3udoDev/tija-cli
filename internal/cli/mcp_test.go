package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func TestServeOutsideRepoExits2(t *testing.T) {
	outside := t.TempDir()
	for _, args := range [][]string{
		{"mcp", "serve", "--dir", outside},
		{"mcp", "serve", "--dir", outside, "--json"},
		{"--json", "mcp", "serve", "--dir", outside},
		{"mcp", "serve", "--json=true", "--dir", outside},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := Execute(args, strings.NewReader(""), &out, &errOut)
			if code != 2 || out.Len() != 0 || !strings.Contains(errOut.String(), "is not a git repository") {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
			}
		})
	}
}

func TestServeInvalidLogLevelExits2(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Execute([]string{"mcp", "serve", "--log-level", "trace"}, strings.NewReader(""), &out, &errOut)
	if code != 2 || out.Len() != 0 || !strings.Contains(errOut.String(), "invalid log level") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

// This deliberately launches the production entry point: injected writers alone
// cannot detect stray output from dependencies or child processes.
func TestServeStdoutIsJSONRPCOnly(t *testing.T) {
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	name := "tija"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer buildCancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", bin, "./cmd/tija")
	build.Dir = moduleRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	root := testutil.NewRepo(t, "")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "mcp", "serve", "--dir", root)
	// Desktop clients can start in a directory unrelated to the repository.
	cmd.Dir = t.TempDir()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	for _, request := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"stdio-test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	} {
		if _, err := fmt.Fprintln(stdin, request); err != nil {
			t.Fatal(err)
		}
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	seen := map[int]bool{}
	closed := false
	for scanner.Scan() {
		var message struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      int             `json:"id"`
			Result  json.RawMessage `json:"result"`
			Error   json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil || message.JSONRPC != "2.0" {
			t.Fatalf("non-JSON-RPC stdout line %q: %v", scanner.Text(), err)
		}
		if message.ID == 1 || message.ID == 2 {
			if len(message.Error) > 0 || len(message.Result) == 0 {
				t.Fatalf("request %d failed: %s", message.ID, scanner.Text())
			}
			if message.ID == 1 {
				var init struct {
					ProtocolVersion string `json:"protocolVersion"`
				}
				if err := json.Unmarshal(message.Result, &init); err != nil || init.ProtocolVersion != "2025-06-18" {
					t.Fatalf("protocol negotiation: %s (%v)", message.Result, err)
				}
			}
			seen[message.ID] = true
		}
		if seen[1] && seen[2] && !closed {
			_ = stdin.Close()
			closed = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("serve: %v (context=%v), stderr=%s", err, ctx.Err(), stderr.String())
	}
	if !seen[1] || !seen[2] {
		t.Fatalf("missing responses: %v; stderr=%s", seen, stderr.String())
	}
}
