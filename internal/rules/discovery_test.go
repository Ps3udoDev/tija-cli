package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOverlappingAndMissingGlobs(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, ".tija", "rules", "architecture")
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "a.md"), []byte("---\nname: example\n---\n## example-rule: Example\nBody\n"), 0644); err != nil {
		t.Fatal(err)
	}
	set, errs := Load(root, []string{".tija/rules/architecture/*.md", ".tija/rules/**/*.md", "missing/**/*.md"}, nil)
	if len(errs) > 0 || len(set.Files) != 1 {
		t.Fatalf("%+v %v", set, errs)
	}
}

func TestRuleGlobWalkPrefix(t *testing.T) {
	// A missing root is deliberately unrelated to the absent optional rule
	// directory. Discovery succeeds without walking the repository root.
	root := filepath.Join(t.TempDir(), "missing-root")
	set, errs := Load(root, []string{".tija/rules/architecture/*.md"}, nil)
	if len(errs) > 0 || len(set.Files) != 0 {
		t.Fatalf("%+v %v", set, errs)
	}
}

func TestRuleGlobStaysInsideRepository(t *testing.T) {
	_, errs := Load(t.TempDir(), []string{"../*.md"}, nil)
	if len(errs) == 0 {
		t.Fatal("outside-repository glob accepted")
	}
}
