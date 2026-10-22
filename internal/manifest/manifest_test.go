package manifest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/detect"
)

func frontendDetection(t *testing.T) detect.Detection {
	t.Helper()
	d, err := detect.Detect(filepath.Join("..", "detect", "testdata", "react-vite"), "https://github.com/acme/gmx-frontend.git")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestFromDetectionFrontend(t *testing.T) {
	m := FromDetection(frontendDetection(t), []string{"codex", "gemini"}, "dev")

	if m.Version != 1 || m.Project.Type != "react-vite" || m.Content.Language != "es" {
		t.Fatalf("header = %+v %+v %+v", m.Version, m.Project, m.Content)
	}
	if m.Git.Host != "github" || m.Git.BranchConvention != "conventional" {
		t.Errorf("git = %+v", m.Git)
	}
	wantPrefixes := []string{"feature", "feat", "bugfix", "fix", "hotfix", "docs", "refactor", "chore", "release"}
	if !reflect.DeepEqual(m.Git.BranchPrefixes, wantPrefixes) {
		t.Errorf("BranchPrefixes = %v, want %v", m.Git.BranchPrefixes, wantPrefixes)
	}
	pr := m.PullRequest
	if pr.Target != "dev" || pr.Template != ".github/PULL_REQUEST_TEMPLATE.md" || pr.Rules != ".tija/rules/pull-request.md" {
		t.Errorf("pullRequest paths = %q %q %q", pr.Target, pr.Template, pr.Rules)
	}
	if pr.Title.Convention != "conventional-commits" || pr.Title.RequireScope == nil || !*pr.Title.RequireScope || pr.Title.MaxLength != 100 {
		t.Errorf("title = %+v", pr.Title)
	}
	if !reflect.DeepEqual(pr.RequiredSections, []string{"Resumen", "Objetivo", "Cambios realizados", "Pruebas"}) {
		t.Errorf("RequiredSections = %v", pr.RequiredSections)
	}
	if pr.MinSectionLength != 10 || pr.AutoAssign == nil || !*pr.AutoAssign {
		t.Errorf("minSectionLength/autoAssign = %d %v", pr.MinSectionLength, pr.AutoAssign)
	}
	wantInvalid := []string{"n/a", "na", "no aplica", "pendiente", "todo", "por definir", "...", "-"}
	if !reflect.DeepEqual(pr.InvalidValues, wantInvalid) {
		t.Errorf("InvalidValues = %v", pr.InvalidValues)
	}
	if m.Quality["lint"].Provider != "biome" || m.Quality["format"].Provider != "biome" {
		t.Errorf("Quality = %v", m.Quality)
	}
	if m.Agents.Default != "codex" {
		t.Errorf("Agents.Default = %q", m.Agents.Default)
	}
	wantTasks := map[string]Task{"pullRequest": {Tier: "fast"}, "review": {Tier: "balanced"}, "architecture": {Tier: "deep"}}
	if !reflect.DeepEqual(m.Agents.Tasks, wantTasks) {
		t.Errorf("Tasks = %v", m.Agents.Tasks)
	}
}

func TestFromDetectionDefaults(t *testing.T) {
	d := detect.Detection{ProjectName: "x", Stack: detect.StackDotnetAPI, Tools: []string{"dotnet-format", "xunit"}}
	m := FromDetection(d, nil, "main")
	if m.PullRequest.Template != ".tija/templates/pull-request.md" || m.PullRequest.Title.MaxLength != 100 {
		t.Errorf("defaults = %+v", m.PullRequest)
	}
	if len(m.Git.BranchPrefixes) == 0 || m.Agents.Default != "" {
		t.Errorf("prefixes/default = %v %q", m.Git.BranchPrefixes, m.Agents.Default)
	}
	want := map[string]QualityEntry{"format": {"dotnet-format"}, "build": {"dotnet"}, "tests": {"dotnet-test"}}
	if !reflect.DeepEqual(m.Quality, want) {
		t.Errorf("Quality = %v", m.Quality)
	}
}

func TestManifestRoundTrip(t *testing.T) {
	root := t.TempDir()
	m := FromDetection(frontendDetection(t), []string{"claude"}, "main")
	m.Project.Solution = "A.sln"
	m.Tija = Meta{DefaultsVersion: "1", Installed: map[string]string{".tija/rules/pull-request.md": "abc"}}
	if err := Save(root, m); err != nil {
		t.Fatal(err)
	}
	got, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, m) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, m)
	}
	if errs := got.Validate(); errs != nil {
		t.Errorf("Validate() = %v", errs)
	}
}

func TestLoadErrors(t *testing.T) {
	root := t.TempDir()
	if _, err := Load(root); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing manifest err = %v, want fs.ErrNotExist", err)
	}
	p := filepath.Join(root, filepath.FromSlash(Path))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("version: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "manifest.yaml") {
		t.Errorf("unparsable manifest err = %v", err)
	}

	// Decoding is lenient: unknown keys are allowed; schema problems are
	// reported by Validate.
	if err := os.WriteFile(p, []byte(":::\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(root)
	if err != nil || len(m.Validate()) == 0 {
		t.Errorf("':::' -> err %v, Validate %v; want parse ok and Validate errors", err, m.Validate())
	}
	valid := FromDetection(frontendDetection(t), nil, "main")
	if err := Save(root, valid); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	if err := os.WriteFile(p, append(data, []byte("myTeamKey: hello\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if m, err := Load(root); err != nil || m.Validate() != nil {
		t.Errorf("user key rejected: %v %v", err, m.Validate())
	}
}

func TestPatchKeepsLayout(t *testing.T) {
	data := []byte("# mine\nversion: 1\n\nproject:\n    name: x\n    type: unknown\n\npullRequest:\n    target: dev\n    title:\n        convention: conventional-commits\n")
	fresh := FromDetection(frontendDetection(t), nil, "main")
	out, err := FillMissing(data, fresh, []string{"pullRequest.title.maxLength"})
	if err != nil {
		t.Fatal(err)
	}
	want := string(data) + "        maxLength: 100\n"
	if string(out) != want {
		t.Errorf("FillMissing =\n%s\nwant\n%s", out, want)
	}
	crlf := []byte(strings.ReplaceAll(string(data), "\n", "\r\n"))
	out, err = FillMissing(crlf, fresh, []string{"pullRequest.title.maxLength"})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != strings.ReplaceAll(want, "\n", "\r\n") {
		t.Errorf("CRLF FillMissing = %q", out)
	}
}

func TestValidate(t *testing.T) {
	base := FromDetection(frontendDetection(t), nil, "main")
	if errs := base.Validate(); errs != nil {
		t.Fatalf("base Validate() = %v", errs)
	}
	cases := map[string]func(m *Manifest){
		"version":   func(m *Manifest) { m.Version = 2 },
		"tier":      func(m *Manifest) { m.Agents.Tasks = map[string]Task{"review": {Tier: "ultra"}} },
		"target":    func(m *Manifest) { m.PullRequest.Target = "" },
		"agent":     func(m *Manifest) { m.Agents.Default = "gpt" },
		"stackType": func(m *Manifest) { m.Project.Type = "rails" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := FromDetection(frontendDetection(t), nil, "main")
			mutate(&m)
			if errs := m.Validate(); len(errs) != 1 {
				t.Errorf("Validate() = %v, want exactly 1 error", errs)
			}
		})
	}
}

func TestMissingFields(t *testing.T) {
	fresh := FromDetection(frontendDetection(t), nil, "main")
	existing := FromDetection(frontendDetection(t), nil, "main")
	existing.PullRequest.Title.MaxLength = 0
	got := MissingFields(existing, fresh)
	if !reflect.DeepEqual(got, []string{"pullRequest.title.maxLength"}) {
		t.Errorf("MissingFields = %v", got)
	}

	existing.PullRequest.AutoAssign = nil
	delete(existing.Quality, "lint")
	existing.Tija.Installed = nil
	fresh.Tija.Installed = map[string]string{"a": "b"}
	got = MissingFields(existing, fresh)
	want := []string{"pullRequest.title.maxLength", "pullRequest.autoAssign", "quality.lint"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MissingFields = %v, want %v", got, want)
	}

	off := false
	existing = fresh
	existing.PullRequest.AutoAssign = &off
	if got := MissingFields(existing, fresh); got != nil {
		t.Errorf("explicit false must not be missing: %v", got)
	}
}

func TestFillMissingPreservesDocument(t *testing.T) {
	data := []byte("# my manifest\nversion: 1\npullRequest:\n  target: dev # keep\n  title:\n    convention: conventional-commits\nquality: {}\n")
	fresh := FromDetection(frontendDetection(t), nil, "main")
	out, err := FillMissing(data, fresh, []string{"pullRequest.title.maxLength", "quality.lint", "content.language"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"# my manifest", "target: dev # keep", "maxLength: 100", "lint:", "provider: biome", "language: es"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "target: main") {
		t.Errorf("existing value overwritten:\n%s", s)
	}
}

func TestRecordInstalled(t *testing.T) {
	data := []byte("version: 1 # v\ntija:\n  defaultsVersion: \"0\"\n  installed:\n    .tija/a.md: old\n")
	out, err := RecordInstalled(data, "1", map[string]string{".tija/a.md": "new", ".github/b.md": "h"})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, filepath.FromSlash(Path))
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, out, 0o644)
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{".tija/a.md": "new", ".github/b.md": "h"}
	if m.Tija.DefaultsVersion != "1" || !reflect.DeepEqual(m.Tija.Installed, want) {
		t.Errorf("tija = %+v", m.Tija)
	}
	if !strings.Contains(string(out), "# v") {
		t.Errorf("comment lost:\n%s", out)
	}
}

func TestSetString(t *testing.T) {
	data := []byte("# keep\npullRequest:\n    target: dev\n    template: .tija/templates/pull-request.md\n")
	out, err := SetString(data, "pullRequest.template", ".github/PULL_REQUEST_TEMPLATE.md")
	if err != nil {
		t.Fatal(err)
	}
	want := "# keep\npullRequest:\n    target: dev\n    template: .github/PULL_REQUEST_TEMPLATE.md\n"
	if string(out) != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}
