package render

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/branch"
	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/engine"
)

var update = flag.Bool("update", false, "rewrite golden files")

// golden compares got with testdata/<name>.golden, rewriting it with -update.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update): %v", err)
	}
	// Normalize CRLF in case git converted the golden on checkout.
	if w := strings.ReplaceAll(string(want), "\r\n", "\n"); w != got {
		t.Errorf("output mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", path, got, w)
	}
}

func sampleInit() engine.InitResult {
	return engine.InitResult{
		Detection: detect.Detection{
			Root:           "/work/frontend",
			ProjectName:    "frontend",
			Stack:          detect.StackReactVite,
			Language:       "JavaScript",
			Framework:      "React 19 + Vite 7",
			PackageManager: "bun",
			Lockfiles:      []string{"bun.lock", "package-lock.json"},
			Tools:          []string{"biome", "commitlint"},
			CI:             []string{".github/workflows/pr-governance.yml"},
			GitHost:        "github",
		},
		Created: []string{".tija/rules/pull-request.md", ".tija/workflows/.gitkeep", ".tija/manifest.yaml"},
		Updated: []string{".tija/templates/pull-request.md"},
		Pending: []engine.Action{{
			ID: "update:.gitignore", Kind: engine.KindUpdate, Path: ".gitignore",
			Reason: "ignore local .tija/ paths", NeedsConsent: true,
		}},
		Notes: []string{"multiple PR templates found; using .github/PULL_REQUEST_TEMPLATE.md"},
	}
}

func sampleDoctor() engine.DoctorReport {
	return engine.DoctorReport{
		Root: "/work/frontend",
		Detection: &detect.Detection{
			Language: "JavaScript", Framework: "React 19 + Vite 7", PackageManager: "bun",
			Tools: []string{"biome"}, GitHost: "github",
		},
		Checks: []engine.Check{
			{ID: "git.installed", Title: "Git installed", Status: engine.StatusOK},
			{ID: "repo.detected", Title: "repository detected", Status: engine.StatusOK},
			{ID: "manifest.present", Title: "manifest present", Status: engine.StatusFail, Hint: "run: tija init"},
			{ID: "gh.installed", Title: "GitHub CLI installed", Status: engine.StatusOK},
			{ID: "gh.auth", Title: "GitHub CLI authenticated", Status: engine.StatusFail, Hint: "run: gh auth login"},
			{ID: "agents.available", Title: "agents: claude, codex (default: claude)", Status: engine.StatusOK},
			{ID: "lockfiles.multiple", Title: "multiple lockfiles: bun.lock, package-lock.json", Status: engine.StatusWarn, Hint: "keep a single lockfile"},
			{ID: "tests.none", Title: "no tests detected", Status: engine.StatusWarn},
		},
		Agents:       []string{"claude", "codex"},
		DefaultAgent: "claude",
	}
}

func sampleStatusIssueBranch() engine.StatusReport {
	return engine.StatusReport{
		Root: "/work/frontend",
		Branch: branch.Verdict{
			State:       branch.Nonconforming,
			Name:        "101-fixclaims-fix-integration-issues",
			Suggested:   "fix/101-claims-integration-issues",
			Reason:      "does not follow <prefix>/<short-description>",
			IssueNumber: 101,
		},
		Target: "main", TargetFound: true, Ahead: 2, Behind: 1,
		Staged: 1, Unstaged: 0, Untracked: 3,
		HasRemote: true, Published: false, HasCommits: true,
		HuskyWarning: true,
	}
}

func renderPlain(f func(Printer)) string {
	var b bytes.Buffer
	f(Printer{W: &b, Mode: Plain})
	return b.String()
}

func TestRenderInitPlain(t *testing.T) {
	got := renderPlain(func(p Printer) { InitResult(p, sampleInit()) })
	for _, want := range []string{
		"Detected project\n", "Language:       JavaScript\n", "Created:\n", "Updated:\n",
		"Pending (run with --yes or interactively):\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q", want)
		}
	}
	golden(t, "init_plain", got)
}

func TestRenderDoctorPlain(t *testing.T) {
	got := renderPlain(func(p Printer) { Doctor(p, sampleDoctor()) })
	for _, want := range []string{"Detected project\n", "Framework:      React 19 + Vite 7\n", "Packages:       bun\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q", want)
		}
	}
	golden(t, "doctor_plain", got)
}

func TestRenderDoctorOutsideRepoPlain(t *testing.T) {
	r := engine.DoctorReport{Checks: []engine.Check{{ID: "repo.detected", Title: "repository detected", Status: engine.StatusFail, Hint: "run inside a git repository"}}}
	got := renderPlain(func(p Printer) { Doctor(p, r) })
	if want := "✗ repository detected — run inside a git repository\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderStatusIssueBranchPlain(t *testing.T) {
	got := renderPlain(func(p Printer) { Status(p, sampleStatusIssueBranch()) })
	for _, want := range []string{
		`! branch "101-fixclaims-fix-integration-issues" — does not follow <prefix>/<short-description>` + "\n",
		"  suggested: fix/101-claims-integration-issues\n",
		"  rename: git branch -m 101-fixclaims-fix-integration-issues fix/101-claims-integration-issues\n",
		"! this repo's pre-push hook will reject this branch\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q", want)
		}
	}
	golden(t, "status_issue_branch_plain", got)
}

func TestRenderStatusConformingPlain(t *testing.T) {
	r := engine.StatusReport{
		Branch: branch.Verdict{State: branch.Conforming, Name: "feat/login"},
		Target: "main", TargetFound: true, HasCommits: true, HasRemote: true, Published: true,
	}
	got := renderPlain(func(p Printer) { Status(p, r) })
	if !strings.Contains(got, `✓ branch "feat/login" follows the naming rules`) {
		t.Errorf("got %q", got)
	}
	if !strings.Contains(got, "Working tree:   clean\n") {
		t.Errorf("got %q", got)
	}
}

func TestRenderJSONIsSingleDocument(t *testing.T) {
	cases := map[string]func(Printer){
		"init":         func(p Printer) { InitResult(p, engine.InitResult{}) },
		"doctor":       func(p Printer) { Doctor(p, sampleDoctor()) },
		"doctor-empty": func(p Printer) { Doctor(p, engine.DoctorReport{}) },
		"status":       func(p Printer) { Status(p, sampleStatusIssueBranch()) },
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			f(Printer{W: &b, Mode: JSON})
			dec := json.NewDecoder(&b)
			var v map[string]any
			if err := dec.Decode(&v); err != nil {
				t.Fatalf("invalid JSON: %v\n%s", err, b.String())
			}
			if dec.More() {
				t.Fatal("more than one JSON document")
			}
			if name == "doctor" {
				if _, ok := v["detection"].(map[string]any); !ok {
					t.Errorf("detection = %v, want an object", v["detection"])
				}
			}
			if name == "doctor-empty" {
				if _, ok := v["agents"].([]any); !ok {
					t.Errorf("agents = %v, want an array", v["agents"])
				}
				if _, ok := v["detection"]; ok {
					t.Error("detection present without a repo")
				}
			}
			if name == "init" {
				for _, k := range []string{"created", "updated", "pending", "notes"} {
					if _, ok := v[k].([]any); !ok {
						t.Errorf("%s = %v, want an array", k, v[k])
					}
				}
			}
		})
	}
}

func TestRenderStyledUsesANSI(t *testing.T) {
	var b bytes.Buffer
	Doctor(Printer{W: &b, Mode: Styled}, sampleDoctor())
	if !strings.Contains(b.String(), "\x1b[") {
		t.Fatalf("styled output has no ANSI: %q", b.String())
	}
}

func TestRenderStatusRenameHintQuotesAndPush(t *testing.T) {
	r := engine.StatusReport{
		Branch: branch.Verdict{State: branch.Nonconforming, Name: "it's (wip)", Suggested: "feat/wip"},
		Target: "main", TargetFound: true, HasCommits: true, HasRemote: true, Published: true,
	}
	got := renderPlain(func(p Printer) { Status(p, r) })
	for _, want := range []string{
		`  rename: git branch -m 'it'\''s (wip)' feat/wip` + "\n",
		`  then: git push origin -u feat/wip && git push origin --delete 'it'\''s (wip)'` + "\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}

	r.Published = false
	r.Branch.Name = "wip"
	got = renderPlain(func(p Printer) { Status(p, r) })
	if !strings.Contains(got, "  rename: git branch -m wip feat/wip\n") || strings.Contains(got, "then:") {
		t.Errorf("unpublished branch:\n%s", got)
	}
}

func TestShellQuote(t *testing.T) {
	for in, want := range map[string]string{
		"feat/x-1.2_a": "feat/x-1.2_a",
		"a b":          "'a b'",
		"it's":         `'it'\''s'`,
		"":             "''",
	} {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
