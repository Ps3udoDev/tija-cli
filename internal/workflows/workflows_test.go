package workflows

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestValidateRejectsShellAndNested(t *testing.T) {
	wShell := Workflow{
		Name:  "test-shell",
		Steps: []Step{{Run: "npm test"}},
	}
	errs := Validate(wShell)
	if len(errs) == 0 {
		t.Fatal("expected validation errors for npm test, got none")
	}
	foundUnknown := false
	for _, err := range errs {
		if strings.Contains(err.Error(), `unknown command "npm"`) {
			foundUnknown = true
			break
		}
	}
	if !foundUnknown {
		t.Errorf("expected error containing 'unknown command \"npm\"', got %v", errs)
	}

	wNested := Workflow{
		Name:  "test-nested",
		Steps: []Step{{Run: "run pre-pr"}},
	}
	errs = Validate(wNested)
	if len(errs) == 0 {
		t.Fatal("expected validation errors for run pre-pr, got none")
	}
	foundNested := false
	for _, err := range errs {
		if strings.Contains(err.Error(), "nested run is not allowed") {
			foundNested = true
			break
		}
	}
	if !foundNested {
		t.Errorf("expected error containing 'nested run is not allowed', got %v", errs)
	}
}

func TestValidateNameAndSteps(t *testing.T) {
	wNoName := Workflow{
		Steps: []Step{{Run: "check"}},
	}
	errs := Validate(wNoName)
	if len(errs) == 0 {
		t.Error("expected error for empty workflow name")
	}

	wNoSteps := Workflow{
		Name:  "no-steps",
		Steps: []Step{},
	}
	errs = Validate(wNoSteps)
	if len(errs) == 0 {
		t.Error("expected error for empty steps")
	}

	wValid := Workflow{
		Name: "valid",
		Steps: []Step{
			{Run: "clean comments --write"},
			{Run: "check"},
			{Run: "pr prepare"},
		},
	}
	if errs := Validate(wValid); len(errs) > 0 {
		t.Errorf("unexpected errors for valid workflow: %v", errs)
	}
}

func TestLoadWorkflows(t *testing.T) {
	dir := t.TempDir()
	wfDir := filepath.Join(dir, ".tija", "workflows")
	if err := os.MkdirAll(wfDir, 0755); err != nil {
		t.Fatal(err)
	}

	// .gitkeep should be ignored
	if err := os.WriteFile(filepath.Join(wfDir, ".gitkeep"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	content := `name: pre-pr
description: Run checks before PR
steps:
  - run: check
  - run: status
    continueOnError: true
`
	if err := os.WriteFile(filepath.Join(wfDir, "pre-pr.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	wfs, errs := Load(dir)
	if len(errs) > 0 {
		t.Fatalf("unexpected load errors: %v", errs)
	}
	if len(wfs) != 1 {
		t.Fatalf("loaded %d workflows, want 1", len(wfs))
	}
	w := wfs[0]
	if w.Name != "pre-pr" {
		t.Errorf("Name = %q, want pre-pr", w.Name)
	}
	if w.Description != "Run checks before PR" {
		t.Errorf("Description = %q", w.Description)
	}
	if len(w.Steps) != 2 {
		t.Fatalf("Steps count = %d, want 2", len(w.Steps))
	}
	if w.Steps[0].Run != "check" || w.Steps[0].ContinueOnError {
		t.Errorf("Step 0 = %+v", w.Steps[0])
	}
	if w.Steps[1].Run != "status" || !w.Steps[1].ContinueOnError {
		t.Errorf("Step 1 = %+v", w.Steps[1])
	}
}

func TestArgs(t *testing.T) {
	s := Step{Run: `clean comments --write "src/my files"`}
	args, err := Args(s)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"clean", "comments", "--write", "src/my files"}
	if !slices.Equal(args, want) {
		t.Errorf("Args = %q, want %q", args, want)
	}
}
