package render

import (
	"fmt"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
)

// CleanComments renders a comment cleaning report in plain text, styled text or JSON.
func CleanComments(p Printer, r engine.CleanReport) {
	if p.Mode == JSON {
		if r.Files == nil {
			r.Files = []engine.CleanFile{}
		}
		_ = p.JSONOut(r)
		return
	}

	mode := r.Scope.Mode
	if mode == "" {
		mode = "diff"
	}
	header := fmt.Sprintf("Scope: %s (%d files)", mode, r.Scope.Files)
	if r.Agent.Provider != "" && r.Agent.Batches > 0 {
		header += fmt.Sprintf(" · Agent: %s (tier: %s, %d batches)", r.Agent.Provider, r.Agent.Tier, r.Agent.Batches)
	}
	p.Heading(header)

	for _, f := range r.Files {
		if f.Status == "unchanged" {
			continue
		}
		switch f.Status {
		case "written":
			p.Ok(f.Path)
		case "pending":
			p.Warn(f.Path)
		case "failed":
			msg := f.Path
			if f.Reason != "" {
				msg = fmt.Sprintf("%s (%s)", f.Path, f.Reason)
			}
			p.Fail(msg)
		case "restored":
			msg := fmt.Sprintf("%s (restored: %s)", f.Path, f.Reason)
			if f.Reason == "" {
				msg = fmt.Sprintf("%s (restored)", f.Path)
			}
			p.Warn(msg)
		case "skipped":
			msg := fmt.Sprintf("%s (skipped: %s)", f.Path, f.Reason)
			if f.Reason == "" {
				msg = fmt.Sprintf("%s (skipped)", f.Path)
			}
			p.Warn(msg)
		default:
			p.Line(fmt.Sprintf("%s %s", f.Status, f.Path))
		}

		for _, ch := range f.Changes {
			p.Line(fmt.Sprintf("  L%d  %s  %s  %s  %s", ch.Line, ch.Action, ch.Class, ch.Safety, ch.Source))
		}

		if f.Diff != "" {
			for _, line := range strings.Split(f.Diff, "\n") {
				if line == "" {
					continue
				}
				if p.Mode == Styled {
					if strings.HasPrefix(line, "+") {
						p.Line(okStyle.Render(line))
					} else if strings.HasPrefix(line, "-") {
						p.Line(failStyle.Render(line))
					} else {
						p.Line(line)
					}
				} else {
					p.Line(line)
				}
			}
		}
	}

	for _, w := range r.Warnings {
		msg := strings.TrimPrefix(w, "! ")
		p.Warn(msg)
	}

	parts := []string{
		fmt.Sprintf("Comments %d", r.Summary.Comments),
		fmt.Sprintf("protected %d", r.Summary.Protected),
		fmt.Sprintf("keep %d", r.Summary.Keep),
		fmt.Sprintf("safe %d", r.Summary.Safe),
		fmt.Sprintf("unsafe %d", r.Summary.Unsafe),
	}
	if r.Summary.Unclassified > 0 {
		parts = append(parts, fmt.Sprintf("unclassified %d", r.Summary.Unclassified))
	}
	if r.Summary.SkippedFiles > 0 {
		parts = append(parts, fmt.Sprintf("skipped %d", r.Summary.SkippedFiles))
	}
	if r.Summary.FailedFiles > 0 {
		parts = append(parts, fmt.Sprintf("failed %d", r.Summary.FailedFiles))
	}
	p.Line(strings.Join(parts, " · "))

	if !r.Written {
		if r.Summary.Safe > 0 {
			p.Line("run with --write to apply safe changes")
		} else if r.Summary.Unsafe > 0 {
			p.Line("run with --write --unsafe to apply unsafe changes")
		}
	}
}

// CleanProgress reports intermediate progress during comment cleaning.
func CleanProgress(p Printer, ev engine.CleanEvent) {
	if p.Mode == JSON || ev.Total <= 0 {
		return
	}
	unit := "files"
	if ev.Type == "classify" {
		unit = "batches"
	}
	msg := fmt.Sprintf("[%s] %d/%d %s", ev.Type, ev.Done, ev.Total, unit)
	if ev.Path != "" {
		msg += " " + ev.Path
	}
	p.Line(msg)
}
