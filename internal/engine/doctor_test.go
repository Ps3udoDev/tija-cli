package engine

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

// doctorEnv finds git (real) plus the named fake tools; `gh auth status`
// exits with ghAuth.
func doctorEnv(ghAuth int, found ...string) sys.Env {
	real := sys.Default()
	return sys.Env{
		Run: func(ctx context.Context, dir, name string, args ...string) (string, int, error) {
			if name == "gh" {
				return "", ghAuth, nil
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
