package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// wrapAndSliceResult wraps content to fit inside the panel width,
// and if height is constrained, slices lines using offset.
func wrapAndSliceResult(content string, width, height, offset int) (string, bool, int) {
	availWidth := width - panelBodyIndent - 3
	if availWidth < 20 {
		availWidth = 20
	}
	wrapped := lipgloss.NewStyle().Width(availWidth).Render(content)
	lines := strings.Split(wrapped, "\n")

	if height <= 0 {
		return wrapped, false, 0
	}

	visibleHeight := height - 6
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	needsScroll := len(lines) > visibleHeight
	if !needsScroll {
		return wrapped, false, 0
	}

	maxOffset := len(lines) - visibleHeight
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}

	end := offset + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}

	sliced := strings.Join(lines[offset:end], "\n")
	return sliced, true, maxOffset
}
