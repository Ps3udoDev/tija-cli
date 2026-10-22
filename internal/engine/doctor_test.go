package engine

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

// doctorEnv finds git (real) plus the named fake tools; `gh auth status`
// exits with ghAuth.
func doctorEnv(ghAuth int, found ...string) sys.Env {
	real := sys.Default()
	return sys.Env{
		Run: func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
			if name == "gh" {
				return "", "", ghAuth, nil
			}
			return real.Run(ctx, dir, name, args...)
		},
		LookPath: func(name string) (string, error) {
			if name == "git" {
				return exec.LookPath("git")
			}
			if slices.Contains(found, name) {
				return "/bin/" + name, nil
			}
			return "", errors.New("not found: " + name)
		},
		Getenv: func(string) string { return "" },
	}
}

func initRepo(t *testing.T, name string) string {
	t.Helper()
	repo := testutil.NewRepo(t, fixture(name))
	p := plan(t, repo)
	apply(t, p, nil, approveAll(p))
	return repo
}

func checkByID(r DoctorReport, id string) (Check, bool) {
	for _, c := range r.Checks {
		if c.ID == id {
			return c, true
		}
	}
	return Check{}, false
}

func wantStatus(t *testing.T, r DoctorReport, id string, want CheckStatus) {
	t.Helper()
	c, ok := checkByID(r, id)
	if !ok {
		t.Fatalf("check %s missing in %+v", id, r.Checks)
	}
	if c.Status != want {
		t.Errorf("%s = %s (%s), want %s", id, c.Status, c.Title, want)
	}
}

func TestDoctorFrontendNoGhNoAgents(t *testing.T) {
	repo := initRepo(t, "react-vite")
	r, err := Doctor(context.Background(), repo, fakeEnv())
	if err != nil {
		t.Fatal(err)
	}
	wantStatus(t, r, "gh.installed", StatusFail)
	wantStatus(t, r, "agents.available", StatusWarn)
	wantStatus(t, r, "lockfiles.multiple", StatusWarn)
	wantStatus(t, r, "tests.none", StatusWarn)
	if r.Detection == nil || r.Detection.Framework != "React 19 + Vite 7" || r.Detection.PackageManager != "bun" {
		t.Errorf("Detection = %+v", r.Detection)
	}
	if r.ExitCode() != 1 {
		t.Errorf("ExitCode = %d, want 1", r.ExitCode())
	}
	if _, ok := checkByID(r, "gh.auth"); ok {
		t.Error("gh.auth should be skipped without gh")
	}
}

func TestDoctorAllGood(t *testing.T) {
	repo := initRepo(t, "dotnet-api")
	r, err := Doctor(context.Background(), repo, doctorEnv(0, "gh", "dotnet", "claude", "codex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range r.Checks {
		if c.Status == StatusFail {
			t.Errorf("unexpected fail: %+v", c)
		}
	}
	wantStatus(t, r, "pr.template", StatusOK)
	wantStatus(t, r, "gh.auth", StatusOK)
	if !slices.Equal(r.Agents, []string{"claude", "codex"}) || r.DefaultAgent != "claude" {
		t.Errorf("agents = %v default %q", r.Agents, r.DefaultAgent)
	}
	if r.ExitCode() != 0 {
		t.Errorf("ExitCode = %d, want 0", r.ExitCode())
	}
}

func TestDoctorGhNotAuthenticated(t *testing.T) {
	repo := initRepo(t, "dotnet-api")
	r, err := Doctor(context.Background(), repo, doctorEnv(1, "gh", "dotnet"))
	if err != nil {
		t.Fatal(err)
	}
	c, _ := checkByID(r, "gh.auth")
	if c.Status != StatusFail || c.Hint != "run: gh auth login" {
		t.Errorf("gh.auth = %+v", c)
	}
}

func TestDoctorNotInitialized(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	r, err := Doctor(context.Background(), repo, fakeEnv())
	if err != nil {
		t.Fatal(err)
	}
	c, ok := checkByID(r, "manifest.present")
	if !ok || c.Status != StatusFail || c.Hint != "run: tija init" {
		t.Errorf("manifest.present = %+v", c)
	}
	for _, c := range r.Checks {
		if strings.HasPrefix(c.ID, "tool.") || c.ID == "pr.template" {
			t.Errorf("dependent check ran: %s", c.ID)
		}
	}
	wantStatus(t, r, "gh.installed", StatusFail)
	wantStatus(t, r, "lockfiles.multiple", StatusWarn)
}

func TestDoctorInvalidManifest(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	putFile(t, repo, ".tija/manifest.yaml", []byte(":::"))
	r, err := Doctor(context.Background(), repo, fakeEnv())
	if err != nil {
		t.Fatal(err)
	}
	c, ok := checkByID(r, "manifest.valid")
	if !ok || c.Status != StatusFail || c.Title == "" {
		t.Errorf("manifest.valid = %+v", c)
	}
}

func TestDoctorOutsideRepo(t *testing.T) {
	r, err := Doctor(context.Background(), t.TempDir(), fakeEnv())
	if err != nil {
		t.Fatal(err)
	}
	wantStatus(t, r, "repo.detected", StatusFail)
	for _, c := range r.Checks {
		if strings.HasPrefix(c.ID, "manifest.") {
			t.Errorf("unexpected %s", c.ID)
		}
	}
	if r.Detection != nil {
		t.Errorf("Detection = %+v, want nil outside a repo", r.Detection)
	}
	wantStatus(t, r, "gh.installed", StatusFail)
	wantStatus(t, r, "agents.available", StatusWarn)
}

func TestDoctorGitFailureShowsMessage(t *testing.T) {
	env := fakeEnv()
	msg := "fatal: detected dubious ownership in repository at 'C:/x'"
	env.Run = func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
		return "", msg + "\n", 128, nil
	}
	r, err := Doctor(context.Background(), t.TempDir(), env)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := checkByID(r, "repo.detected")
	if c.Status != StatusFail || !strings.Contains(c.Hint, "detected dubious ownership") {
		t.Errorf("repo.detected = %+v, want fail with git's message", c)
	}
}

func TestDoctorToolChecksDeduplicatedByBinary(t *testing.T) {
	repo := initRepo(t, "dotnet-api")
	r, err := Doctor(context.Background(), repo, doctorEnv(0, "gh"))
	if err != nil {
		t.Fatal(err)
	}
	var tools []string
	for _, c := range r.Checks {
		if strings.HasPrefix(c.ID, "tool.") {
			tools = append(tools, c.ID+"="+c.Title)
		}
	}
	if want := []string{"tool.dotnet=dotnet available"}; !slices.Equal(tools, want) {
		t.Errorf("tool checks = %v, want %v", tools, want)
	}
	c, _ := checkByID(r, "tool.dotnet")
	if c.Status != StatusFail || c.Hint != "install the .NET SDK: https://dot.net" {
		t.Errorf("tool.dotnet = %+v", c)
	}
}

func TestDoctorEmptyPRTemplate(t *testing.T) {
	repo := initRepo(t, "dotnet-api")
	data := readFile(t, repo, ".tija/manifest.yaml")
	data = []byte(strings.Replace(string(data), "template: .github/PULL_REQUEST_TEMPLATE.md", `template: ""`, 1))
	putFile(t, repo, ".tija/manifest.yaml", data)
	r, err := Doctor(context.Background(), repo, doctorEnv(0, "gh", "dotnet"))
	if err != nil {
		t.Fatal(err)
	}
	c, _ := checkByID(r, "pr.template")
	if c.Status != StatusFail || c.Title != "PR template not configured" {
		t.Errorf("pr.template = %+v", c)
	}
}

func TestDoctorGhAuthTimeout(t *testing.T) {
	repo := initRepo(t, "dotnet-api")
	env := doctorEnv(0, "gh", "dotnet")
	real := env.Run
	var deadline time.Duration
	env.Run = func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
		if name == "gh" {
			if d, ok := ctx.Deadline(); ok {
				deadline = time.Until(d)
			}
			return "", "", 0, nil
		}
		return real(ctx, dir, name, args...)
	}
	if _, err := Doctor(context.Background(), repo, env); err != nil {
		t.Fatal(err)
	}
	if deadline <= 0 || deadline > 10*time.Second {
		t.Errorf("gh auth status deadline = %v, want <= 10s", deadline)
	}
}


func TestDoctorChecksScriptFallbackTool(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	putFile(t, repo, "package.json", []byte(`{
  "name": "gmx-frontend",
  "packageManager": "bun@1.3.5",
  "scripts": { "lint": "eslint ." }
}`))
	manifestContent := `version: 1
project:
  name: gmx
  type: react-vite
pullRequest:
  target: main
  template: .github/PULL_REQUEST_TEMPLATE.md
`
	putFile(t, repo, ".tija/manifest.yaml", []byte(manifestContent))
	putFile(t, repo, ".github/PULL_REQUEST_TEMPLATE.md", []byte("PR template"))

	r, err := Doctor(context.Background(), repo, doctorEnv(0, "gh"))
	if err != nil {
		t.Fatal(err)
	}
	c, ok := checkByID(r, "tool.bun")
	if !ok {
		t.Fatalf("expected tool.bun check in %+v", r.Checks)
	}
	if c.Status != StatusFail || c.Hint != "install bun" {
		t.Errorf("tool.bun = %+v, want fail with 'install bun'", c)
	}
}

func TestDoctorMissingBiomeHint(t *testing.T) {
	repo := initRepo(t, "react-vite")
	r, err := Doctor(context.Background(), repo, doctorEnv(0, "gh"))
	if err != nil {
		t.Fatal(err)
	}
	c, ok := checkByID(r, "tool.biome")
	if !ok {
		t.Fatalf("expected tool.biome check in %+v", r.Checks)
	}
	if c.Status != StatusFail || c.Hint != "run: bun install" {
		t.Errorf("tool.biome = %+v, want fail with 'run: bun install'", c)
	}
}
