package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

func run(t *testing.T, deps Deps, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := ExecuteWith(deps, args, strings.NewReader(""), &out, &errOut)
	return code, out.String(), errOut.String()
}

func baseDeps(isTTY bool, calls *[]string) Deps {
	return Deps{
		Env:   sys.Default(),
		IsTTY: func(any) bool { return isTTY },
		RunTUI: func(dir string) error {
			*calls = append(*calls, dir)
			return nil
		},
	}
}

func TestVersionCommand(t *testing.T) {
	var calls []string
	code, out, _ := run(t, baseDeps(false, &calls), "version")
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !strings.Contains(out, "tija dev") {
		t.Fatalf("out = %q, want it to contain %q", out, "tija dev")
	}
}

func TestUnknownCommandExit2(t *testing.T) {
	var calls []string
	code, _, errOut := run(t, baseDeps(false, &calls), "nope")
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.HasPrefix(errOut, "✗ ") {
		t.Fatalf("errOut = %q, want prefix %q", errOut, "✗ ")
	}
}

func TestNoArgsNonTTYPrintsHelp(t *testing.T) {
	var calls []string
	code, out, _ := run(t, baseDeps(false, &calls))
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("out = %q, want help", out)
	}
	if len(calls) != 0 {
		t.Fatalf("RunTUI called %d times, want 0", len(calls))
	}
}

func TestNoArgsTTYRunsTUI(t *testing.T) {
	var calls []string
	code, _, _ := run(t, baseDeps(true, &calls))
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	cwd, _ := os.Getwd()
	if len(calls) != 1 || calls[0] != cwd {
		t.Fatalf("RunTUI calls = %v, want [%s]", calls, cwd)
	}
}

func TestNoArgsJSONDoesNotRunTUI(t *testing.T) {
	var calls []string
	code, _, _ := run(t, baseDeps(true, &calls), "--json")
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if len(calls) != 0 {
		t.Fatalf("RunTUI called with --json")
	}
}
