package render

import (
	"fmt"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
)

// ActivityReport renders the outcome of a report as text or JSON.
func ActivityReport(p Printer, r engine.ReportResult) {
	if p.Mode == JSON {
		_ = p.JSONOut(r)
		return
	}
	p.KV("Period", r.Since+" to "+r.Until)
	p.KV("Found", fmt.Sprintf("%d commits, %d pull requests", r.Commits, r.PRs))
	if r.Agent != "" {
		p.KV("Agent", r.Agent)
	}
	if r.PDF != "" {
		p.Ok("PDF: " + r.PDF)
	} else if r.Activity != "" {
		p.Line("Activity: " + r.Activity)
	}
	for _, w := range r.Warnings {
		p.Warn(w)
	}
}
