package agents

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

func TestArgv(t *testing.T) {
	tmp := filepath.Join("x", "tmp")
	geminiArgs := []string{"-p", "Follow the instructions given on standard input."}
	cases := []struct {
		provider, bin, tier string
		want                []string
	}{
		{"claude", "/bin/claude", "fast", []string{"-p", "--output-format", "text", "--model", "haiku"}},
		{"claude", "/bin/claude", "balanced", []string{"-p", "--output-format", "text", "--model", "sonnet"}},
		{"claude", "/bin/claude", "deep", []string{"-p", "--output-format", "text", "--model", "opus"}},
		{"claude", "/bin/claude", "", []string{"-p", "--output-format", "text"}},
		{"codex", "/bin/codex", "deep", []string{"exec", "--sandbox", "read-only", "--skip-git-repo-check", "--color", "never", "-o", filepath.Join(tmp, "codex-last.txt"), "-"}},
		{"gemini", "/bin/gemini", "fast", geminiArgs},
		{"gemini", `C:\agy\bin\agy.exe`, "deep", geminiArgs},
	}
	for _, c := range cases {
		got := Argv(c.provider, c.bin, c.tier, tmp)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Argv(%s, %s, %q) = %q, want %q", c.provider, c.bin, c.tier, got, c.want)
		}
	}
}

type call struct {
	dir, stdin, name string
	args             []string
}

func fakeEnv(found []string, run sys.InputRunner) sys.Env {
	return sys.Env{LookPath: fakeLookPath(found...), RunInput: run, Getenv: func(string) string { return "" }}
}

func TestGenerateUsesStdin(t *testing.T) {
	var got call
	env := fakeEnv([]string{"claude"}, func(ctx context.Context, dir, stdin, name string, args ...string) (string, string, int, error) {
		got = call{dir, stdin, name, args}
		return "<<<TITLE\nfeat(x): y\nTITLE>>>", "", 0, nil
	})
	out, err := Generate(context.Background(), env, "claude", Request{Prompt: "the prompt", Tier: "fast", Dir: "repo", TmpDir: t.TempDir(), Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "<<<TITLE") {
		t.Errorf("out = %q", out)
	}
	if got.stdin != "the prompt" || got.name != "/bin/claude" || got.dir != "repo" {
		t.Errorf("call = %+v", got)
	}
	if !reflect.DeepEqual(got.args, Argv("claude", "/bin/claude", "fast", "")) {
		t.Errorf("args = %q", got.args)
	}
}

func TestGenerateNotInstalled(t *testing.T) {
	env := fakeEnv(nil, func(context.Context, string, string, string, ...string) (string, string, int, error) {
		t.Fatal("must not run")
		return "", "", 0, nil
	})
	_, err := Generate(context.Background(), env, "codex", Request{Prompt: "p"})
	if !errors.Is(err, ErrNotInstalled) {
		t.Errorf("err = %v, want ErrNotInstalled", err)
	}
}

func TestGenerateTimeout(t *testing.T) {
	env := fakeEnv([]string{"claude"}, func(ctx context.Context, _, _, _ string, _ ...string) (string, string, int, error) {
		<-ctx.Done()
		return "", "", -1, ctx.Err()
	})
	start := time.Now()
	_, err := Generate(context.Background(), env, "claude", Request{Prompt: "p", Timeout: 50 * time.Millisecond})
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("err = %v, want ErrTimeout", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("took %v", d)
	}
}

func TestGenerateNonZeroExit(t *testing.T) {
	env := fakeEnv([]string{"claude"}, func(context.Context, string, string, string, ...string) (string, string, int, error) {
		return "", "l1\nl2\nl3\nl4\nl5\nl6\nl7\n", 3, nil
	})
	_, err := Generate(context.Background(), env, "claude", Request{Prompt: "p"})
	if err == nil || !strings.Contains(err.Error(), "claude exited 3: l3\nl4\nl5\nl6\nl7") || strings.Contains(err.Error(), "l2") {
		t.Errorf("err = %v", err)
	}
}

func TestGenerateEmptyOutput(t *testing.T) {
	env := fakeEnv([]string{"claude"}, func(context.Context, string, string, string, ...string) (string, string, int, error) {
		return "  \n", "", 0, nil
	})
	if _, err := Generate(context.Background(), env, "claude", Request{Prompt: "p"}); err == nil {
		t.Error("want error for empty output")
	}
}

func TestGenerateCodexReadsLastMessage(t *testing.T) {
	tmp := t.TempDir()
	env := fakeEnv([]string{"codex"}, func(_ context.Context, _, _, _ string, args ...string) (string, string, int, error) {
		if err := os.WriteFile(filepath.Join(tmp, "codex-last.txt"), []byte("last message"), 0o644); err != nil {
			t.Fatal(err)
		}
		return "noisy progress output", "", 0, nil
	})
	out, err := Generate(context.Background(), env, "codex", Request{Prompt: "p", TmpDir: tmp})
	if err != nil {
		t.Fatal(err)
	}
	if out != "last message" {
		t.Errorf("out = %q", out)
	}
}

func TestGenerateCodexFallsBackToStdout(t *testing.T) {
	env := fakeEnv([]string{"codex"}, func(context.Context, string, string, string, ...string) (string, string, int, error) {
		return "from stdout", "", 0, nil
	})
	out, err := Generate(context.Background(), env, "codex", Request{Prompt: "p", TmpDir: t.TempDir()})
	if err != nil || out != "from stdout" {
		t.Errorf("out = %q, err = %v", out, err)
	}
}

func TestLastLinesStripsANSI(t *testing.T) {
	got := lastLines("one\n\x1b[31merror:\x1b[0m bad flag\r\n\x1b[2Kdone\n", 2)
	if got != "error: bad flag\ndone" {
		t.Errorf("lastLines = %q", got)
	}
}
