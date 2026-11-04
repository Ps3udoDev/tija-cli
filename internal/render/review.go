package render

import (
	"fmt"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/review"
)

func Review(p Printer, r engine.ReviewReport) {
	if p.Mode == JSON {
		_ = p.JSONOut(r.Result)
		return
	}
	for _, f := range r.Findings {
		location := f.File
		if f.Line > 0 {
			location += fmt.Sprintf(":%d", f.Line)
		}
		text := fmt.Sprintf("%-8s %s  %s", f.Severity, location, f.Rule)
		if len(f.ConfirmedBy) > 0 {
			text += "  [" + strings.Join(f.ConfirmedBy, ", ") + "]"
		}
		if f.Disputed {
			values := []string{}
			for _, s := range f.Severities {
				values = append(values, string(s))
			}
			text += " (disputed: " + strings.Join(values, "/") + ")"
		}
		if f.Severity.Rank() >= review.High.Rank() {
			p.Fail(text)
		} else {
			p.Warn(text)
		}
		if f.Title != "" && f.Title != f.Explanation {
			p.Line("  " + f.Title)
		}
		if f.Explanation != "" {
			p.Line("  " + f.Explanation)
		}
		if f.Suggestion != "" {
			p.Line("  → " + f.Suggestion)
		}
		for _, c := range f.PRComments {
			p.Line("  PR comment: " + c.URL)
		}
	}
	if len(r.Findings) == 0 {
		p.Ok("no findings")
	}
	p.Line(fmt.Sprintf("Summary: %d critical, %d high, %d medium, %d low · %d discarded · quality: %s · mode: %s", r.Summary[review.Critical], r.Summary[review.High], r.Summary[review.Medium], r.Summary[review.Low], len(r.Discarded), r.Quality.Status, r.Mode))
	if len(r.Discarded) > 0 {
		p.Line("Use --json to inspect discarded findings.")
	}
	if r.Chunks.Failed > 0 {
		p.Fail(fmt.Sprintf("%d/%d chunks failed", r.Chunks.Failed, r.Chunks.Total))
	}
	if len(r.PRComments.Uncovered) > 0 {
		p.Heading("PR comments not covered")
		for _, c := range r.PRComments.Uncovered {
			p.Line(fmt.Sprintf("  %s:%d %s", c.Path, c.Line, c.URL))
		}
	}
	for _, w := range r.Warnings {
		p.Warn(w)
	}
}

func ReviewProgress(p Printer, ev engine.ReviewEvent) {
	if p.Mode != JSON {
		p.Line("> " + ev.Message)
	}
}
