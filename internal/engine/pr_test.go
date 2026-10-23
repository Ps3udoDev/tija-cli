package engine

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/agents"
	"github.com/Ps3udoDev/tija-cli/internal/assets"
	"github.com/Ps3udoDev/tija-cli/internal/branch"
	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/prrules"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func writeTestFile(t *testing.T, repo, rel, content string) {
	t.Helper()
	p := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitFile(t *testing.T, repo, rel, content, msg string) {
	t.Helper()
	writeTestFile(t, repo, rel, content)
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-m", msg)
}

func collect(t *testing.T, repo string) PRContext {
	t.Helper()
	c, err := CollectPRContext(context.Background(), repo, fakeEnv())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCollectPRContext(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"), testutil.WithRemote("", true), testutil.WithBranch("fix/133-password-leak"))
	commitFile(t, repo, "src/auth/password.ts", "export const mask = (s: string) => '*'.repeat(s.length)\n", "fix(auth): mask passwords in logs")
	commitFile(t, repo, "src/auth/password.test.ts", "test('mask', () => {})\n", "test(auth): cover password masking")

	c := collect(t, repo)
	if c.IssueNumber != 133 {
		t.Errorf("IssueNumber = %d", c.IssueNumber)
	}
	if c.BaseRef != "refs/remotes/origin/main" || c.Target != "main" || c.Branch != "fix/133-password-leak" {
		t.Errorf("base/target/branch = %q %q %q", c.BaseRef, c.Target, c.Branch)
	}
	if len(c.Commits) != 2 || c.Commits[0].Subject != "fix(auth): mask passwords in logs" {
		t.Errorf("commits = %+v", c.Commits)
	}
	if len(c.Files) == 0 || !strings.Contains(c.Diff, "src/auth/password.ts") {
		t.Errorf("files = %+v diff = %q", c.Files, c.Diff)
	}
	if c.Initialized {
		t.Error("repo must not be initialized")
	}
	files := assets.Files()
	if c.Rules != string(files[assets.RulesPath]) || c.Template != string(files[assets.TemplatePath]) {
		t.Error("rules/template must be the embedded defaults")
	}
	if c.Manifest.PullRequest.Target != "main" || len(c.Manifest.PullRequest.RequiredSections) == 0 {
		t.Errorf("manifest = %+v", c.Manifest.PullRequest)
	}
	if c.Head == "" || c.Dirty {
		t.Errorf("head = %q dirty = %v", c.Head, c.Dirty)
	}
}

func TestCollectPRContextErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("base", func(t *testing.T) {
		repo := testutil.NewRepo(t, "")
		if _, err := CollectPRContext(ctx, repo, fakeEnv()); !errors.Is(err, ErrBaseBranch) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("detached", func(t *testing.T) {
		repo := testutil.NewRepo(t, "")
		testutil.Git(t, repo, "checkout", "--detach")
		if _, err := CollectPRContext(ctx, repo, fakeEnv()); !errors.Is(err, ErrDetached) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("target", func(t *testing.T) {
		repo := initRepo(t, "react-vite")
		path := filepath.Join(repo, filepath.FromSlash(manifest.Path))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		m, err := manifest.Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		old := "target: " + m.PullRequest.Target
		if !strings.Contains(string(data), old) {
			t.Fatalf("manifest has no %q", old)
		}
		writeTestFile(t, repo, manifest.Path, strings.Replace(string(data), old, "target: nope", 1))
		testutil.Git(t, repo, "checkout", "-b", "feat/thing")
		commitFile(t, repo, "x.txt", "x\n", "feat(x): add x")
		if _, err := CollectPRContext(ctx, repo, fakeEnv()); !errors.Is(err, ErrTargetNotFound) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("no commits", func(t *testing.T) {
		repo := testutil.NewRepo(t, "", testutil.WithBranch("feat/empty"))
		if _, err := CollectPRContext(ctx, repo, fakeEnv()); !errors.Is(err, ErrNoCommits) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestCollectPRContextTruncatesDiff(t *testing.T) {
	repo := testutil.NewRepo(t, "", testutil.WithBranch("feat/big-change"))
	line := strings.Repeat("a", 99) + "\n"
	writeTestFile(t, repo, "package-lock.json", strings.Repeat(line, 1000))
	writeTestFile(t, repo, "big.txt", strings.Repeat(line, 700))
	writeTestFile(t, repo, "small.go", "package small\n")
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-m", "feat(big): add files")

	c := collect(t, repo)
	if !strings.Contains(c.Diff, "small.go") {
		t.Error("diff must contain small.go")
	}
	if strings.Contains(c.Diff, "package-lock.json") {
		t.Error("diff must not contain package-lock.json")
	}
	if len(c.Diff) > maxDiffBytes {
		t.Errorf("len(diff) = %d", len(c.Diff))
	}
	for _, p := range []string{"package-lock.json", "big.txt"} {
		if !slices.Contains(c.Truncated, p) {
			t.Errorf("Truncated = %v, want %s", c.Truncated, p)
		}
	}
	if slices.Contains(c.Truncated, "small.go") {
		t.Error("small.go must not be truncated")
	}
}

func TestExcludedFromDiff(t *testing.T) {
	for path, want := range map[string]bool{
		"package-lock.json":          true,
		"web/pnpm-lock.yaml":         true,
		"src/Api/bin/Debug/a.dll":    true,
		"src/Api/obj/project.json":   true,
		"dist/index.js":              true,
		"node_modules/x/index.js":    true,
		".tija/session/quality.json": true,
		"public/app.min.js":          true,
		"src/binder.go":              false,
		"src/distance.ts":            false,
		"src/main.go":                false,
	} {
		if got := excludedFromDiff(gitx.FileStat{Path: path}); got != want {
			t.Errorf("excludedFromDiff(%q) = %v", path, got)
		}
	}
	if !excludedFromDiff(gitx.FileStat{Path: "logo.png", Binary: true}) {
		t.Error("binary files are excluded")
	}
}

func TestFreshTests(t *testing.T) {
	head := "abc"
	s := quality.Session{Version: 1, Results: map[quality.Kind]quality.StepResult{
		quality.Tests:  {Kinds: []quality.Kind{quality.Tests}, Command: "fresh", Commit: head},
		quality.Lint:   {Kinds: []quality.Kind{quality.Lint}, Command: "old", Commit: "def"},
		quality.Format: {Kinds: []quality.Kind{quality.Format}, Command: "dirty", Commit: head, Dirty: true},
	}}
	got := FreshTests(s, head)
	if len(got) != 1 || got[0].Command != "fresh" {
		t.Errorf("FreshTests = %+v", got)
	}
}

func TestFreshTestsDedupesMultiKind(t *testing.T) {
	r := quality.StepResult{Kinds: []quality.Kind{quality.Lint, quality.Format}, Provider: "biome", Command: "biome ci .", Commit: "h"}
	s := quality.Session{Results: map[quality.Kind]quality.StepResult{quality.Lint: r, quality.Format: r}}
	if got := FreshTests(s, "h"); len(got) != 1 {
		t.Errorf("FreshTests = %+v", got)
	}
}

func sampleContext() PRContext {
	m := manifest.FromDetection(detect.Detection{}, nil, "main")
	return PRContext{
		Root: "/repo", Branch: "fix/133-password-leak", Target: "main", BaseRef: "refs/remotes/origin/main", Head: "h1",
		Verdict:     branch.Verdict{State: branch.Conforming, Name: "fix/133-password-leak", IssueNumber: 133},
		IssueNumber: 133,
		Commits: []gitx.Commit{
			{SHA: "a", Subject: "fix(auth): mask passwords in logs", Body: "Uses a fixed mask."},
			{SHA: "b", Subject: "test(auth): cover password masking"},
		},
		Files:    []gitx.FileStat{{Path: "src/auth/password.ts", Added: 3, Deleted: 1}, {Path: "src/auth/password.test.ts", Added: 5}},
		Diff:     "diff --git a/src/auth/password.ts b/src/auth/password.ts\n",
		Rules:    "REPO RULES",
		Template: string(assets.Files()[assets.TemplatePath]),
		Manifest: m,
	}
}

func TestBuildPRPromptContainsFacts(t *testing.T) {
	c := sampleContext()
	p := BuildPRPrompt(c, nil)
	for _, want := range []string{
		"Closes #133", "fix(auth): mask passwords in logs", "test(auth): cover password masking",
		"Do not claim any test was run", "REPO RULES", "## Resumen", "Spanish", "src/auth/password.ts",
		"<<<TITLE", "BODY>>>",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if strings.Contains(p, "Your previous answer failed") {
		t.Error("no problems were passed")
	}

	c.Tests = []quality.StepResult{{Command: "vitest run", Status: quality.StatusPassed, Tests: &quality.TestCounts{Total: 4, Passed: 4}}}
	c.Truncated = []string{"package-lock.json"}
	p = BuildPRPrompt(c, []prrules.Problem{{Field: "section:Pruebas", Message: "section ## Pruebas needs useful content"}})
	for _, want := range []string{"vitest run", "Your previous answer failed these checks:", "section ## Pruebas needs useful content", "package-lock.json"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if strings.Contains(p, "Do not claim any test was run") {
		t.Error("tests are verified")
	}
}

func TestDeterministicDraftValid(t *testing.T) {
	for name, prefix := range map[string]string{
		"fix/133-password-leak":                "fix(password):",
		"feature/shared-imputation-form":       "feat(shared):",
		"101-fixclaims-fix-integration-issues": "fix(",
	} {
		t.Run(name, func(t *testing.T) {
			c := sampleContext()
			c.Branch = name
			c.Verdict = branch.Classify(name, false, branch.Rules{Prefixes: branch.DefaultPrefixes, Bases: branch.DefaultBases, Target: "main"})
			c.IssueNumber = c.Verdict.IssueNumber
			d := DeterministicDraft(c)
			if !strings.HasPrefix(d.Title, prefix) {
				t.Errorf("title = %q", d.Title)
			}
			if ps := prrules.ValidateTitle(d.Title, c.Manifest.PullRequest); len(ps) > 0 {
				t.Errorf("title problems: %v", ps)
			}
			if ps := prrules.ValidateBody(d.Body, c.Manifest.PullRequest); len(ps) > 0 {
				t.Errorf("body problems: %v\n%s", ps, d.Body)
			}
			if strings.Contains(d.Body, "<!--") {
				t.Error("HTML comments must be removed")
			}
			if c.IssueNumber > 0 && !strings.Contains(d.Body, "Closes #"+strconv.Itoa(c.IssueNumber)) {
				t.Errorf("body lacks Closes: %s", d.Body)
			}
		})
	}
}

func TestDeterministicDraftContent(t *testing.T) {
	c := sampleContext()
	c.Tests = []quality.StepResult{
		{Command: "dotnet test SampleService.sln", Status: quality.StatusPassed, Tests: &quality.TestCounts{Total: 74, Passed: 74}},
		{Command: "biome ci .", Status: quality.StatusPassed},
	}
	d := DeterministicDraft(c)
	if d.Title != "fix(password): password leak" {
		t.Errorf("title = %q", d.Title)
	}
	for _, want := range []string{
		"Cambios de la rama `fix/133-password-leak` hacia `main`: 2 commit(s) y 2 archivo(s) modificados.",
		"Password leak. Relacionado con el issue #133.",
		"- mask passwords in logs",
		"- cover password masking",
		"- `dotnet test SampleService.sln`: 74 superadas, 0 fallidas.",
		"- `biome ci .`: sin errores.",
		"## Evidencias\n\nN/A",
		"## Referencias\n\nCloses #133",
	} {
		if !strings.Contains(d.Body, want) {
			t.Errorf("body lacks %q:\n%s", want, d.Body)
		}
	}
}

func TestDeterministicDraftTitleMaxLength(t *testing.T) {
	c := sampleContext()
	c.Branch = "feat/very-long-branch-description-that-keeps-going-and-going-forever"
	c.Verdict = branch.Verdict{State: branch.Conforming, Name: c.Branch}
	c.IssueNumber = 0
	c.Manifest.PullRequest.Title.MaxLength = 40
	d := DeterministicDraft(c)
	if len(d.Title) > 40 || strings.HasSuffix(d.Title, " ") || !strings.HasPrefix(d.Title, "feat(very): very long") {
		t.Errorf("title = %q (%d)", d.Title, len(d.Title))
	}
	if ps := prrules.ValidateTitle(d.Title, c.Manifest.PullRequest); len(ps) > 0 {
		t.Errorf("problems: %v", ps)
	}
	if !strings.Contains(d.Body, "Rama `"+c.Branch+"`.") {
		t.Errorf("references: %s", d.Body)
	}
}

func TestDeterministicDraftIgnoresStaleSession(t *testing.T) {
	repo := testutil.NewRepo(t, "", testutil.WithBranch("fix/133-password-leak"))
	commitFile(t, repo, "src/a.go", "package a\n", "fix(a): repair a")
	s := quality.StepResult{Kinds: []quality.Kind{quality.Tests}, Provider: "vitest", Command: "vitest run", Status: quality.StatusPassed,
		Commit: "0000000000000000000000000000000000000000", Tests: &quality.TestCounts{Total: 3, Passed: 3}}
	if err := quality.SaveSession(repo, []quality.StepResult{s}); err != nil {
		t.Fatal(err)
	}
	c := collect(t, repo)
	if len(c.Tests) != 0 {
		t.Errorf("Tests = %+v", c.Tests)
	}
	d := DeterministicDraft(c)
	content, _ := prrules.SectionContent(d.Body, "Pruebas")
	if !strings.Contains(content, "No se ejecutaron verificaciones automáticas") {
		t.Errorf("Pruebas = %q", content)
	}
	if strings.Contains(d.Body, "vitest run") {
		t.Error("stale result cited")
	}
}

func TestSaveLoadDraftRoundTrip(t *testing.T) {
	root := t.TempDir()
	if _, _, err := LoadDraft(root); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want ErrNotExist", err)
	}
	d := agents.Draft{Title: "fix(auth): mask passwords", Body: "## Resumen\r\n\r\nTexto largo.\r\n"}
	meta := DraftMeta{Head: "h1", Branch: "fix/133-password-leak", Target: "main", Source: "deterministic",
		Problems: []prrules.Problem{{Field: "title", Message: "x"}}}
	if err := SaveDraft(root, d, meta); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".tija", "session", "pr.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "\r") || string(raw) != "fix(auth): mask passwords\n\n## Resumen\n\nTexto largo.\n" {
		t.Errorf("pr.md = %q", raw)
	}
	js, err := os.ReadFile(filepath.Join(root, ".tija", "session", "pr.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `"head": "h1"`) || !strings.Contains(string(js), `"problems"`) {
		t.Errorf("pr.json = %s", js)
	}
	got, gotMeta, err := LoadDraft(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != d.Title || got.Body != "## Resumen\n\nTexto largo.\n" {
		t.Errorf("draft = %+v", got)
	}
	if gotMeta.Head != "h1" || gotMeta.Source != "deterministic" || len(gotMeta.Problems) != 1 || gotMeta.Problems[0].Field != "title" {
		t.Errorf("meta = %+v", gotMeta)
	}
}

func TestDeterministicDraftFailedChecks(t *testing.T) {
	c := sampleContext()
	c.Tests = []quality.StepResult{
		{Command: "vitest run", Status: quality.StatusFailed, ExitCode: 1, Tests: &quality.TestCounts{Total: 74, Passed: 70, Failed: 4}},
		{Command: "dotnet test", Status: quality.StatusFailed, ExitCode: -1, TimedOut: true, Tests: &quality.TestCounts{Total: 10, Passed: 10}},
		{Command: "biome ci .", Status: quality.StatusFailed, ExitCode: 2},
		{Command: "eslint .", Status: quality.StatusSkipped, Tests: &quality.TestCounts{Total: 1, Passed: 1}},
	}
	content, _ := prrules.SectionContent(DeterministicDraft(c).Body, "Pruebas")
	for _, want := range []string{
		"`vitest run`: 70 superadas, 4 fallidas; falló (código de salida 1).",
		"`dotnet test`: 10 superadas, 0 fallidas; falló por tiempo agotado.",
		"`biome ci .`: falló (código de salida 2).",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("Pruebas lacks %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "eslint") {
		t.Errorf("skipped step cited:\n%s", content)
	}
	for _, l := range strings.Split(content, "\n") {
		if strings.Contains(l, "superadas") && !strings.Contains(l, "falló") {
			t.Errorf("failed run presented as passing: %q", l)
		}
	}
	p := BuildPRPrompt(c, nil)
	if !strings.Contains(p, "`dotnet test`: failed (timed out)") {
		t.Error("prompt must state the timeout")
	}
}
