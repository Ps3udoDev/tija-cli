package tui

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/render"
	"github.com/Ps3udoDev/tija-cli/internal/review"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

// ReviewRunner allows the interactive flow to use the same engine as the CLI.
type ReviewRunner func(context.Context, string, sys.Env, engine.ReviewRequest, func(engine.ReviewEvent)) (engine.ReviewReport, error)

func WithReviewRunner(runner ReviewRunner) Option { return func(m *model) { m.reviewRunner = runner } }

var reviewModes = []MenuItem{
	{Key: "review", Title: "Review", Description: "Find bugs, security issues and regressions"},
	{Key: "quality", Title: "Quality", Description: "Review design and code quality"},
	{Key: "deep", Title: "Deep", Description: "Run both passes and combine verified findings"},
}

type reviewFlow struct {
	cursor  int
	mode    string
	working bool
	detail  bool
	event   engine.ReviewEvent
	report  *engine.ReviewReport
	err     error
	events  chan engine.ReviewEvent
}
type reviewEventMsg engine.ReviewEvent
type reviewDoneMsg struct {
	report engine.ReviewReport
	err    error
}

func waitForReviewEvent(ch <-chan engine.ReviewEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return reviewEventMsg(ev)
	}
}
func (m model) startReview() (model, tea.Cmd) {
	m.screen, m.title, m.mascot, m.caption = screenReview, "Review", Idle, ""
	m.review = &reviewFlow{}
	m.scrollOffset = 0
	return m, nil
}
func (m model) runReview(mode string) (model, tea.Cmd) {
	r := m.review
	r.mode, r.working = mode, true
	m.mascot = Thinking
	r.events = make(chan engine.ReviewEvent, 32)
	runner := m.reviewRunner
	if runner == nil {
		runner = engine.RunReview
	}
	dir, env, ch := m.dir, m.env, r.events
	cmd := func() tea.Msg {
		defer close(ch)
		rep, err := runner(context.Background(), dir, env, engine.ReviewRequest{Mode: mode}, func(ev engine.ReviewEvent) { ch <- ev })
		return reviewDoneMsg{rep, err}
	}
	return m, tea.Batch(cmd, waitForReviewEvent(ch))
}
func (m model) updateReview(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.review == nil {
		return m, nil
	}
	r := m.review
	switch msg := msg.(type) {
	case reviewEventMsg:
		r.event = engine.ReviewEvent(msg)
		if r.working {
			return m, waitForReviewEvent(r.events)
		}
	case reviewDoneMsg:
		r.working = false
		r.report = &msg.report
		r.err = msg.err
		r.cursor = 0
		m.scrollOffset = 0
		review.Sort(r.report.Findings)
		m.mascot = Success
		if msg.report.ExitCode() == 1 {
			m.mascot = Warning
		}
		if msg.err != nil || msg.report.ExitCode() == 2 {
			m.mascot = Error
		}
	}
	return m, nil
}
func (m model) reviewKey(k string) (model, tea.Cmd) {
	r := m.review
	if r == nil {
		return m.backToMenu()
	}
	if r.working {
		return m, nil
	}
	if k == "q" {
		return m.backToMenu()
	}
	if r.report == nil && r.err == nil {
		switch k {
		case "up", "k":
			r.cursor = max(0, r.cursor-1)
		case "down", "j":
			r.cursor = min(len(reviewModes)-1, r.cursor+1)
		case "enter":
			return m.runReview(reviewModes[r.cursor].Key)
		case "esc":
			return m.backToMenu()
		}
		return m, nil
	}
	if r.detail {
		if k == "esc" || k == "enter" {
			r.detail = false
			m.scrollOffset = 0
			return m, nil
		}
		m.scroll(k, m.reviewBody(), m.height)
		return m, nil
	}
	switch k {
	case "esc":
		return m.backToMenu()
	case "up", "k":
		r.cursor = max(0, r.cursor-1)
		m.revealReviewSelection()
	case "down", "j":
		if r.report != nil {
			r.cursor = min(max(0, len(r.report.Findings)-1), r.cursor+1)
			m.revealReviewSelection()
		}
	case "enter":
		if r.report != nil && len(r.report.Findings) > 0 {
			r.detail = true
			m.scrollOffset = 0
		}
	default:
		m.scroll(k, m.reviewBody(), m.height)
	}
	return m, nil
}

// Keep the selected row visible even in a short terminal with many findings.
func (m *model) revealReviewSelection() {
	content := m.reviewBody()
	wrapped, _, _ := wrapAndSliceResult(content, m.width, 0, 0)
	lines := strings.Split(wrapped, "\n")
	selected := 0
	for i, line := range lines {
		if strings.HasPrefix(line, "> ") {
			selected = i
			break
		}
	}
	visible := max(1, m.height-6)
	if selected < m.scrollOffset {
		m.scrollOffset = selected
	}
	if selected >= m.scrollOffset+visible {
		m.scrollOffset = selected - visible + 1
	}
}
func (m model) reviewBody() string {
	r := m.review
	if r == nil {
		return ""
	}
	if r.working {
		ev := r.event
		progress := fmt.Sprintf("%s: %s", ev.Stage, ev.Message)
		if ev.Pass != "" || ev.Chunks > 0 {
			progress += fmt.Sprintf("\nPass: %s · chunk %d/%d", ev.Pass, ev.Chunk, ev.Chunks)
		}
		if ev.Quality != nil {
			progress += fmt.Sprintf("\nQuality %d/%d: %s · %s", ev.Quality.Index+1, ev.Quality.Total, ev.Quality.Step.Command, ev.Quality.Step.Status)
		}
		return "Reviewing (" + r.mode + ")...\n\n" + progress
	}
	if r.report == nil && r.err == nil {
		return "Which mode?\n\n" + renderMenu(reviewModes, r.cursor, true)
	}
	if r.detail && r.report != nil && r.cursor < len(r.report.Findings) {
		f := r.report.Findings[r.cursor]
		location := f.File
		if f.Line > 0 {
			location += fmt.Sprintf(":%d", f.Line)
		}
		return fmt.Sprintf("%s · %s · %s\n%s\n\nExplanation\n%s\n\nSuggestion\n%s\n\nEvidence\n%s", f.Severity, location, f.Rule, f.Title, f.Explanation, f.Suggestion, f.Evidence)
	}
	var lines []string
	if r.err != nil {
		lines = append(lines, "Error: "+r.err.Error(), "")
	}
	if r.report == nil {
		return strings.Join(lines, "\n")
	}
	if len(r.report.Findings) == 0 {
		lines = append(lines, "No verified findings.")
	}
	last := review.Severity("")
	for i, f := range r.report.Findings {
		if f.Severity != last {
			lines = append(lines, "", string(f.Severity))
			last = f.Severity
		}
		prefix := "  "
		if i == r.cursor {
			prefix = "> "
		}
		location := f.File
		if f.Line > 0 {
			location += fmt.Sprintf(":%d", f.Line)
		}
		lines = append(lines, prefix+location+" · "+f.Rule+" · "+f.Title)
	}
	var buf bytes.Buffer
	render.Review(render.Printer{W: &buf, Mode: render.Plain}, *r.report)
	// The standard renderer supplies the stable summary, warnings and uncovered PR comments.
	rendered := buf.String()
	if at := strings.Index(rendered, "Summary:"); at >= 0 {
		lines = append(lines, "", strings.TrimSpace(rendered[at:]))
	}
	return strings.Join(lines, "\n")
}
