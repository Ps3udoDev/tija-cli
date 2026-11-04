package agents

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestArgvReadOnly(t *testing.T) {
	args := strings.Join(Argv("claude", "claude", "balanced", "", true), " ")
	for _, want := range []string{"--allowedTools Read,Grep,Glob", "--disallowedTools Bash,Edit,Write,NotebookEdit,WebFetch,WebSearch"} {
		if !strings.Contains(args, want) {
			t.Fatalf("missing %s in %s", want, args)
		}
	}
	for _, provider := range []string{"codex", "gemini"} {
		if !reflect.DeepEqual(Argv(provider, provider, "deep", "tmp", true), Argv(provider, provider, "deep", "tmp", false)) {
			t.Fatal(provider)
		}
	}
	if strings.Contains(strings.Join(Argv("claude", "claude", "balanced", "", false), " "), "--allowedTools") {
		t.Fatal("restricted normal request")
	}
}
func TestGenerateReadOnly(t *testing.T) {
	var args []string
	env := fakeEnv([]string{"claude"}, func(_ context.Context, _, _, _ string, a ...string) (string, string, int, error) {
		args = a
		return `{"findings":[]}`, "", 0, nil
	})
	if _, err := Generate(context.Background(), env, "claude", Request{Prompt: "review", ReadOnly: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(args, " "), "--allowedTools Read,Grep,Glob") {
		t.Fatal(args)
	}
}
