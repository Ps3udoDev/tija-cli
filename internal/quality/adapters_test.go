package quality_test

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
)

func TestBiomeGroupsLintAndFormat(t *testing.T) {
	p := quality.Project{PackageManager: "bun"}
	steps := []quality.Step{
		{Kind: quality.Lint, Entry: manifest.QualityEntry{Provider: "biome"}},
		{Kind: quality.Format, Entry: manifest.QualityEntry{Provider: "biome"}},
	}
	lookPath := func(string) (string, error) { return "/bin/biome", nil }

	// CheckMode
	invs := quality.Plan(p, steps, quality.CheckMode, lookPath)
	if len(invs) != 1 {
		t.Fatalf("Plan len = %d, want 1", len(invs))
	}
	wantKinds := []quality.Kind{quality.Lint, quality.Format}
	if !reflect.DeepEqual(invs[0].Kinds, wantKinds) {
		t.Errorf("Kinds = %v, want %v", invs[0].Kinds, wantKinds)
	}
	wantArgs := []string{"ci", "."}
	if !reflect.DeepEqual(invs[0].Args, wantArgs) {
		t.Errorf("Args = %v, want %v", invs[0].Args, wantArgs)
	}

	// FixMode
	invsFix := quality.Plan(p, steps, quality.FixMode, lookPath)
	if len(invsFix) != 1 {
		t.Fatalf("Plan FixMode len = %d, want 1", len(invsFix))
	}
	wantFixArgs := []string{"check", "--write", "."}
	if !reflect.DeepEqual(invsFix[0].Args, wantFixArgs) {
		t.Errorf("FixMode Args = %v, want %v", invsFix[0].Args, wantFixArgs)
	}
}

func TestDotnetCommands(t *testing.T) {
	lookPath := func(string) (string, error) { return "/bin/dotnet", nil }

	// With Solution
	pSln := quality.Project{Solution: "ClaimsService.sln"}
	buildSteps := []quality.Step{{Kind: quality.Build, Entry: manifest.QualityEntry{Provider: "dotnet"}}}
	invs := quality.Plan(pSln, buildSteps, quality.CheckMode, lookPath)
	if len(invs) != 1 || !reflect.DeepEqual(invs[0].Args, []string{"build", "ClaimsService.sln", "--nologo"}) {
		t.Errorf("dotnet build with sln = %v", invs[0].Args)
	}

	formatSteps := []quality.Step{{Kind: quality.Format, Entry: manifest.QualityEntry{Provider: "dotnet-format"}}}
	invs = quality.Plan(pSln, formatSteps, quality.CheckMode, lookPath)
	if len(invs) != 1 || !reflect.DeepEqual(invs[0].Args, []string{"format", "ClaimsService.sln", "--verify-no-changes"}) {
		t.Errorf("dotnet format with sln = %v", invs[0].Args)
	}

	// Without Solution
	pEmpty := quality.Project{}
	invs = quality.Plan(pEmpty, buildSteps, quality.CheckMode, lookPath)
	if len(invs) != 1 || !reflect.DeepEqual(invs[0].Args, []string{"build", "--nologo"}) {
		t.Errorf("dotnet build without sln = %v", invs[0].Args)
	}

	invs = quality.Plan(pEmpty, formatSteps, quality.CheckMode, lookPath)
	if len(invs) != 1 || !reflect.DeepEqual(invs[0].Args, []string{"format", "--verify-no-changes"}) {
		t.Errorf("dotnet format without sln = %v", invs[0].Args)
	}
}

func TestDotnetTestReportDir(t *testing.T) {
	lookPath := func(string) (string, error) { return "/bin/dotnet", nil }
	tmpDir := "/tmp/quality123"
	p := quality.Project{TmpDir: tmpDir}
	testSteps := []quality.Step{{Kind: quality.Tests, Entry: manifest.QualityEntry{Provider: "dotnet-test"}}}
	invs := quality.Plan(p, testSteps, quality.CheckMode, lookPath)
	if len(invs) != 1 {
		t.Fatalf("Plan len = %d, want 1", len(invs))
	}
	expectedReport := filepath.ToSlash(filepath.Join(tmpDir, "trx"))
	if invs[0].Report != expectedReport {
		t.Errorf("Report = %q, want %q", invs[0].Report, expectedReport)
	}
	// Check that args contain "--logger", "trx", "--results-directory", expectedReport
	idx := slices.Index(invs[0].Args, "--results-directory")
	if idx == -1 || idx+1 >= len(invs[0].Args) || invs[0].Args[idx+1] != expectedReport {
		t.Errorf("Args missing --results-directory %q: %v", expectedReport, invs[0].Args)
	}
}

func TestScriptAdapter(t *testing.T) {
	lookPath := func(string) (string, error) { return "/bin/bun", nil }
	pBun := quality.Project{PackageManager: "bun"}
	steps := []quality.Step{{Kind: quality.Lint, Entry: manifest.QualityEntry{Provider: "script", Script: "lint"}}}
	invs := quality.Plan(pBun, steps, quality.CheckMode, lookPath)
	if len(invs) != 1 || invs[0].Bin != "bun" || !reflect.DeepEqual(invs[0].Args, []string{"run", "lint"}) {
		t.Errorf("bun script = %+v", invs[0])
	}

	pEmpty := quality.Project{}
	lookNpm := func(string) (string, error) { return "/bin/npm", nil }
	invs = quality.Plan(pEmpty, steps, quality.CheckMode, lookNpm)
	if len(invs) != 1 || invs[0].Bin != "npm" || !reflect.DeepEqual(invs[0].Args, []string{"run", "lint"}) {
		t.Errorf("npm script = %+v", invs[0])
	}
}

func TestFixUnsupportedDropped(t *testing.T) {
	lookPath := func(string) (string, error) { return "/bin/dotnet", nil }
	p := quality.Project{}
	steps := []quality.Step{{Kind: quality.Tests, Entry: manifest.QualityEntry{Provider: "dotnet-test"}}}
	invs := quality.Plan(p, steps, quality.FixMode, lookPath)
	if len(invs) != 0 {
		t.Fatalf("expected 0 invocations in FixMode for dotnet-test, got %d", len(invs))
	}
}

func TestDisplayQuotes(t *testing.T) {
	inv := quality.Invocation{
		Bin:  "dotnet",
		Args: []string{"build", "My Solution.sln", "--nologo"},
	}
	got := inv.Display()
	want := `dotnet build "My Solution.sln" --nologo`
	if got != want {
		t.Errorf("Display() = %q, want %q", got, want)
	}
}
