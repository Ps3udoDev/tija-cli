package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/Ps3udoDev/tija-cli/internal/render"
)

// MascotState is the expression of the pixel cat.
type MascotState string

const (
	Idle     MascotState = "idle"
	Scanning MascotState = "scanning"
	Thinking MascotState = "thinking"
	Success  MascotState = "success"
	Warning  MascotState = "warning"
	Error    MascotState = "error"
)

// A 20 x 24 pixel cat: broad white muzzle, highlighted eyes, white paws
// and a curled tail. Accessories are font glyphs, not square sprite pixels.
var sprites = map[MascotState][]string{
	Idle: {
		"...O.......O........",
		"...OO.....OO........",
		"...OSO...OSO........",
		"...OSOOOOOSO........",
		"..OOOOOOOOOOO.......",
		"..OOOOOOOOOOO.......",
		"..OOKKOOOKKOO.......",
		"..OOKWOOOKWOO.......",
		"..OWKKWWWKKWO.......",
		".OOWWWWSWWWWOO......",
		"..WWWWWKWWWWW.......",
		"...WWWWWWWWW........",
		"....OOWWWOO.........",
		"....OWWWWWOO....OOO.",
		"...OOWWWWWOOO..OO.OO",
		"...OOWWWWWOOO..O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWW.WWOOOOOO....",
		"..OOOWW.WWOOOOO.....",
		"..OOOWW.WWOOOO......",
		"..WWWWW.WWWWWO......",
		"...WWWW.WWWW........",
	},
	Scanning: {
		"...O.......O........",
		"...OO.....OO........",
		"...OSO...OSO........",
		"...OSOOOOOSO........",
		"..OOOOOOOOOOO.......",
		"..OOOOOOOOOOO.......",
		"..OOKKOOOKKOO.......",
		"..OOKWOOOKWOO.......",
		"..OWKKWWWKKWO.......",
		".OOWWWWSWWWWOO......",
		"..WWWWWKWWWWW.......",
		"...WWWWWWWWW........",
		"....OOWWWOO.........",
		"....OWWWWWOO....OOO.",
		"...OOWWWWWOOO..OO.OO",
		"...OOWWWWWOOO..O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWW.WWOOOOOO....",
		"..OOOWW.WWOOOOO.....",
		"..OOOWW.WWOOOO......",
		"..WWWWW.WWWWWO......",
		"...WWWW.WWWW........",
	},
	Thinking: {
		"...O.......O........",
		"...OO.....OO........",
		"...OSO...OSO........",
		"...OSOOOOOSO........",
		"..OOOOOOOOOOO.......",
		"..OOOOOOOOOOO.......",
		"..OOOOOOOOOOO.......",
		"..OKKKOOKKKOO.......",
		"..OWWWWWWWWWO.......",
		".OOWWWWSWWWWOO......",
		"..WWWWWKWWWWW.......",
		"...WWWWWWWWW........",
		"....OOWWWOO.........",
		"....OWWWWWOO....OOO.",
		"...OOWWWWWOOO..OO.OO",
		"...OOWWWWWOOO..O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWW.WWOOOOOO....",
		"..OOOWW.WWOOOOO.....",
		"..OOOWW.WWOOOO......",
		"..WWWWW.WWWWWO......",
		"...WWWW.WWWW........",
	},
	Success: {
		"...O.......O........",
		"...OO.....OO........",
		"...OSO...OSO........",
		"...OSOOOOOSO........",
		"..OOOOOOOOOOO.......",
		"..OOOOOOOOOOO.......",
		"..OOKOOOOKOOO.......",
		"..OKOKOOKOKOO.......",
		"..OWWWWWWWWWO.......",
		".OOWWWWSWWWWOO......",
		"..WWWWWKWWWWW.......",
		"...WWWWWWWWW........",
		"....OOWWWOO.........",
		"....OWWWWWOO....OOO.",
		"...OOWWWWWOOO..OO.OO",
		"...OOWWWWWOOO..O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWW.WWOOOOOO....",
		"..OOOWW.WWOOOOO.....",
		"..OOOWW.WWOOOO......",
		"..WWWWW.WWWWWO......",
		"...WWWW.WWWW........",
	},
	Warning: {
		"...O.......O........",
		"...OO.....OO........",
		"...OSO...OSO........",
		"...OSOOOOOSO........",
		"..OOOOOOOOOOO.......",
		"..OOOOOOOOOOO.......",
		"..OOKKOOOKKOO.......",
		"..OOKWOOOKWOO.......",
		"..OWKKWWWKKWO.......",
		".OOWWWWSWWWWOO......",
		"..WWWWWKWWWWW.......",
		"...WWWWKWWWW........",
		"....OOWWWOO.........",
		"....OWWWWWOO....OOO.",
		"...OOWWWWWOOO..OO.OO",
		"...OOWWWWWOOO..O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWW.WWOOOOOO....",
		"..OOOWW.WWOOOOO.....",
		"..OOOWW.WWOOOO......",
		"..WWWWW.WWWWWO......",
		"...WWWW.WWWW........",
	},
	Error: {
		"...O.......O........",
		"...OO.....OO........",
		"...OSO...OSO........",
		"...OSOOOOOSO........",
		"..OOOOOOOOOOO.......",
		"..OOOOOOOOOOO.......",
		"..OOOOOOOOOOO.......",
		"..OKKKOOKKKOO.......",
		"..OWWWWWWWWWO.......",
		".OOWWWWSWWWWOO......",
		"..WWWWWKWWWWW.......",
		"...WWWKKKWWW........",
		"....OOWWWOO.........",
		"....OWWWWWOO....OOO.",
		"...OOWWWWWOOO..OO.OO",
		"...OOWWWWWOOO..O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWWWWWOOOO.O....",
		"..OOOWW.WWOOOOOO....",
		"..OOOWW.WWOOOOO.....",
		"..OOOWW.WWOOOO......",
		"..WWWWW.WWWWWO......",
		"...WWWW.WWWW........",
	},
}

// captions are the one-line legends drawn under the mascot.
var captions = map[MascotState]string{
	Idle:     "Tija is ready.",
	Scanning: "Scanning repository...",
	Thinking: "Thinking...",
	Success:  "All tasks completed!",
	Warning:  "Needs your review.",
	Error:    "Check failed.",
}

// Caption is the legend shown under the mascot in state s. An unknown state
// has the Idle caption.
func Caption(s MascotState) string {
	if c, ok := captions[s]; ok {
		return c
	}
	return captions[Idle]
}

var spriteColors = map[byte]string{
	'O': render.Orange,
	'K': render.Ink,
	'W': render.White,
	'S': render.Sand,
}

// Sprite renders the pixel cat for state s with half-block characters.
// An unknown state renders as Idle.
func Sprite(s MascotState) string {
	grid, ok := sprites[s]
	if !ok {
		grid = sprites[Idle]
		s = Idle
	}
	props := mascotAccessory(s)
	var b strings.Builder
	for y := 0; y < len(grid); y += 2 {
		if y > 0 {
			b.WriteByte('\n')
		}
		top, bottom := grid[y], ""
		if y+1 < len(grid) {
			bottom = grid[y+1]
		}
		for x := 0; x < len(top); x++ {
			b.WriteString(cell(pixel(top, x), pixel(bottom, x)))
		}
		b.WriteString(" " + props[y/2])
	}
	return b.String()
}

func pixel(row string, x int) string {
	if x >= len(row) {
		return ""
	}
	return spriteColors[row[x]]
}

// Accessories use rounded text glyphs. They occupy a fixed five-cell gutter
// so changing states never shifts the report or menu beside the mascot.
func mascotAccessory(state MascotState) []string {
	lines := make([]string, 12)
	for i := range lines {
		lines[i] = "     "
	}
	put := func(row int, text string) { lines[row] = lipgloss.NewStyle().Width(5).Render(text) }
	switch state {
	case Scanning:
		put(3, catAccessoryStyle.Render("╭─╮"))
		put(4, catAccessoryStyle.Render("╰─╯"))
		put(5, catAccessoryStyle.Render("   ╲"))
	case Thinking:
		put(0, catAccessoryStyle.Render("    ○"))
		put(1, catAccessoryStyle.Render("  °"))
		put(2, catAccessoryStyle.Render("·"))
	case Success:
		put(2, sandStyle.Render("✦"))
		put(4, okStyle.Render("  ✓"))
	case Warning:
		put(3, selectedStyle.Render(" !"))
	case Error:
		put(3, failStyle.Render(" ×"))
	}
	return lines
}

var catAccessoryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(render.White))

// cell draws two stacked pixels in one terminal cell.
func cell(top, bottom string) string {
	switch {
	case top == "" && bottom == "":
		return " "
	case top == bottom:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(top)).Render("█")
	case bottom == "":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(top)).Render("▀")
	case top == "":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(bottom)).Render("▄")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(top)).Background(lipgloss.Color(bottom)).Render("▀")
}
