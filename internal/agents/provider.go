package agents

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

var (
	// ErrNotInstalled means the provider's CLI was not found on PATH.
	ErrNotInstalled = errors.New("agent CLI is not installed")
	// ErrTimeout means the agent did not answer before Request.Timeout.
	ErrTimeout = errors.New("agent timed out")
)

// Request is one prompt for an agent CLI.
type Request struct {
	Prompt  string        // written to the agent's stdin
	Tier    string        // fast | balanced | deep; "" keeps the CLI default
	Dir     string        // working directory of the process
	TmpDir  string        // where the agent may write temporary files
	Timeout time.Duration // 0 means no timeout
}

// geminiPrompt is the positional prompt for gemini/agy print mode; the real
// instructions arrive on stdin.
const geminiPrompt = "Follow the instructions given on standard input."

// codexLastFile is the file codex writes its last message to (-o).
const codexLastFile = "codex-last.txt"

var claudeModels = map[string]string{"fast": "haiku", "balanced": "sonnet", "deep": "opus"}

// Argv returns the arguments (without the binary) that run provider
// non-interactively with the prompt on stdin. bin is the resolved binary;
// gemini and agy (Antigravity) currently take the same arguments.
func Argv(provider, bin, tier, tmpDir string) []string {
	switch provider {
	case "claude":
		args := []string{"-p", "--output-format", "text"}
		if m, ok := claudeModels[tier]; ok {
			args = append(args, "--model", m)
		}
		return args
	case "codex":
		return []string{"exec", "--sandbox", "read-only", "--skip-git-repo-check", "--color", "never",
			"-o", filepath.Join(tmpDir, codexLastFile), "-"}
	case "gemini":
		return []string{"-p", geminiPrompt}
	}
	return nil
}

// Generate runs provider's CLI with req.Prompt on stdin and returns its
// answer. It never adds flags that skip permissions or sandboxes.
func Generate(ctx context.Context, env sys.Env, provider string, req Request) (string, error) {
	bin := Binary(provider, env.LookPath)
	if bin == "" {
		return "", fmt.Errorf("%s: %w", provider, ErrNotInstalled)
	}

	tmpDir := req.TmpDir
	if provider == "codex" {
		if tmpDir == "" {
			d, err := os.MkdirTemp("", "tija-agent-")
			if err != nil {
				return "", err
			}
			defer os.RemoveAll(d)
			tmpDir = d
		} else if err := os.MkdirAll(tmpDir, 0o755); err != nil {
			return "", err
		}
		_ = os.Remove(filepath.Join(tmpDir, codexLastFile)) // never read a stale answer
	}

	runCtx := ctx
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	stdout, stderr, code, err := env.RunInput(runCtx, req.Dir, req.Prompt, bin, Argv(provider, bin, req.Tier, tmpDir)...)
	if runCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("%s: %w after %s", provider, ErrTimeout, req.Timeout)
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", provider, err)
	}
	if code != 0 {
		return "", fmt.Errorf("%s exited %d: %s", provider, code, lastLines(stderr, 5))
	}

	out := stdout
	if provider == "codex" {
		if b, err := os.ReadFile(filepath.Join(tmpDir, codexLastFile)); err == nil {
			out = string(b)
		}
	}
	if strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("%s returned no output", provider)
	}
	return out, nil
}

// lastLines returns the last n non-trailing lines of s, without ANSI sequences.
func lastLines(s string, n int) string {
	s = ansiRx.ReplaceAllString(s, "")
	s = strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), " \t\n")
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
