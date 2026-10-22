package detect

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func fixture(name string) string { return filepath.Join("testdata", name) }

func TestDetectDotnet(t *testing.T) {
	d, err := Detect(fixture("dotnet-api"), "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Stack != "dotnet-api" || d.Language != "C#" || d.Framework != ".NET 8" {
		t.Errorf("stack/lang/framework = %q %q %q", d.Stack, d.Language, d.Framework)
	}
	if d.ProjectName != "dotnet-api" {
		t.Errorf("ProjectName = %q", d.ProjectName)
	}
	if d.Solution != "ClaimsService.sln" {
		t.Errorf("Solution = %q", d.Solution)
	}
	want := []string{"src/Claims.API/Claims.API.csproj", "tests/Claims.Tests/Claims.Tests.csproj"}
	if !slices.Equal(d.Projects, want) {
		t.Errorf("Projects = %v", d.Projects)
	}
	if d.PackageManager != "dotnet" {
		t.Errorf("PackageManager = %q", d.PackageManager)
	}
	for _, tool := range []string{"dotnet-format", "xunit"} {
		if !slices.Contains(d.Tools, tool) {
			t.Errorf("Tools %v missing %q", d.Tools, tool)
		}
	}
	for _, p := range d.Projects {
		if strings.Contains(p, "scratch") {
			t.Errorf("scratch project leaked: %s", p)
		}
	}
	if d.TestFramework != "xunit" || d.TestCount != 3 {
		t.Errorf("TestFramework/Count = %q %d", d.TestFramework, d.TestCount)
	}
}

func TestDetectReactVite(t *testing.T) {
	d, err := Detect(fixture("react-vite"), "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Stack != "react-vite" || d.Framework != "React 19 + Vite 7" || d.Language != "JavaScript" {
		t.Errorf("stack/framework/lang = %q %q %q", d.Stack, d.Framework, d.Language)
	}
	if d.PackageManager != "bun" {
		t.Errorf("PackageManager = %q", d.PackageManager)
	}
	if !slices.Equal(d.Lockfiles, []string{"bun.lock", "package-lock.json"}) {
		t.Errorf("Lockfiles = %v", d.Lockfiles)
	}
	if !slices.Contains(d.Tools, "biome") {
		t.Errorf("Tools = %v", d.Tools)
	}
	if d.TestCount != 0 {
		t.Errorf("TestCount = %d", d.TestCount)
	}
}

func TestDetectMultiSln(t *testing.T) {
	d, err := Detect(fixture("multi-sln"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(d.Solutions, []string{"A.sln", "B.sln"}) || d.Solution != "" {
		t.Errorf("Solutions=%v Solution=%q", d.Solutions, d.Solution)
	}
	d2, err := d.WithSolution("B.sln")
	if err != nil {
		t.Fatal(err)
	}
	if d2.Solution != "B.sln" {
		t.Errorf("Solution = %q", d2.Solution)
	}
	if _, err := d.WithSolution("C.sln"); err == nil {
		t.Error("expected error for unknown solution")
	}
}

func TestDetectUnknown(t *testing.T) {
	d, err := Detect(fixture("unknown"), "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Stack != "unknown" {
		t.Errorf("Stack = %q", d.Stack)
	}
}

func TestGitHost(t *testing.T) {
	d, _ := Detect(fixture("unknown"), "https://github.com/TEAMS4SOFT/GMX-frontend")
	if d.GitHost != "github" {
		t.Errorf("GitHost = %q", d.GitHost)
	}
	d, _ = Detect(fixture("unknown"), "")
	if d.GitHost != "" {
		t.Errorf("GitHost = %q", d.GitHost)
	}
}

func TestDetectTypeScriptAndCI(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", `{"dependencies":{"react":"~18.3.0"},"devDependencies":{"vite":"5.4.1","vitest":"^2.0.0","eslint":"^9"}}`)
	write("src/main.tsx", "")
	write("src/a.test.ts", "")
	write("node_modules/x/b.test.js", "")
	write(".github/workflows/ci.yml", "")
	write(".github/workflows/deploy.yaml", "")
	write("pnpm-lock.yaml", "")
	d, err := Detect(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Language != "TypeScript" || d.Framework != "React 18 + Vite 5" || d.PackageManager != "pnpm" {
		t.Errorf("got %q %q %q", d.Language, d.Framework, d.PackageManager)
	}
	if d.TestCount != 1 {
		t.Errorf("TestCount = %d", d.TestCount)
	}
	if !slices.Equal(d.CI, []string{".github/workflows/ci.yml", ".github/workflows/deploy.yaml"}) {
		t.Errorf("CI = %v", d.CI)
	}
	if !slices.Equal(d.Tools, []string{"eslint", "vitest"}) {
		t.Errorf("Tools = %v", d.Tools)
	}
}
