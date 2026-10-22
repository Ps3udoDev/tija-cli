package engine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/assets"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

// fakeEnv finds only git on PATH.
func fakeEnv() sys.Env {
	return sys.Env{
		Run: sys.Default().Run,
		LookPath: func(name string) (string, error) {
			if name == "git" {
				return exec.LookPath("git")
			}
			return "", errors.New("not found: " + name)
		},
		Getenv: func(string) string { return "" },
	}
}

func fixture(name string) string { return filepath.Join("..", "detect", "testdata", name) }

func approveAll(p InitPlan) map[string]bool {
	m := map[string]bool{}
	for _, a := range p.Actions {
		m[a.ID] = true
	}
	return m
}

func plan(t *testing.T, dir string) InitPlan {
	t.Helper()
	p, err := PlanInit(context.Background(), dir, fakeEnv())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func apply(t *testing.T, p InitPlan, answers map[string]string, approved map[string]bool) InitResult {
	t.Helper()
	r, err := ApplyInit(context.Background(), p, answers, approved)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func readFile(t *testing.T, root, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func putFile(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

func pendingPaths(r InitResult) []string {
	var out []string
	for _, a := range r.Pending {
		out = append(out, a.Path)
	}
	return out
}

func findAction(p InitPlan, id string) (Action, bool) {
	for _, a := range p.Actions {
		if a.ID == id {
			return a, true
		}
	}
	return Action{}, false
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func samePath(t *testing.T, a, b string) bool {
	t.Helper()
	sa, err1 := os.Stat(a)
	sb, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(sa, sb)
}

func TestInitFrontendNonInteractive(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	putFile(t, repo, ".gitignore", []byte("node_modules\n"))

	p := plan(t, repo)
	if p.AlreadyInitialized {
		t.Error("AlreadyInitialized = true on a fresh repo")
	}
	if _, ok := findAction(p, "create:.github/PULL_REQUEST_TEMPLATE.md"); ok {
		t.Error("offered .github template although the repo has one")
	}
	gi, ok := findAction(p, "update:.gitignore")
	if !ok || !gi.NeedsConsent || !strings.Contains(gi.Diff, "+.tija/cache/") {
		t.Errorf("gitignore action = %+v, %v", gi, ok)
	}

	r := apply(t, p, nil, nil)
	if !slices.Contains(r.Created, ".tija/manifest.yaml") || !slices.Contains(r.Created, ".tija/rules/pull-request.md") {
		t.Errorf("Created = %v", r.Created)
	}
	if !reflect.DeepEqual(pendingPaths(r), []string{".gitignore"}) {
		t.Errorf("Pending = %v", pendingPaths(r))
	}
	if got := readFile(t, repo, ".gitignore"); string(got) != "node_modules\n" {
		t.Errorf(".gitignore modified: %q", got)
	}
	if exists(repo, ".tija/templates/pull-request.md") {
		t.Error("default template installed although the repo has one")
	}
	if !exists(repo, ".tija/workflows/.gitkeep") {
		t.Error(".tija/workflows/.gitkeep missing")
	}
	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if m.PullRequest.Template != ".github/PULL_REQUEST_TEMPLATE.md" || m.PullRequest.Target != "main" || m.Project.Type != "react-vite" {
		t.Errorf("manifest = %+v", m)
	}
	if errs := m.Validate(); errs != nil {
		t.Errorf("Validate() = %v", errs)
	}
}

func TestInitBackendWithYes(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("dotnet-api"))
	p := plan(t, repo)
	offer, ok := findAction(p, "create:.github/PULL_REQUEST_TEMPLATE.md")
	if !ok || !offer.NeedsConsent || offer.Reason != "offer: copy PR template to .github" {
		t.Fatalf("offer = %+v, %v", offer, ok)
	}

	r := apply(t, p, nil, approveAll(p))
	if len(r.Pending) != 0 {
		t.Errorf("Pending = %v", r.Pending)
	}
	tpl := assets.Files()[assets.TemplatePath]
	if got := readFile(t, repo, ".github/PULL_REQUEST_TEMPLATE.md"); !bytes.Equal(got, tpl) {
		t.Error(".github/PULL_REQUEST_TEMPLATE.md differs from the embedded default")
	}
	if exists(repo, ".tija/rules/pull-request.md") {
		t.Error("default rules installed although the repo has a guide")
	}
	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if m.PullRequest.Rules != "templates/PULL_REQUEST_TEMPLATE_GUIDE.md" || m.PullRequest.Template != ".tija/templates/pull-request.md" {
		t.Errorf("pullRequest = %+v", m.PullRequest)
	}
	wantInstalled := map[string]string{
		".github/PULL_REQUEST_TEMPLATE.md": sum(tpl),
		".tija/templates/pull-request.md":  sum(tpl),
		".tija/workflows/.gitkeep":         sum(nil),
	}
	if m.Tija.DefaultsVersion != assets.DefaultsVersion || !reflect.DeepEqual(m.Tija.Installed, wantInstalled) {
		t.Errorf("tija = %+v", m.Tija)
	}
	if got := readFile(t, repo, ".gitignore"); !strings.Contains(string(got), ".tija/session/") {
		t.Errorf(".gitignore = %q", got)
	}
}

func TestInitIdempotent(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("dotnet-api"))
	p := plan(t, repo)
	apply(t, p, nil, approveAll(p))

	p2 := plan(t, repo)
	if !p2.AlreadyInitialized {
		t.Error("AlreadyInitialized = false after init")
	}
	r := apply(t, p2, nil, approveAll(p2))
	if len(r.Created) != 0 || len(r.Updated) != 0 || len(r.Pending) != 0 {
		t.Errorf("second run: Created %v Updated %v Pending %v", r.Created, r.Updated, r.Pending)
	}
}

func TestInitKeepsUserEditedRules(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	p := plan(t, repo)
	apply(t, p, nil, approveAll(p))

	edited := []byte("# my own rules\n")
	putFile(t, repo, ".tija/rules/pull-request.md", edited)
	p2 := plan(t, repo)
	r := apply(t, p2, nil, nil)
	if got := readFile(t, repo, ".tija/rules/pull-request.md"); !bytes.Equal(got, edited) {
		t.Errorf("edited rules overwritten: %q", got)
	}
	if len(r.Pending) != 1 || r.Pending[0].Kind != KindReplace || !r.Pending[0].NeedsConsent || r.Pending[0].Path != ".tija/rules/pull-request.md" {
		t.Errorf("Pending = %+v", r.Pending)
	}
}

func TestInitUpgradesUneditedDefaults(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	p := plan(t, repo)
	apply(t, p, nil, approveAll(p))

	// Simulate a default installed by an older Tija version.
	old := []byte("# old default rules\n")
	putFile(t, repo, ".tija/rules/pull-request.md", old)
	data, err := manifest.RecordInstalled(readFile(t, repo, manifest.Path), "0", map[string]string{".tija/rules/pull-request.md": sum(old)})
	if err != nil {
		t.Fatal(err)
	}
	putFile(t, repo, manifest.Path, data)

	p2 := plan(t, repo)
	a, ok := findAction(p2, "replace:.tija/rules/pull-request.md")
	if !ok || a.NeedsConsent {
		t.Fatalf("replace action = %+v, %v", a, ok)
	}
	r := apply(t, p2, nil, nil)
	rules := assets.Files()[assets.RulesPath]
	if got := readFile(t, repo, ".tija/rules/pull-request.md"); !bytes.Equal(got, rules) {
		t.Error("unedited old default not upgraded")
	}
	if !slices.Contains(r.Updated, ".tija/rules/pull-request.md") || !slices.Contains(r.Updated, manifest.Path) {
		t.Errorf("Updated = %v", r.Updated)
	}
	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if m.Tija.DefaultsVersion != assets.DefaultsVersion || m.Tija.Installed[".tija/rules/pull-request.md"] != sum(rules) {
		t.Errorf("tija = %+v", m.Tija)
	}
}

func TestInitProposesMissingFields(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	p := plan(t, repo)
	apply(t, p, nil, approveAll(p))

	data := string(readFile(t, repo, manifest.Path))
	if !strings.Contains(data, "    maxLength: 100\n") {
		t.Fatalf("unexpected manifest layout:\n%s", data)
	}
	data = "# mine\n" + strings.Replace(data, "    maxLength: 100\n", "", 1)
	putFile(t, repo, manifest.Path, []byte(data))

	p2 := plan(t, repo)
	a, ok := findAction(p2, "update:.tija/manifest.yaml")
	if !ok || !a.NeedsConsent || !strings.Contains(a.Reason, "pullRequest.title.maxLength") || !strings.Contains(a.Diff, "+    maxLength: 100") {
		t.Fatalf("manifest update = %+v, %v", a, ok)
	}
	r := apply(t, p2, nil, nil)
	if !reflect.DeepEqual(pendingPaths(r), []string{manifest.Path}) || string(readFile(t, repo, manifest.Path)) != data {
		t.Errorf("unapproved update applied; Pending = %v", pendingPaths(r))
	}

	r = apply(t, p2, nil, approveAll(p2))
	got := string(readFile(t, repo, manifest.Path))
	if !strings.HasPrefix(got, "# mine\n") || !strings.Contains(got, "maxLength: 100") {
		t.Errorf("merged manifest:\n%s", got)
	}
	if !reflect.DeepEqual(r.Updated, []string{manifest.Path}) {
		t.Errorf("Updated = %v", r.Updated)
	}
}

func TestInitFromSubdirectory(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("dotnet-api"))
	p := plan(t, filepath.Join(repo, "src"))
	if !samePath(t, p.Root, repo) {
		t.Fatalf("Root = %q, want %q", p.Root, repo)
	}
	apply(t, p, nil, nil)
	if exists(repo, "src/.tija") {
		t.Error(".tija created in the subdirectory")
	}
	if !exists(repo, manifest.Path) {
		t.Error("manifest not created at the root")
	}
}

func TestInitNotARepo(t *testing.T) {
	_, err := PlanInit(context.Background(), t.TempDir(), fakeEnv())
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("err = %v, want ErrNotARepo", err)
	}
}

func TestInitMultiSlnChoice(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("multi-sln"))
	p := plan(t, repo)
	var sol *Choice
	for i := range p.Choices {
		if p.Choices[i].ID == "solution" {
			sol = &p.Choices[i]
		}
	}
	if sol == nil || !reflect.DeepEqual(sol.Options, []string{"A.sln", "B.sln"}) || sol.Default != "A.sln" {
		t.Fatalf("solution choice = %+v", sol)
	}
	r := apply(t, p, map[string]string{"solution": "B.sln"}, nil)
	if r.Detection.Solution != "B.sln" {
		t.Errorf("Detection.Solution = %q", r.Detection.Solution)
	}
	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if m.Project.Solution != "B.sln" {
		t.Errorf("Project.Solution = %q", m.Project.Solution)
	}
	if _, err := ApplyInit(context.Background(), plan(t, repo), map[string]string{"solution": "C.sln"}, nil); err == nil {
		t.Error("unknown solution accepted")
	}
}

func TestInitTargetChoice(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("unknown"), testutil.WithBranch("dev"))
	p := plan(t, repo)
	var target *Choice
	for i := range p.Choices {
		if p.Choices[i].ID == "target" {
			target = &p.Choices[i]
		}
	}
	if target == nil || !reflect.DeepEqual(target.Options, []string{"main", "dev"}) || target.Default != "main" {
		t.Fatalf("target choice = %+v", target)
	}
	apply(t, p, map[string]string{"target": "dev"}, nil)
	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if m.PullRequest.Target != "dev" {
		t.Errorf("Target = %q", m.PullRequest.Target)
	}
}

func TestDefaultTarget(t *testing.T) {
	ctx := context.Background()
	run := sys.Default().Run

	repo := testutil.NewRepo(t, "", testutil.WithRemote("", true), testutil.WithBranch("feat/x"))
	testutil.Git(t, repo, "branch", "-m", "main", "trunk")
	if got := defaultTarget(ctx, gitx.Repo{Dir: repo, Run: run}); got != "main" {
		t.Errorf("with origin/HEAD: %q, want main", got)
	}

	repo = testutil.NewRepo(t, "", testutil.WithBranch("feat/x"))
	testutil.Git(t, repo, "branch", "-m", "main", "master")
	if got := defaultTarget(ctx, gitx.Repo{Dir: repo, Run: run}); got != "master" {
		t.Errorf("local master: %q", got)
	}

	repo = testutil.NewRepo(t, "", testutil.WithBranch("feat/x"))
	testutil.Git(t, repo, "branch", "-m", "main", "trunk")
	if got := defaultTarget(ctx, gitx.Repo{Dir: repo, Run: run}); got != "feat/x" {
		t.Errorf("fallback: %q, want current branch", got)
	}
}

func TestInitDoesNotOverwriteInvalidManifest(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("unknown"))
	putFile(t, repo, manifest.Path, []byte(":::"))
	p := plan(t, repo)
	if !p.AlreadyInitialized {
		t.Error("AlreadyInitialized = false with a manifest present")
	}
	r := apply(t, p, nil, approveAll(p))
	if got := readFile(t, repo, manifest.Path); string(got) != ":::" {
		t.Errorf("manifest modified: %q", got)
	}
	found := false
	for _, n := range r.Notes {
		found = found || strings.Contains(n, "manifest is invalid")
	}
	if !found {
		t.Errorf("Notes = %v", r.Notes)
	}
}
