package detect

import (
	"slices"
	"testing"
)

func TestDetectPRArtifactsFrontend(t *testing.T) {
	d, err := Detect(fixture("react-vite"), "")
	if err != nil {
		t.Fatal(err)
	}
	if d.PRTemplate != ".github/PULL_REQUEST_TEMPLATE.md" {
		t.Errorf("PRTemplate = %q", d.PRTemplate)
	}
	if d.PRGovernanceWorkflow != ".github/workflows/pr-governance.yml" {
		t.Errorf("PRGovernanceWorkflow = %q", d.PRGovernanceWorkflow)
	}
	want := []string{"feature", "feat", "bugfix", "fix", "hotfix", "docs", "refactor", "chore", "release"}
	if !slices.Equal(d.BranchPrefixes, want) {
		t.Errorf("BranchPrefixes = %v", d.BranchPrefixes)
	}
	if !d.HuskyBranchHook {
		t.Error("HuskyBranchHook = false")
	}
	if d.CommitlintHeaderMax != 100 {
		t.Errorf("CommitlintHeaderMax = %d", d.CommitlintHeaderMax)
	}
	for _, tool := range []string{"husky", "commitlint", "biome"} {
		if !slices.Contains(d.Tools, tool) {
			t.Errorf("Tools %v missing %q", d.Tools, tool)
		}
	}
	if !slices.IsSorted(d.Tools) {
		t.Errorf("Tools not sorted: %v", d.Tools)
	}
}

func TestDetectPRGuideBackend(t *testing.T) {
	d, err := Detect(fixture("dotnet-api"), "")
	if err != nil {
		t.Fatal(err)
	}
	if d.PRGuide != "templates/PULL_REQUEST_TEMPLATE_GUIDE.md" {
		t.Errorf("PRGuide = %q", d.PRGuide)
	}
	if d.PRTemplate != "" {
		t.Errorf("PRTemplate = %q, want empty", d.PRTemplate)
	}
}

func TestDetectMultipleTemplates(t *testing.T) {
	d, err := Detect(fixture("multi-template"), "")
	if err != nil {
		t.Fatal(err)
	}
	if d.PRTemplate != ".github/PULL_REQUEST_TEMPLATE/bugfix.md" {
		t.Errorf("PRTemplate = %q", d.PRTemplate)
	}
	if len(d.PRTemplateCandidates) != 2 {
		t.Errorf("candidates = %v", d.PRTemplateCandidates)
	}
}

func TestDetectSingleTemplateCandidates(t *testing.T) {
	d, err := Detect(fixture("react-vite"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(d.PRTemplateCandidates, []string{".github/PULL_REQUEST_TEMPLATE.md"}) {
		t.Errorf("candidates = %v", d.PRTemplateCandidates)
	}
}
