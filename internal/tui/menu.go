package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/Ps3udoDev/tija-cli/internal/render"
)

// MenuItem is one entry of the main menu. Key identifies the action.
type MenuItem struct {
	Key, Title, Description string
}

const (
	keyPR        = "pr"
	keyComments  = "comments"
	keyCheck     = "check"
	keyTests     = "tests"
	keyWorkflows = "workflows"
	keyInit      = "init"
	keyDoctor    = "doctor"
	keyStatus    = "status"
)

var (
	itemPR        = MenuItem{Key: keyPR, Title: "Pull Request", Description: "Draft and open a pull request"}
	itemComments  = MenuItem{Key: keyComments, Title: "Clean comments", Description: "Remove AI-slop comments, keep the useful ones"}
	itemCheck     = MenuItem{Key: keyCheck, Title: "Check", Description: "Lint, format and tests"}
	itemTests     = MenuItem{Key: keyTests, Title: "Tests", Description: "Run the test suite"}
	itemWorkflows = MenuItem{Key: keyWorkflows, Title: "Workflows", Description: "Run a composed sequence of commands"}
	itemInit      = MenuItem{Key: keyInit, Title: "Init", Description: "Detect the stack and set up .tija/"}
	itemDoctor    = MenuItem{Key: keyDoctor, Title: "Doctor", Description: "Check tools, manifest and agents"}
	itemStatus    = MenuItem{Key: keyStatus, Title: "Status", Description: "Branch, commits and working tree"}
)

// MenuItems returns the menu items. Init comes first until the repository
// is initialized, then last.
func MenuItems(initialized, hasJS, hasWorkflows bool) []MenuItem {
	var items []MenuItem
	if !initialized {
		items = append(items, itemInit)
	}
	items = append(items, itemPR)
	if hasJS {
		items = append(items, itemComments)
	}
	items = append(items, itemCheck, itemTests)
	if hasWorkflows {
		items = append(items, itemWorkflows)
	}
	items = append(items, itemDoctor, itemStatus)
	if initialized {
		items = append(items, itemInit)
	}
	return items
}

// Menu row styles. The blue selection stays visible against the ink canvas.
var (
	itemTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(render.White))
	selBarStyle    = lipgloss.NewStyle().Background(lipgloss.Color(selectionBackground))
	selArrowStyle  = selBarStyle.Foreground(lipgloss.Color(render.Orange)).Bold(true)
	selTitleStyle  = selBarStyle.Foreground(lipgloss.Color(render.White)).Bold(true)
	selDescStyle   = selBarStyle.Foreground(lipgloss.Color(render.Sand))
)

// renderMenu lists items with "> " on the selected one, which is drawn as
// a selection bar. With withDesc each title is followed by its
// description and the bar spans the titles and the widest description;
// otherwise only the selected item's description is shown under the list
// and the bar is as wide as that description.
func renderMenu(items []MenuItem, cursor int, withDesc bool) string {
	titleW, descW := 0, 0
	for _, it := range items {
		titleW = max(titleW, lipgloss.Width(it.Title))
		descW = max(descW, lipgloss.Width(it.Description))
	}
	barW := max(2+titleW, compactDescWidth)
	if withDesc {
		barW = 2 + titleW + 3 + descW
	}
	var lines []string
	for i, it := range items {
		gap := strings.Repeat(" ", titleW-lipgloss.Width(it.Title)+3)
		if i != cursor {
			line := "  " + itemTitleStyle.Render(it.Title)
			if withDesc {
				line += gap + mutedStyle.Render(it.Description)
			}
			lines = append(lines, line)
			continue
		}
		line := selArrowStyle.Render("> ") + selTitleStyle.Render(it.Title)
		w := 2 + lipgloss.Width(it.Title)
		if withDesc {
			line += selBarStyle.Render(gap) + selDescStyle.Render(it.Description)
			w += len(gap) + lipgloss.Width(it.Description)
		}
		if w < barW {
			line += selBarStyle.Render(strings.Repeat(" ", barW-w))
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
