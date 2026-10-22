package quality_test

import (
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/quality"
)

func TestSessionMergeByKind(t *testing.T) {
	root := t.TempDir()

	// 1. Save [lint passed]
	lintStep := quality.StepResult{
		Kinds:     []quality.Kind{quality.Lint},
		Provider:  "biome",
		Command:   "biome lint .",
		Status:    "passed",
		ExitCode:  0,
		StartedAt: time.Now(),
		Commit:    "abc1234",
	}
	if err := quality.SaveSession(root, []quality.StepResult{lintStep}); err != nil {
		t.Fatalf("SaveSession 1: %v", err)
	}

	// 2. Save [tests failed]
	testStep := quality.StepResult{
		Kinds:     []quality.Kind{quality.Tests},
		Provider:  "dotnet-test",
		Command:   "dotnet test",
		Status:    "failed",
		ExitCode:  1,
		StartedAt: time.Now(),
		Commit:    "abc1234",
		Tests:     &quality.TestCounts{Total: 10, Passed: 9, Failed: 1},
	}
	if err := quality.SaveSession(root, []quality.StepResult{testStep}); err != nil {
		t.Fatalf("SaveSession 2: %v", err)
	}

	// 3. Load has both lint and tests
	session, err := quality.LoadSession(root)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if len(session.Results) != 2 {
		t.Fatalf("expected 2 results, got %d: %+v", len(session.Results), session.Results)
	}
	if session.Results[quality.Lint].Status != "passed" {
		t.Errorf("lint status = %q, want passed", session.Results[quality.Lint].Status)
	}
	if session.Results[quality.Tests].Status != "failed" {
		t.Errorf("tests status = %q, want failed", session.Results[quality.Tests].Status)
	}

	// 4. Overwrite lint
	lintStep2 := lintStep
	lintStep2.Status = "failed"
	lintStep2.ExitCode = 1
	if err := quality.SaveSession(root, []quality.StepResult{lintStep2}); err != nil {
		t.Fatalf("SaveSession 3: %v", err)
	}

	session2, err := quality.LoadSession(root)
	if err != nil {
		t.Fatalf("LoadSession 2: %v", err)
	}
	if session2.Results[quality.Lint].Status != "failed" {
		t.Errorf("updated lint status = %q, want failed", session2.Results[quality.Lint].Status)
	}
	if session2.Results[quality.Tests].Status != "failed" {
		t.Errorf("tests status = %q, want failed", session2.Results[quality.Tests].Status)
	}
}

func TestLoadSessionMissing(t *testing.T) {
	root := t.TempDir()
	session, err := quality.LoadSession(root)
	if err != nil {
		t.Fatalf("LoadSession missing file err: %v", err)
	}
	if session.Version != 1 {
		t.Errorf("session Version = %d, want 1", session.Version)
	}
	if session.Results == nil || len(session.Results) != 0 {
		t.Errorf("session Results = %v, want empty map", session.Results)
	}
}
