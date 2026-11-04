package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/assets"
	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/review"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func TestPrePRStopsAtReview(t *testing.T) {
	repo := testutil.NewRepo(t, "", testutil.WithBranch("feat/review-gate"))
	write := func(path, text string) {
		t.Helper()
		path = filepath.Join(repo, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(assets.PrePRWorkflowPath, string(assets.Files()[assets.PrePRWorkflowPath]))
	write(".gitignore", ".tija/session/\n.tija/tmp/\n")
	write("package.json", `{"name":"review-gate","private":true}`)
	write(".tija/rules/architecture/gate.md", "---\nname: workflow-gate\nchecks:\n  - id: workflow-forbidden\n    severity: High\n    files: '*.ts'\n    forbid: forbidden\n    message: Seeded architecture violation\n---\n")
	m := manifest.FromDetection(detect.Detection{ProjectName: "review-gate", Stack: detect.StackUnknown}, nil, "main")
	// An empty default with no installed CLI is the manifest's no-agent setup.
	m.Agents.Default = ""
	m.Quality = map[string]manifest.QualityEntry{"lint": {Provider: "biome"}, "format": {Provider: "none"}, "build": {Provider: "none"}, "tests": {Provider: "none"}}
	if err := manifest.Save(repo, m); err != nil {
		t.Fatal(err)
	}
	if errs := m.Validate(); len(errs) > 0 {
		t.Fatal(errs)
	}
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-m", "workflow setup")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	write("a.ts", "const forbidden = true;\n")
	deps := cmdDeps(false)
	look := deps.Env.LookPath
	qualityCalls := 0
	deps.Env.LookPath = func(name string) (string, error) {
		if name == "biome" {
			return "fake-biome", nil
		}
		return look(name)
	}
	deps.Env.Stream = func(_ context.Context, _ string, w io.Writer, name string, args ...string) (int, error) {
		qualityCalls++
		if name != "fake-biome" {
			t.Errorf("unexpected tool: %s %v", name, args)
		}
		fmt.Fprintln(w, "Quality passed")
		return 0, nil
	}
	code, out, stderr := runIn(t, repo, deps, "", "run", "pre-pr", "--json")
	if code != 1 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, out, stderr)
	}
	var report WorkflowReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(report.Steps) != 3 || report.Steps[0].Run != "clean comments --write" || report.Steps[0].Exit != 0 || report.Steps[1].Run != "check" || report.Steps[1].Exit != 0 || report.Steps[2].Run != "review" || report.Steps[2].Exit != 1 {
		t.Fatalf("%+v", report)
	}
	var reviewed review.Result
	if err := json.Unmarshal(report.Steps[2].Output, &reviewed); err != nil {
		t.Fatal(err)
	}
	if qualityCalls < 1 || len(reviewed.Findings) != 1 || reviewed.Findings[0].Rule != "workflow-forbidden" || reviewed.Findings[0].Severity != review.High {
		t.Fatalf("quality calls=%d review=%+v", qualityCalls, reviewed)
	}
	if _, err := os.Stat(filepath.Join(repo, ".tija/tmp/pr")); !os.IsNotExist(err) {
		t.Fatalf("pr prepare wrote a draft after review gate: %v", err)
	}
}
