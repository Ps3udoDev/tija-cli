package agents

import (
	"errors"
	"reflect"
	"testing"
)

func fakeLookPath(found ...string) func(string) (string, error) {
	return func(name string) (string, error) {
		for _, f := range found {
			if f == name {
				return "/bin/" + name, nil
			}
		}
		return "", errors.New("not found")
	}
}

func TestAvailableOrder(t *testing.T) {
	got := Available(fakeLookPath("gemini", "codex", "claude"))
	if !reflect.DeepEqual(got, []string{"claude", "codex", "gemini"}) {
		t.Errorf("Available = %v", got)
	}
	if got := Available(fakeLookPath("git")); len(got) != 0 {
		t.Errorf("Available = %v, want none", got)
	}
}

func TestBinaryGeminiPrefersAntigravity(t *testing.T) {
	if got := Binary("gemini", fakeLookPath("gemini", "agy")); got != "/bin/agy" {
		t.Errorf("Binary(gemini) = %q, want agy", got)
	}
	if got := Binary("gemini", fakeLookPath("gemini")); got != "/bin/gemini" {
		t.Errorf("Binary(gemini) = %q, want gemini fallback", got)
	}
	if got := Binary("claude", fakeLookPath()); got != "" {
		t.Errorf("Binary(claude) = %q, want empty", got)
	}
	if got := Binary("gpt", fakeLookPath("gpt")); got != "" {
		t.Errorf("Binary(unknown) = %q, want empty", got)
	}
}
