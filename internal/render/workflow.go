package render

import (
	"fmt"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/workflows"
)

// Workflows renders a list of workflows as a table or JSON.
func Workflows(p Printer, wfs []workflows.Workflow) {
	if p.Mode == JSON {
		if wfs == nil {
			wfs = []workflows.Workflow{}
		}
		_ = p.JSONOut(wfs)
		return
	}

	headers := []string{"NAME", "DESCRIPTION"}
	rows := make([][]string, len(wfs))
	for i, w := range wfs {
		rows[i] = []string{w.Name, w.Description}
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	formatRow := func(cols []string) string {
		var parts []string
		for i, col := range cols {
			if i == len(cols)-1 {
				parts = append(parts, col)
			} else {
				parts = append(parts, fmt.Sprintf("%-*s", widths[i], col))
			}
		}
		return strings.Join(parts, "  ")
	}

	headerLine := formatRow(headers)
	p.Line(headerLine)
	for _, row := range rows {
		p.Line(formatRow(row))
	}
}

// WorkflowReport renders a workflow execution report as JSON or plain text.
func WorkflowReport(p Printer, r workflows.WorkflowReport) {
	if p.Mode == JSON {
		_ = p.JSONOut(r)
		return
	}
	for _, s := range r.Steps {
		if s.Exit == 0 {
			p.Ok(fmt.Sprintf("%s (exit 0)", s.Run))
		} else {
			p.Fail(fmt.Sprintf("%s (exit %d)", s.Run, s.Exit))
		}
	}
}
