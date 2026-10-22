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
	if d.TestFiles != 1 || d.TestCount != 0 {
		t.Errorf("TestFiles/TestCount = %d %d", d.TestFiles, d.TestCount)
	}
	if !slices.Equal(d.CI, []string{".github/workflows/ci.yml", ".github/workflows/deploy.yaml"}) {
		t.Errorf("CI = %v", d.CI)
	}
	if !slices.Equal(d.Tools, []string{"eslint", "vitest"}) {
		t.Errorf("Tools = %v", d.Tools)
	}
}

func TestDetectDotnetSlnx(t *testing.T) {
	d, err := Detect(fixture("dotnet-slnx"), "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Stack != StackDotnetAPI || d.Solution != "App.slnx" || d.Framework != ".NET 10" {
		t.Errorf("stack/solution/framework = %q %q %q", d.Stack, d.Solution, d.Framework)
	}
	want := []string{"src/App.Api/App.Api.csproj", "tests/App.Tests/App.Tests.csproj"}
	if !slices.Equal(d.Projects, want) {
		t.Errorf("Projects = %v", d.Projects)
	}
	if d.TestFramework != "xunit" || d.TestCount != 1 {
		t.Errorf("TestFramework/Count = %q %d", d.TestFramework, d.TestCount)
	}
}

func TestDetectDotnetRootCsproj(t *testing.T) {
	d, err := Detect(fixture("dotnet-csproj"), "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Stack != StackDotnetAPI || d.Language != "C#" || d.PackageManager != "dotnet" || d.Framework != ".NET 9" {
		t.Errorf("stack/lang/pm/framework = %q %q %q %q", d.Stack, d.Language, d.PackageManager, d.Framework)
	}
	if d.Solution != "" || len(d.Solutions) != 0 {
		t.Errorf("Solution = %q, Solutions = %v", d.Solution, d.Solutions)
	}
	if !slices.Equal(d.Projects, []string{"Api.csproj"}) {
		t.Errorf("Projects = %v", d.Projects)
	}
	if !slices.Contains(d.Tools, "dotnet-format") {
		t.Errorf("Tools = %v", d.Tools)
	}
}

func TestDotnetTestPackages(t *testing.T) {
	for pkg, want := range map[string]string{
		"xunit": "xunit", "xunit.v3": "xunit", "NUnit": "nunit",
		"MSTest.TestFramework": "mstest", "MSTest": "mstest", "Moq": "",
	} {
		csproj := `<ItemGroup><PackageReference Include="` + pkg + `" Version="1.0.0" /></ItemGroup>`
		if got := testFramework(csproj); got != want {
			t.Errorf("testFramework(%s) = %q, want %q", pkg, got, want)
		}
	}
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDetectNodeVersionRangesBOMAndSkipDirs(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"package.json":             "\ufeff" + `{"dependencies":{"react":"latest"},"devDependencies":{"vite":"catalog:"}}`,
		"src/a.test.js":            "",
		".next/server/b.test.js":   "",
		"coverage/c.test.js":       "",
		"build/d.test.js":          "",
		"out/e.test.js":            "",
		"node_modules/x/f.test.js": "",
	})
	d, err := Detect(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Stack != StackReactVite || d.Framework != "React + Vite" {
		t.Errorf("stack/framework = %q %q", d.Stack, d.Framework)
	}
	if d.TestFiles != 1 {
		t.Errorf("TestFiles = %d, want 1 (skip .next, coverage, build, out)", d.TestFiles)
	}
}

func TestMajor(t *testing.T) {
	for in, want := range map[string]string{
		"^19.1.0": "19", "~7.0.4": "7", ">=18": "18", "19": "19", "v7.1.0": "7",
		"latest": "", "catalog:": "", "workspace:*": "", "npm:react@19": "", "*": "",
	} {
		if got := major(in); got != want {
			t.Errorf("major(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDetectNodeScripts(t *testing.T) {
	dir := t.TempDir()
	pkgJSON := `{
		"name": "foo",
		"scripts": {
			"test": "vitest",
			"lint": "eslint",
			"build": "vite build"
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := Detect(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"build", "lint", "test"}
	if !slices.Equal(d.Scripts, want) {
		t.Fatalf("Scripts = %v, want %v", d.Scripts, want)
	}

	// Without scripts
	dirNoScripts := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirNoScripts, "package.json"), []byte(`{"name":"bar"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	dNo, err := Detect(dirNoScripts, "")
	if err != nil {
		t.Fatal(err)
	}
	if dNo.Scripts != nil {
		t.Fatalf("expected nil Scripts, got %v", dNo.Scripts)
	}
}
