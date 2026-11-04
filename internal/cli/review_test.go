package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func TestReviewFlags(t *testing.T) {
	dir := testutil.NewRepo(t, "", testutil.WithBranch("feat/review"))
	for _, args := range [][]string{{"--quality", "--deep"}, {"--fail-on", "urgent"}, {"--agent-timeout", "0s"}, {"--agent", "oops"}} {
		code, _, _ := runIn(t, dir, cmdDeps(false), "", append([]string{"review"}, args...)...)
		if code != 2 {
			t.Fatalf("%v code %d", args, code)
		}
	}
}
func TestReviewJSON(t *testing.T) {
	dir := testutil.NewRepo(t, "", testutil.WithBranch("feat/review"))
	code, out, stderr := runIn(t, dir, cmdDeps(false), "", "review", "--json", "--no-agent", "--no-check", "--no-pr-comments")
	if code != 0 || stderr != "" {
		t.Fatalf("%d %s %s", code, out, stderr)
	}
	var object map[string]json.RawMessage
	if e := json.Unmarshal([]byte(out), &object); e != nil {
		t.Fatal(e)
	}
	for _, k := range []string{"mode", "base", "commit", "dirty", "failOn", "exit", "quality", "findings", "discarded", "prComments", "chunks", "summary"} {
		if _, ok := object[k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
}
func TestReviewExitCodes(t *testing.T) {
	dir := testutil.NewRepo(t, "", testutil.WithBranch("feat/review"))
	p := filepath.Join(dir, ".tija", "rules", "architecture")
	if e := os.MkdirAll(p, 0755); e != nil {
		t.Fatal(e)
	}
	// Init supplies the configured rules glob and ignores session output.
	if code, _, err := runIn(t, dir, cmdDeps(false), "", "init", "--yes"); code != 0 {
		t.Fatal(err)
	}
	if e := os.WriteFile(filepath.Join(p, "bad.md"), []byte("---\nname: bad\nchecks:\n  - id: bad\n    severity: High\n    forbid: danger\n    files: '*.ts'\n    message: violation\n---\n"), 0644); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(dir, "a.ts"), []byte("danger\n"), 0644); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		threshold string
		want      int
	}{{"high", 1}, {"none", 0}, {"critical", 0}} {
		code, _, err := runIn(t, dir, cmdDeps(false), "", "review", "--no-agent", "--no-check", "--no-pr-comments", "--fail-on", tc.threshold)
		if code != tc.want {
			t.Fatalf("%s code %d: %s", tc.threshold, code, err)
		}
	}
	if code, _, _ := runIn(t, t.TempDir(), cmdDeps(false), "", "review"); code != 2 {
		t.Fatalf("not repo: %d", code)
	}
}
