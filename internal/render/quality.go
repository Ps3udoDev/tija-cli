package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
)

// FormatDuration formats duration as "%.1fs" below one minute and "%dm%02ds" from one minute.
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d / time.Minute)
	s := int((d % time.Minute) / time.Second)
	return fmt.Sprintf("%dm%02ds", m, s)
}

// Quality renders a full quality report in plain text, styled text or JSON.
func Quality(p Printer, r engine.QualityReport) {
	if p.Mode == JSON {
		if r.Steps == nil {
			r.Steps = []quality.StepResult{}
		}
		if r.ChangedFiles == nil {
			r.ChangedFiles = []string{}
		}
		if r.Warnings == nil {
			r.Warnings = []string{}
		}
		_ = p.JSONOut(r)
		return
	}

	title := "Check"
	if r.Mode == "fix" {
		title = "Fix"
	}
	p.Heading(title)

	if len(r.Steps) == 0 {
		p.Warn("nothing to run — no quality tools configured or detected")
	} else {
		for i, step := range r.Steps {
			p.Line(renderStepLine(p.Mode, step, r.Steps[:i]))
			if step.Status == quality.StatusFailed {
				for _, line := range step.Tail {
					p.Line("    " + line)
				}
				if step.Log != "" {
					p.Line("    log: " + step.Log)
				}
			}
		}
	}

	if r.Mode == "fix" && len(r.Steps) > 0 {
		p.Line("")
		if len(r.ChangedFiles) == 0 {
			p.Line("No files changed")
		} else {
			p.Heading("Changed files:")
			for _, f := range r.ChangedFiles {
				p.Line("  " + f)
			}
		}
	}

	if len(r.Warnings) > 0 {
		p.Line("")
		for _, w := range r.Warnings {
			p.Warn(w)
		}
	}
}

// QualityProgress prints a single line for an engine.EventDone event.
func QualityProgress(p Printer, ev engine.QualityEvent) {
	if p.Mode == JSON || ev.Type != engine.EventDone {
		return
	}
	p.Line(renderStepLine(p.Mode, ev.Step, nil))
}

func renderStepLine(mode Mode, step quality.StepResult, prevSteps []quality.StepResult) string {
	var sym string
	switch step.Status {
	case quality.StatusPassed:
		sym = symOk
		if mode == Styled {
			sym = okStyle.Render(sym)
		}
	case quality.StatusFailed:
		sym = symFail
		if mode == Styled {
			sym = failStyle.Render(sym)
		}
	case quality.StatusSkipped:
		sym = symSkip
		if mode == Styled {
			sym = skipStyle.Render(sym)
		}
	case quality.StatusUnavailable:
		sym = symWarn
		if mode == Styled {
			sym = warnStyle.Render(sym)
		}
	default:
		sym = " "
	}

	var kinds []string
	for _, k := range step.Kinds {
		kinds = append(kinds, string(k))
	}
	kindsStr := strings.Join(kinds, " + ")
	kindsCol := fmt.Sprintf("%-15s", kindsStr)

	switch step.Status {
	case quality.StatusSkipped:
		reason := ""
		for _, prev := range prevSteps {
			if prev.TimedOut {
				reason = "timeout"
				break
			}
			if prev.Status == quality.StatusFailed {
				var prevKinds []string
				for _, k := range prev.Kinds {
					prevKinds = append(prevKinds, string(k))
				}
				reason = strings.Join(prevKinds, " + ") + " failed"
				break
			}
		}
		msg := "skipped"
		if reason != "" {
			msg = fmt.Sprintf("skipped (%s)", reason)
		}
		return fmt.Sprintf("%s %s%s", sym, kindsCol, msg)

	case quality.StatusUnavailable:
		msg := step.Provider + " unavailable"
		if step.Hint != "" {
			msg += " — " + step.Hint
		}
		return fmt.Sprintf("%s %s%s", sym, kindsCol, msg)

	case quality.StatusPassed, quality.StatusFailed:
		dur := FormatDuration(time.Duration(step.DurationMs) * time.Millisecond)
		cmd := step.Command
		targetWidth := 61
		var mid string
		if len(cmd)+2+len(dur) <= targetWidth {
			mid = cmd + strings.Repeat(" ", targetWidth-len(cmd)-len(dur)) + dur
		} else {
			mid = cmd + "  " + dur
		}
		if step.Tests != nil {
			tc := formatTestCounts(step.Tests)
			if tc != "" {
				mid += "  " + tc
			}
		}
		return fmt.Sprintf("%s %s%s", sym, kindsCol, mid)

	default:
		return fmt.Sprintf("%s %s%s", sym, kindsCol, step.Command)
	}
}

func formatTestCounts(t *quality.TestCounts) string {
	if t == nil {
		return ""
	}
	var parts []string
	if t.Passed > 0 {
		parts = append(parts, fmt.Sprintf("%d passed", t.Passed))
	}
	if t.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", t.Failed))
	}
	if t.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", t.Skipped))
	}
	if len(parts) == 0 {
		if t.Total > 0 {
			return fmt.Sprintf("%d passed", t.Total)
		}
		return ""
	}
	return strings.Join(parts, ", ")
}
