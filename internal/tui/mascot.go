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

// Pixel grids, 20 columns by 18 rows, drawn after the brand sheet: an orange
// cat sitting three-quarters with a white muzzle, bib and front paws and a
// tail curving up its right side. Two pixel rows make one terminal row
// through half-block characters, so a sprite is 20x9 cells.
//
// Columns 0-13 hold the cat and columns 15-19 the prop of the state; column
// 14 stays empty so the prop never touches the cat. Rows 10-17 (the body)
// are the same in every state: states only change ears, eyes and mouth.
// O orange, K ink, W white, S sand, '.' transparent.
var sprites = map[MascotState][]string{
	// Open eyes and an ellipsis.
	Idle: {
		".O.......O..........",
		".OO.....OO..........",
		".OSO...OSO..........",
		"OOOOOOOOOOO.........",
		"OOOKOOOKOOO.........",
		"OOOKOOOKOOO.........",
		"OOWWWKWWWOO.........",
		".OWWWWWWWO.....S.S.S",
		"..OOOOOOO...........",
		"..OOWWWOO...O.......",
		"..OOWWWOOO..OO......",
		".OOWWWWOOOO..O......",
		".OOWWWWOOOO..O......",
		".OOOWWWOOOO.OO......",
		".OOOWWOOOOOOO.......",
		".OOOOWOOOOOO........",
		"..WW.WW.OOO.........",
		"..WW.WW.OOOO........",
	},
	// Eyes to the right and a magnifier.
	Scanning: {
		".O.......O..........",
		".OO.....OO..........",
		".OSO...OSO.......SS.",
		"OOOOOOOOOOO.....S..S",
		"OOOOKOOOKOO.....S..S",
		"OOOOKOOOKOO......SS.",
		"OOWWWKWWWOO.....S...",
		".OWWWWWWWO.....S....",
		"..OOOOOOO...........",
		"..OOWWWOO...O.......",
		"..OOWWWOOO..OO......",
		".OOWWWWOOOO..O......",
		".OOWWWWOOOO..O......",
		".OOOWWWOOOO.OO......",
		".OOOWWOOOOOOO.......",
		".OOOOWOOOOOO........",
		"..WW.WW.OOO.........",
		"..WW.WW.OOOO........",
	},
	// Closed eyes and thought bubbles rising.
	Thinking: {
		".O.......O........SS",
		".OO.....OO........SS",
		".OSO...OSO..........",
		"OOOOOOOOOOO.........",
		"OOOOOOOOOOO.....SS..",
		"OOKKOOOKKOO.....SS..",
		"OOWWWKWWWOO.........",
		".OWWWWWWWO.....S....",
		"..OOOOOOO...........",
		"..OOWWWOO...O.......",
		"..OOWWWOOO..OO......",
		".OOWWWWOOOO..O......",
		".OOWWWWOOOO..O......",
		".OOOWWWOOOO.OO......",
		".OOOWWOOOOOOO.......",
		".OOOOWOOOOOO........",
		"..WW.WW.OOO.........",
		"..WW.WW.OOOO........",
	},
	// Happy ^ eyes and two sparkles.
	Success: {
		".O.......O..........",
		".OO.....OO......S...",
		".OSO...OSO.....SSS..",
		"OOOOOOOOOOO.....S...",
		"OOKOOOOOKOO.........",
		"OKOKOOOKOKO.........",
		"OOWWWKWWWOO.......S.",
		".OWWWWWWWO.......SSS",
		"..OOOOOOO.........S.",
		"..OOWWWOO...O.......",
		"..OOWWWOOO..OO......",
		".OOWWWWOOOO..O......",
		".OOWWWWOOOO..O......",
		".OOOWWWOOOO.OO......",
		".OOOWWOOOOOOO.......",
		".OOOOWOOOOOO........",
		"..WW.WW.OOO.........",
		"..WW.WW.OOOO........",
	},
	// Alert ears, open mouth and a warning sign.
	Warning: {
		"O.........O.........",
		"OO.......OO......O..",
		"OSO.....OSO......O..",
		"OOOOOOOOOOO.....OKO.",
		"OOOKOOOKOOO.....OKO.",
		"OOOKOOOKOOO....OOOOO",
		"OOWWWKWWWOO....OOKOO",
		".OWWWKWWWO.....OOOOO",
		"..OOOOOOO...........",
		"..OOWWWOO...O.......",
		"..OOWWWOOO..OO......",
		".OOWWWWOOOO..O......",
		".OOWWWWOOOO..O......",
		".OOOWWWOOOO.OO......",
		".OOOWWOOOOOOO.......",
		".OOOOWOOOOOO........",
		"..WW.WW.OOO.........",
		"..WW.WW.OOOO........",
	},
	// X eyes, flat ears and a cross mark.
	Error: {
		"....................",
		"OOO.....OOO.........",
		"OSOOOOOOOSO.....OOO.",
		"OKOKOOOKOKO....OKOKO",
		"OOKOOOOOKOO....OOKOO",
		"OKOKOOOKOKO....OKOKO",
		"OOWWWKWWWOO.....OOO.",
		".OWWWWWWWO..........",
		"..OOOOOOO...........",
		"..OOWWWOO...O.......",
		"..OOWWWOOO..OO......",
		".OOWWWWOOOO..O......",
		".OOWWWWOOOO..O......",
		".OOOWWWOOOO.OO......",
		".OOOWWOOOOOOO.......",
		".OOOOWOOOOOO........",
		"..WW.WW.OOO.........",
		"..WW.WW.OOOO........",
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
	}
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
	}
	return b.String()
}

func pixel(row string, x int) string {
	if x >= len(row) {
		return ""
	}
	return spriteColors[row[x]]
}

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
