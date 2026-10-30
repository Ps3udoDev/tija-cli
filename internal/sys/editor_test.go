package sys

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	got := SplitArgs(`"C:/Program Files/Code/code.exe" --wait -n`)
	want := []string{"C:/Program Files/Code/code.exe", "--wait", "-n"}
	if !slices.Equal(got, want) {
		t.Errorf("SplitArgs = %q, want %q", got, want)
	}
}

func editorEnv(stdout, stderr string, code int) Env {
	return Env{Run: func(_ context.Context, _, name string, args ...string) (string, string, int, error) {
		if name != "git" || !slices.Equal(args, []string{"var", "GIT_EDITOR"}) {
			return "", "unexpected", 1, nil
		}
		return stdout, stderr, code, nil
	}}
}

func TestEditorCommand(t *testing.T) {
	dir := t.TempDir()
	c, err := EditorCommand(context.Background(), editorEnv("'my editor' --wait\n", "", 0), dir, "pr.md")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"my editor", "--wait", "pr.md"}; !slices.Equal(c.Args, want) {
		t.Errorf("args = %q, want %q", c.Args, want)
	}
	if c.Dir != dir {
		t.Errorf("dir = %q, want %q", c.Dir, dir)
	}
	if c.Process != nil {
		t.Error("command was started")
	}
}

func TestEditorCommandNoEditor(t *testing.T) {
	_, err := EditorCommand(context.Background(), editorEnv("  \n", "", 0), t.TempDir(), "pr.md")
	if err == nil || err.Error() != "no editor configured (set GIT_EDITOR or core.editor)" {
		t.Errorf("err = %v", err)
	}
}

func TestEditorCommandGitFails(t *testing.T) {
	_, err := EditorCommand(context.Background(), editorEnv("", "fatal: boom\n", 128), t.TempDir(), "pr.md")
	if err == nil || !strings.Contains(err.Error(), "git var GIT_EDITOR failed: fatal: boom") {
		t.Errorf("err = %v", err)
	}
}
