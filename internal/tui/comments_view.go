package tui

import (
	"bytes"
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/render"
)

type commentsEventMsg engine.CleanEvent

type commentsReportMsg struct {
	report engine.CleanReport
	err    error
	write  bool
}

func waitForCommentsEvent(ch <-chan engine.CleanEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return commentsEventMsg(ev)
	}
}

type commentsFlow struct {
	working       bool
	confirmUnsafe bool
	report        *engine.CleanReport
	events        chan engine.CleanEvent
	err           error
}

func (m model) startComments() (model, tea.Cmd) {
	m.screen = screenComments
	m.title = "Clean comments"
	m.mascot = Scanning
	m.caption = ""
	m.comments = &commentsFlow{
		working: true,
	}
	ch := make(chan engine.CleanEvent, 64)
	m.comments.events = ch
	dir, env := m.dir, m.env
	cmd := func() tea.Msg {
		defer close(ch)
		rep, err := engine.CleanComments(context.Background(), dir, env, engine.CleanRequest{Diff: true}, func(ev engine.CleanEvent) {
			ch <- ev
		})
		return commentsReportMsg{report: rep, err: err, write: false}
	}
	return m, tea.Batch(cmd, waitForCommentsEvent(ch))
}

func (m model) runCleanCommentsWrite(unsafe bool) (model, tea.Cmd) {
	if m.comments == nil {
		m.comments = &commentsFlow{}
	}
	m.comments.working = true
	m.mascot = Scanning
	ch := make(chan engine.CleanEvent, 64)
	m.comments.events = ch
	dir, env := m.dir, m.env
	cmd := func() tea.Msg {
		defer close(ch)
		rep, err := engine.CleanComments(context.Background(), dir, env, engine.CleanRequest{
			Write:  true,
			Unsafe: unsafe,
			Diff:   true,
		}, func(ev engine.CleanEvent) {
			ch <- ev
		})
		return commentsReportMsg{report: rep, err: err, write: true}
	}
	return m, tea.Batch(cmd, waitForCommentsEvent(ch))
}

func (m model) commentsKey(k string) (model, tea.Cmd) {
	if m.comments == nil {
		return m.backToMenu()
	}
	if m.comments.working {
		return m, nil
	}
	if m.comments.confirmUnsafe {
		switch k {
		case "y", "Y":
			m.comments.confirmUnsafe = false
			return m.runCleanCommentsWrite(true)
		default:
			m.comments.confirmUnsafe = false
			return m, nil
		}
	}
	switch k {
	case "w":
		return m.runCleanCommentsWrite(false)
	case "u":
		m.comments.confirmUnsafe = true
		return m, nil
	case "esc", "q":
		return m.backToMenu()
	}
	return m, nil
}

func (m model) commentsBody() string {
	if m.comments == nil {
		return ""
	}
	if m.comments.working && m.comments.report == nil {
		return mutedStyle.Render("Scanning and classifying comments...")
	}
	if m.comments.err != nil {
		return failStyle.Render(m.comments.err.Error())
	}
	if m.comments.report == nil {
		return ""
	}

	var buf bytes.Buffer
	render.CleanComments(render.Printer{W: &buf, Mode: render.Styled}, *m.comments.report)
	out := strings.TrimRight(buf.String(), "\n")

	if m.comments.confirmUnsafe {
		prompt := warnStyle.Render("Apply unsafe changes too? (y/n)")
		out += "\n\n" + prompt
	}
	return out
}
