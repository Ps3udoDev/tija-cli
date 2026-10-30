package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/workflows"
)

func TestRenderWorkflowsJSON(t *testing.T) {
	var buf bytes.Buffer
	p := Printer{W: &buf, Mode: JSON}
	wfs := []workflows.Workflow{
		{Name: "pre-pr", Description: "Pre-PR checks"},
	}
	Workflows(p, wfs)

	var got []workflows.Workflow
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if len(got) != 1 || got[0].Name != "pre-pr" {
		t.Errorf("got %+v, want pre-pr", got)
	}
}

func TestRenderWorkflowsPlain(t *testing.T) {
	var buf bytes.Buffer
	p := Printer{W: &buf, Mode: Plain}
	wfs := []workflows.Workflow{
		{Name: "pre-pr", Description: "Pre-PR checks"},
	}
	Workflows(p, wfs)

	out := buf.String()
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "DESCRIPTION") {
		t.Errorf("expected headers in output, got: %s", out)
	}
	if !strings.Contains(out, "pre-pr") || !strings.Contains(out, "Pre-PR checks") {
		t.Errorf("expected workflow content in output, got: %s", out)
	}
}

func TestRenderWorkflowReport(t *testing.T) {
	var buf bytes.Buffer
	p := Printer{W: &buf, Mode: JSON}
	rep := workflows.WorkflowReport{
		Name: "pre-pr",
		Exit: 0,
		Steps: []workflows.StepResult{
			{Run: "check", Exit: 0},
		},
	}
	WorkflowReport(p, rep)

	var got workflows.WorkflowReport
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if got.Name != "pre-pr" || got.Exit != 0 || len(got.Steps) != 1 {
		t.Errorf("got %+v, want pre-pr with 1 step", got)
	}
}
