package render

import (
	"fmt"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/prrules"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
)

// PRPrepare renders a pull request draft report in plain text, styled text or JSON.
func PRPrepare(p Printer, r engine.PRReport) {
	if p.Mode == JSON {
		if r.Problems == nil {
			r.Problems = []prrules.Problem{}
		}
		if r.Tests == nil {
			r.Tests = []quality.StepResult{}
		}
		if r.Warnings == nil {
			r.Warnings = []string{}
		}
		_ = p.JSONOut(r)
		return
	}

	p.Heading(fmt.Sprintf("Pull request  %s → %s", r.Branch, r.Target))
	p.Ok(fmt.Sprintf("%s, %s", count(r.Commits, "commit"), count(r.Files, "file")))
	if len(r.Tests) == 0 {
		p.Warn("no verified test results for this commit")
	}
	for _, s := range r.Tests {
		line := "tests: " + s.Command
		if tc := prTestCounts(s.Tests); tc != "" {
			line += " — " + tc
		}
		switch s.Status {
		case quality.StatusPassed:
			p.Ok(line)
		case quality.StatusFailed:
			p.Fail(line)
		default: // skipped or unavailable: not a failure, but not verified either
			p.Warn(line)
		}
	}
	for _, w := range r.Warnings {
		p.Warn(w)
	}
	if r.Source == engine.SourceDeterministic {
		p.Ok("drafted without an agent")
	} else {
		p.Ok(fmt.Sprintf("drafted by %s (%s)", r.Source, count(r.Attempts, "attempt")))
	}

	p.Line("")
	p.Line(r.Title)
	p.Line("")
	p.Line(strings.TrimRight(r.Body, "\n"))
	p.Line("")
	if len(r.Problems) == 0 {
		p.Ok("title and sections pass pr-governance rules")
	}
	for _, pr := range r.Problems {
		p.Fail(pr.Message)
	}
	if r.DraftPath != "" {
		p.Line("Draft: " + r.DraftPath)
	}
}

// PRCreate renders the outcome of creating or updating a pull request.
func PRCreate(p Printer, plan engine.CreatePlan, r engine.CreateReport) {
	if p.Mode == JSON {
		_ = p.JSONOut(struct {
			Plan   engine.CreatePlan   `json:"plan"`
			Result engine.CreateReport `json:"result"`
		}{plan, r})
		return
	}
	if r.Pushed {
		p.Ok("pushed " + plan.Report.Branch + " to origin")
	}
	if r.Updated {
		p.Ok("updated pull request " + r.URL)
	} else {
		p.Ok("created pull request " + r.URL)
	}
	if r.Assigned {
		p.Ok("assigned to you")
	}
}

func count(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// prTestCounts always shows passed and failed, and skipped only when there are any.
func prTestCounts(t *quality.TestCounts) string {
	if t == nil {
		return ""
	}
	s := fmt.Sprintf("%d passed, %d failed", t.Passed, t.Failed)
	if t.Skipped > 0 {
		s += fmt.Sprintf(", %d skipped", t.Skipped)
	}
	return s
}
