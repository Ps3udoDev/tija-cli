package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// asciiCat is the big startup cat of the brand concept 06, with the
// terminal prompt in its mouth.
const asciiCat = `   /\_____/\
  /  0   0  \
 (    >_     )
  \  '---'  /
 .-'       '-.
(__/ |   | \__)`

// asciiCatSmall is the startup cat of the product draft (§31), used when
// the terminal is too narrow for the big one.
const asciiCatSmall = ` /\_/\
( o.o )
 > ^ <`

// logoBlock is the block "TIJA" logo of the brand concept 06.
const logoBlock = `████████ ██      ██  █████
   ██    ██      ██ ██   ██
   ██    ██      ██ ███████
   ██    ██ ██   ██ ██   ██
   ██    ██  █████  ██   ██`

// brandTagline closes the footer when it fits.
const brandTagline = "Small cat. Big workflows."

// brandHeader is the big cat beside the block logo, aligned at the bottom,
// from wideLayout columns; below that, the small cat over "TIJA".
func brandHeader(width int) string {
	if width < wideLayout {
		return catStyle.Render(asciiCatSmall) + "\n\n" + logoStyle.Render("TIJA")
	}
	lines := strings.Split(asciiCat, "\n")
	for i, l := range lines {
		before, after, found := strings.Cut(l, ">_")
		if !found {
			lines[i] = catStyle.Render(l)
			continue
		}
		lines[i] = catStyle.Render(before) + logoStyle.Render(">_") + catStyle.Render(after)
	}
	return lipgloss.JoinHorizontal(lipgloss.Bottom,
		strings.Join(lines, "\n"), "  ", logoStyle.Render(logoBlock))
}
