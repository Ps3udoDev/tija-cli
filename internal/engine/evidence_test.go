package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sampleEvidence() EvidenceRef {
	return EvidenceRef{Source: "qapture", Project: EvidenceProject{ID: "project-1", Name: "Portal"}, Cases: []EvidenceCase{{ID: "case-1", Code: "EJ-014", Title: "Inicio con MFA", Status: "Aprobado"}}, ExportPath: `C:\Users\private-user\matriz.xlsx`, Notes: "Capturas del sistema migrado"}
}

func TestNormalizeEvidenceRules(t *testing.T) {
	tests := []struct {
		name, field string
		mutate      func(*EvidenceRef)
	}{
		{"source", "[0].source: only \"qapture\" is supported", func(r *EvidenceRef) { r.Source = "jira" }},
		{"project", "[0].project.id", func(r *EvidenceRef) { r.Project.ID = " " }},
		{"minimum", "at least one", func(r *EvidenceRef) { r.Cases = nil; r.ExportPath = ""; r.Notes = " " }},
		{"caseIdentity", "[0].cases[0]: id or code", func(r *EvidenceRef) { r.Cases[0].ID = ""; r.Cases[0].Code = "" }},
		{"titleNewline", "[0].cases[0].title", func(r *EvidenceRef) { r.Cases[0].Title = "line\nline" }},
		{"titleCR", "[0].cases[0].title", func(r *EvidenceRef) { r.Cases[0].Title = "line\rline" }},
		{"unicodeNewline", "[0].notes", func(r *EvidenceRef) { r.Notes = "line\u2028line" }},
		{"longNotes", "[0].notes", func(r *EvidenceRef) { r.Notes = strings.Repeat("á", 201) }},
		{"longProject", "[0].project.name", func(r *EvidenceRef) { r.Project.Name = strings.Repeat("x", 201) }},
		{"longPath", "[0].exportPath", func(r *EvidenceRef) { r.ExportPath = strings.Repeat("x", 201) }},
		{"pathNewline", "[0].exportPath", func(r *EvidenceRef) { r.ExportPath = "dir\nfile.xlsx" }},
		{"caseLimit", "[0].cases", func(r *EvidenceRef) { r.Cases = make([]EvidenceCase, 51) }},
		{"outputOnly", "[0].exportName", func(r *EvidenceRef) { r.ExportPath = ""; r.ExportName = "free-text.xlsx" }},
		{"trailingSeparator", "[0].exportPath", func(r *EvidenceRef) { r.ExportPath = `C:\dir\` }},
		{"dotName", "[0].exportPath", func(r *EvidenceRef) { r.ExportPath = "dir/.." }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := sampleEvidence()
			tt.mutate(&r)
			_, err := NormalizeEvidence([]EvidenceRef{r})
			if err == nil || !strings.HasPrefix(err.Error(), "invalid evidenceRefs:") || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("error=%v, want %q", err, tt.field)
			}
		})
	}
	if _, err := NormalizeEvidence(make([]EvidenceRef, 11)); err == nil || !strings.Contains(err.Error(), "10 references") {
		t.Fatalf("limit error=%v", err)
	}
	r := sampleEvidence()
	r.Notes = strings.Repeat("á", 200)
	r.Cases = make([]EvidenceCase, 50)
	for i := range r.Cases {
		r.Cases[i].ID = "id"
	}
	refs := make([]EvidenceRef, 10)
	for i := range refs {
		refs[i] = r
	}
	got, err := NormalizeEvidence(refs)
	if err != nil || len(got) != 10 {
		t.Fatalf("valid limits: %+v %v", got, err)
	}
	got[0].Cases[0].ID = "changed"
	if refs[0].Cases[0].ID != "id" {
		t.Fatal("normalization aliases caller cases")
	}
	for _, in := range [][]EvidenceRef{nil, {}} {
		out, err := NormalizeEvidence(in)
		if err != nil || !reflect.DeepEqual(in, out) {
			t.Fatalf("nil/empty identity: %#v => %#v %v", in, out, err)
		}
	}
}

func TestEvidenceExportPathWindowsOnLinux(t *testing.T) {
	for _, tt := range []struct{ path, name string }{{`C:\Users\x\matriz.xlsx`, "matriz.xlsx"}, {`C:/a\b/c.xlsx`, "c.xlsx"}, {"/tmp/matrix.xlsx", "matrix.xlsx"}, {"matrix.xlsx", "matrix.xlsx"}} {
		r := sampleEvidence()
		r.ExportPath = tt.path
		r.ExportName = "ignored-free-text"
		got, err := NormalizeEvidence([]EvidenceRef{r})
		if err != nil {
			t.Fatal(err)
		}
		if got[0].ExportPath != "" || got[0].ExportName != tt.name {
			t.Fatalf("%q => %+v", tt.path, got[0])
		}
		if r.ExportPath != tt.path {
			t.Fatal("caller input mutated")
		}
	}
}

func TestRenderEvidenceFormat(t *testing.T) {
	r := sampleEvidence()
	r.Cases = append(r.Cases, EvidenceCase{ID: "case-2"})
	refs, err := NormalizeEvidence([]EvidenceRef{r})
	if err != nil {
		t.Fatal(err)
	}
	want := "- Qapture · proyecto «Portal»\n  - EJ-014 Inicio con MFA — Aprobado\n  - case-2\n  - Matriz exportada: matriz.xlsx\n  - Nota: Capturas del sistema migrado"
	if got := renderEvidence(refs); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	refs[0].Project.Name = ""
	if !strings.Contains(renderEvidence(refs), "«project-1»") {
		t.Fatal("missing project id fallback")
	}
}

func TestPreparePRReplacesEvidenciasSection(t *testing.T) {
	for _, withRefs := range []bool{false, true} {
		t.Run(map[bool]string{false: "withoutRefs", true: "withRefs"}[withRefs], func(t *testing.T) {
			repo := prRepo(t)
			draft := strings.Replace(validAgentDraft, "BODY>>>", "\n## Evidencias\n\nCapturas en Drive\n\n## eViDeNcIaS\n\nEvidencia inventada duplicada\nBODY>>>", 1)
			env, _ := fakeAgentEnv(agentReply{out: draft})
			req := PRRequest{Agent: "claude", NoCheck: true}
			if withRefs {
				req.EvidenceRefs = []EvidenceRef{sampleEvidence()}
			}
			rep, err := PreparePR(context.Background(), repo, env, req, nil)
			if err != nil {
				t.Fatal(err)
			}
			if withRefs {
				if strings.Contains(rep.Body, "Drive") || !strings.Contains(rep.Body, renderEvidence(rep.Evidence)) {
					t.Fatalf("body=%s", rep.Body)
				}
				if strings.Contains(rep.Body, "inventada duplicada") || strings.Count(strings.ToLower(rep.Body), "## evidencias") != 1 {
					t.Fatalf("duplicate evidence survived: %s", rep.Body)
				}
				if rep.EvidenceValidation == nil || *rep.EvidenceValidation != (EvidenceValidation{"passed", "not_checked"}) {
					t.Fatalf("validation=%+v", rep.EvidenceValidation)
				}
			} else if !strings.Contains(rep.Body, "Drive") || rep.EvidenceValidation != nil {
				t.Fatalf("body or validation changed: %+v", rep)
			}
		})
	}
}

func TestEnforceEvidenceMergesDuplicateSections(t *testing.T) {
	refs, err := NormalizeEvidence([]EvidenceRef{sampleEvidence()})
	if err != nil {
		t.Fatal(err)
	}
	body := "## Resumen\r\n\r\nResumen conservado\r\n\r\n## EVIDENCIAS\r\n\r\nPrimera inventada\r\n\r\n## Objetivo\r\n\r\nObjetivo conservado\r\n\r\n## evidencias\r\n\r\nSegunda inventada\r\n\r\n## Otra sección\r\n\r\nContenido conservado\r\n\r\n## Evidencias\r\n\r\nTercera inventada\r\n"
	got := enforceEvidence(body, refs)
	if strings.Contains(got, "inventada") || strings.Count(strings.ToLower(got), "## evidencias") != 1 {
		t.Fatalf("duplicates: %s", got)
	}
	for _, want := range []string{"Resumen conservado", "Objetivo conservado", "Contenido conservado", renderEvidence(refs)} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q: %s", want, got)
		}
	}
}

func TestPreparePRDeterministicEvidence(t *testing.T) {
	repo := prRepo(t)
	rep, err := PreparePR(context.Background(), repo, fakeEnv(), PRRequest{Agent: "none", NoCheck: true, EvidenceRefs: []EvidenceRef{sampleEvidence()}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rep.Body, renderEvidence(rep.Evidence)) {
		t.Fatalf("body=%s", rep.Body)
	}
}

func TestPreparePRInvalidEvidenceSavesNothing(t *testing.T) {
	repo := prRepo(t)
	env, calls := fakeAgentEnv(agentReply{out: validAgentDraft})
	r := sampleEvidence()
	r.Source = "jira"
	_, err := PreparePR(context.Background(), repo, env, PRRequest{Agent: "claude", NoCheck: true, EvidenceRefs: []EvidenceRef{r}}, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid evidenceRefs") {
		t.Fatalf("error=%v", err)
	}
	if len(*calls) != 0 {
		t.Fatal("agent invoked for invalid references")
	}
	for _, path := range []string{DraftPath, DraftMetaPath} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("draft path %s exists: %v", path, err)
		}
	}
}

func TestDraftMetaNeverStoresFullPath(t *testing.T) {
	repo := prRepo(t)
	ref := sampleEvidence()
	rep, err := PreparePR(context.Background(), repo, fakeEnv(), PRRequest{Agent: "none", NoCheck: true, EvidenceRefs: []EvidenceRef{ref}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{DraftPath, DraftMetaPath} {
		data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "private-user") || strings.Contains(string(data), "exportPath") {
			t.Fatalf("private path in %s: %s", path, data)
		}
	}
	data, _ := json.Marshal(rep)
	if strings.Contains(string(data), "private-user") || strings.Contains(string(data), "exportPath") {
		t.Fatalf("private path in result: %s", data)
	}
	_, meta, err := LoadDraft(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(meta.Evidence, rep.Evidence) {
		t.Fatalf("meta=%+v report=%+v", meta.Evidence, rep.Evidence)
	}
}
