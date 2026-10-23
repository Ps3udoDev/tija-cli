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
