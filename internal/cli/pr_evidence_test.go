package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
)

func TestPRPrepareEvidenceFlag(t *testing.T) {
	repo := prBranchRepo(t)
	refs := []engine.EvidenceRef{{Source: "qapture", Project: engine.EvidenceProject{ID: "project-1", Name: "Portal"}, ExportPath: `C:\Users\private-user\matriz.xlsx`, Notes: "Capturas funcionales"}}
	data, err := json.Marshal(struct {
		EvidenceRefs []engine.EvidenceRef `json:"evidenceRefs"`
	}{refs})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "evidence.json")
	// Editors on Windows may save JSON with a UTF-8 BOM.
	if err := os.WriteFile(path, append([]byte{0xef, 0xbb, 0xbf}, data...), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := cmdDeps(false)
	want, err := engine.PreparePR(context.Background(), repo, deps.Env, engine.PRRequest{Agent: "none", NoCheck: true, EvidenceRefs: refs}, nil)
	if err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runIn(t, repo, deps, "", "pr", "prepare", "--no-agent", "--no-check", "--evidence", path, "--json")
	if code != 0 {
		t.Fatalf("code=%d out=%s err=%s", code, out, errOut)
	}
	var got engine.PRReport
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.Body != want.Body || got.EvidenceValidation == nil || got.EvidenceValidation.ExternalExistence != "not_checked" {
		t.Fatalf("got=%+v wantBody=%s", got, want.Body)
	}
	if strings.Contains(out, "private-user") || strings.Contains(out, "exportPath") {
		t.Fatalf("path leaked: %s", out)
	}
}

func TestPREvidenceFileErrors(t *testing.T) {
	repo := prBranchRepo(t)
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	invalid := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{"evidenceRefs":[{"source":"jira","project":{"id":"p"},"notes":"text"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"prepare", "create"} {
		for _, path := range []string{filepath.Join(t.TempDir(), "missing.json"), bad, invalid} {
			var calls [][]string
			code, _, errOut := runIn(t, repo, ghDeps(false, &calls), "", "pr", command, "--no-agent", "--no-check", "--evidence", path)
			if code != 2 || errOut == "" {
				t.Fatalf("%s %s: code=%d stderr=%s", command, path, code, errOut)
			}
			if hasGHCall(calls, "create") || hasGHCall(calls, "edit") {
				t.Fatal("published for invalid evidence")
			}
		}
	}
}

func TestPRCreateEvidenceFlagPreparesBeforeConfirmation(t *testing.T) {
	repo := prBranchRepo(t)
	path := filepath.Join(t.TempDir(), "evidence.json")
	if err := os.WriteFile(path, []byte(`{"evidenceRefs":[{"source":"qapture","project":{"id":"project-1","name":"Portal"},"notes":"Caso funcional aprobado"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	code, out, errOut := runIn(t, repo, ghDeps(false, &calls), "", "pr", "create", "--no-agent", "--no-check", "--evidence", path)
	if code != 2 || !strings.Contains(errOut, "confirmation required") {
		t.Fatalf("code=%d out=%s err=%s", code, out, errOut)
	}
	if !strings.Contains(out, "Caso funcional aprobado") {
		t.Fatalf("evidence not forwarded: %s", out)
	}
	if hasGHCall(calls, "create") || hasGHCall(calls, "edit") {
		t.Fatal("published without confirmation")
	}
}
