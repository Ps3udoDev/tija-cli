package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// MenuItem is one entry of the main menu. Key identifies the action.
type MenuItem struct {
	Key, Title, Description string
}

const (
	keyInit   = "init"
	keyDoctor = "doctor"
	keyStatus = "status"
)

var (
	itemInit   = MenuItem{Key: keyInit, Title: "Init", Description: "Detect the stack and set up .tija/"}
	itemDoctor = MenuItem{Key: keyDoctor, Title: "Doctor", Description: "Check tools, manifest and agents"}
	itemStatus = MenuItem{Key: keyStatus, Title: "Status", Description: "Branch, commits and working tree"}
)

// MenuItems returns the Phase 1 menu. Init comes first until the repository
// is initialized, then last.
func MenuItems(initialized bool) []MenuItem {
	if initialized {
		return []MenuItem{itemDoctor, itemStatus, itemInit}
	}
	return []MenuItem{itemInit, itemDoctor, itemStatus}
}

// renderMenu lists items with "> " on the selected one. With withDesc each
// title is followed by its description; otherwise only the selected
// item's description is shown under the list.
func renderMenu(items []MenuItem, cursor int, withDesc bool) string {
	titleW := 0
	for _, it := range items {
		titleW = max(titleW, lipgloss.Width(it.Title))
	}
	var lines []string
	for i, it := range items {
		line, desc := "  "+it.Title, mutedStyle.Render(it.Description)
		if i == cursor {
			line, desc = selectedStyle.Render("> "+it.Title), sandStyle.Render(it.Description)
		}
		if withDesc {
			line += strings.Repeat(" ", titleW-lipgloss.Width(it.Title)+3) + desc
		}
		lines = append(lines, line)
	}
	if !withDesc && cursor >= 0 && cursor < len(items) {
		lines = append(lines, "", sandStyle.Width(compactDescWidth).Render(items[cursor].Description))
	}
	return strings.Join(lines, "\n")
}

// compactDescWidth wraps the selected description when it sits under the list.
const compactDescWidth = 24
