package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/render"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type spinnerTickMsg time.Time

func tickSpinner() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTickMsg(t)
	})
}

type qualityEventMsg engine.QualityEvent

func waitForEvent(ch <-chan engine.QualityEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return qualityEventMsg(ev)
	}
}

type qualityStepState struct {
	status   string // "pending", "running", "passed", "failed", "skipped", "unavailable"
	command  string
	duration time.Duration
}

func (m model) qualityView() string {
	if len(m.qualitySteps) == 0 {
		return mutedStyle.Render("Preparing checks...")
	}
	var lines []string
	for _, s := range m.qualitySteps {
		var sym string
		switch s.status {
		case "running":
			frame := spinnerFrames[m.spinnerIndex%len(spinnerFrames)]
			sym = selectedStyle.Render(frame)
		case "passed":
			sym = okStyle.Render("✓")
		case "failed":
			sym = failStyle.Render("✗")
		case "skipped":
			sym = sandStyle.Render("-")
		case "unavailable":
			sym = warnStyle.Render("!")
		default: // "pending"
			sym = mutedStyle.Render("·")
		}

		cmd := s.command
		if cmd == "" {
			cmd = "..."
		}
		line := sym + " " + cmd
		if s.duration > 0 {
			line += "  " + render.FormatDuration(s.duration)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// applyQualityEvent records a quality event in the live step list.
func (m *model) applyQualityEvent(ev engine.QualityEvent) {
	if ev.Total > len(m.qualitySteps) {
		steps := make([]qualityStepState, ev.Total)
		for i := range steps {
			steps[i].status = "pending"
		}
		copy(steps, m.qualitySteps)
		m.qualitySteps = steps
	}
	if ev.Index < 0 || ev.Index >= len(m.qualitySteps) {
		return
	}
	st := &m.qualitySteps[ev.Index]
	if ev.Step.Command != "" {
		st.command = ev.Step.Command
	}
	switch ev.Type {
	case engine.EventStart:
		st.status = "running"
	case engine.EventDone:
		st.status = string(ev.Step.Status)
		st.duration = time.Duration(ev.Step.DurationMs) * time.Millisecond
	}
}
