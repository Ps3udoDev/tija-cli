package engine

import (
	"reflect"
	"strings"
	"testing"
)

func TestGitignoreAppends(t *testing.T) {
	out, added := mergeGitignore([]byte("node_modules\n"), gitignoreEntries)
	want := "node_modules\n# tija (local)\n.tija/cache/\n.tija/index/\n.tija/session/\n.tija/tmp/\n"
	if string(out) != want {
		t.Errorf("out = %q, want %q", out, want)
	}
	if !reflect.DeepEqual(added, gitignoreEntries) {
		t.Errorf("added = %v", added)
	}
}

func TestGitignoreCRLFPreserved(t *testing.T) {
	out, added := mergeGitignore([]byte("dist\r\n"), gitignoreEntries)
	if len(added) != 4 {
		t.Fatalf("added = %v", added)
	}
	rest := strings.TrimPrefix(string(out), "dist\r\n")
	if !strings.HasSuffix(rest, "\r\n") || strings.Count(rest, "\n") != strings.Count(rest, "\r\n") {
		t.Errorf("out = %q, want only CRLF line endings", out)
	}
	if strings.Count(rest, "\r\n") != 5 {
		t.Errorf("out = %q, want header + 4 entries", out)
	}
}

func TestGitignoreIdempotent(t *testing.T) {
	first, _ := mergeGitignore([]byte("node_modules\n"), gitignoreEntries)
	second, added := mergeGitignore(first, gitignoreEntries)
	if len(added) != 0 || string(second) != string(first) {
		t.Errorf("second call added %v, out %q", added, second)
	}
}

func TestGitignoreNoTrailingNewline(t *testing.T) {
	out, _ := mergeGitignore([]byte("dist"), gitignoreEntries)
	want := "dist\n# tija (local)\n.tija/cache/\n.tija/index/\n.tija/session/\n.tija/tmp/\n"
	if string(out) != want {
		t.Errorf("out = %q, want %q", out, want)
	}
}

func TestGitignorePartial(t *testing.T) {
	out, added := mergeGitignore([]byte("/.tija/cache/\n.tija/tmp/\n"), gitignoreEntries)
	if !reflect.DeepEqual(added, []string{".tija/index/", ".tija/session/"}) {
		t.Errorf("added = %v", added)
	}
	if strings.Count(string(out), ".tija/cache/") != 1 {
		t.Errorf("duplicated entry: %q", out)
	}
	if _, added := mergeGitignore(nil, gitignoreEntries); len(added) != 4 {
		t.Errorf("empty file added = %v", added)
	}
}
