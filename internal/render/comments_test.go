package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
)

func sampleCleanReport() engine.CleanReport {
	var r engine.CleanReport
	r.Scope.Mode = "paths"
	r.Scope.Paths = []string{"src/pages/(features)/claims"}
	r.Scope.Files = 136
	r.Agent.Provider = "claude"
	r.Agent.Tier = "fast"
	r.Agent.Batches = 12
	r.Summary = engine.CleanSummary{
		Comments:     1420,
		Protected:    9,
		Keep:         610,
		Safe:         640,
		Unsafe:       161,
		Unclassified: 0,
		SkippedFiles: 0,
		FailedFiles:  0,
	}
	r.Files = []engine.CleanFile{
		{
			Path:   "src/pages/(features)/claims/Form.jsx",
			Status: "pending",
			Changes: []engine.CleanChange{
				{
					Line:   120,
					Action: "remove",
					Class:  "separator",
					Safety: "safe",
					Source: "rule",
				},
			},
		},
		{
			Path:   "src/pages/(features)/claims/Header.jsx",
			Status: "written",
			Changes: []engine.CleanChange{
				{
					Line:   45,
					Action: "remove",
					Class:  "separator",
					Safety: "safe",
					Source: "rule",
				},
			},
		},
		{
			Path:   "src/pages/(features)/claims/broken.jsx",
			Status: "failed",
			Reason: "token verification failed",
		},
		{
			Path:   "src/pages/(features)/claims/Table.jsx",
			Status: "restored",
			Reason: "biome reports useExhaustiveDependencies after the edit",
		},
	}
	r.Written = false
	return r
}

func TestRenderCleanCommentsPlain(t *testing.T) {
	got := renderPlain(func(p Printer) { CleanComments(p, sampleCleanReport()) })
	for _, want := range []string{
		"Scope: paths (136 files) · Agent: claude (tier: fast, 12 batches)",
		"! src/pages/(features)/claims/Form.jsx",
		"  L120  remove  separator  safe  rule",
		"✓ src/pages/(features)/claims/Header.jsx",
		"  L45  remove  separator  safe  rule",
		"✗ src/pages/(features)/claims/broken.jsx (token verification failed)",
		"! src/pages/(features)/claims/Table.jsx (restored: biome reports useExhaustiveDependencies after the edit)",
		"Comments 1420 · protected 9 · keep 610 · safe 640 · unsafe 161",
		"run with --write to apply safe changes",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	golden(t, "comments-plain", got)
}

func TestRenderCleanCommentsJSON(t *testing.T) {
	var buf bytes.Buffer
	p := Printer{W: &buf, Mode: JSON}
	CleanComments(p, sampleCleanReport())
	got := buf.String()

	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, got)
	}

	for _, want := range []string{
		`"mode": "paths"`,
		`"provider": "claude"`,
		`"comments": 1420`,
		`"path": "src/pages/(features)/claims/Form.jsx"`,
		`"status": "pending"`,
		`"line": 120`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("JSON lacks %q:\n%s", want, got)
		}
	}
	golden(t, "comments-json", got)
}

func TestRenderCleanCommentsUnsafeHint(t *testing.T) {
	r := sampleCleanReport()
	r.Summary.Safe = 0
	r.Summary.Unsafe = 12
	got := renderPlain(func(p Printer) { CleanComments(p, r) })
	if !strings.Contains(got, "run with --write --unsafe to apply unsafe changes") {
		t.Errorf("expected unsafe hint, got:\n%s", got)
	}
}

func TestRenderCleanCommentsDiff(t *testing.T) {
	r := sampleCleanReport()
	r.Files[0].Diff = "-// removed line\n+// added line"
	got := renderPlain(func(p Printer) { CleanComments(p, r) })
	if !strings.Contains(got, "-// removed line") || !strings.Contains(got, "+// added line") {
		t.Errorf("expected diff lines, got:\n%s", got)
	}
}

func TestRenderCleanCommentsSummaryExtra(t *testing.T) {
	r := sampleCleanReport()
	r.Summary.Unclassified = 5
	r.Summary.SkippedFiles = 2
	r.Summary.FailedFiles = 1
	got := renderPlain(func(p Printer) { CleanComments(p, r) })
	want := "Comments 1420 · protected 9 · keep 610 · safe 640 · unsafe 161 · unclassified 5 · skipped 2 · failed 1"
	if !strings.Contains(got, want) {
		t.Errorf("summary lacks extra counts, want %q in:\n%s", want, got)
	}
}

func TestCleanProgress(t *testing.T) {
	var buf bytes.Buffer
	p := Printer{W: &buf, Mode: Plain}
	CleanProgress(p, engine.CleanEvent{Type: "extract", Done: 2, Total: 10, Path: "src/App.jsx"})
	got := buf.String()
	if !strings.Contains(got, "[extract] 2/10 files") || !strings.Contains(got, "src/App.jsx") {
		t.Errorf("CleanProgress unexpected output: %q", got)
	}

	buf.Reset()
	CleanProgress(p, engine.CleanEvent{Type: "classify", Done: 1, Total: 3})
	got = buf.String()
	if !strings.Contains(got, "[classify] 1/3 batches") {
		t.Errorf("CleanProgress unexpected classify output: %q", got)
	}
}
