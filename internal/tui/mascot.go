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
	Success  MascotState = "success"
	Warning  MascotState = "warning"
	Error    MascotState = "error"
)

// Pixel grids, 16 columns by 16 rows. Two pixel rows make one terminal row
// through half-block characters, so a sprite is 16x8 cells.
// O orange, K ink, W white, S sand, '.' transparent.
var sprites = map[MascotState][]string{
	Idle: {
		"................",
		".O.......O......",
		".OO.....OO......",
		".OSO...OSO......",
		".OOOOOOOOO......",
		"OOOOOOOOOOO.....",
		"OOKOOOOOKOO.....",
		"OOKOOOOOKOO.....",
		"OWWWWKWWWWO.....",
		".WWWWWWWWW......",
		"..OOOOOOO.......",
		".OOWWWWWOO...O..",
		".OOWWWWWOO...O..",
		".OOWWWWWOOO.OO..",
		".OOOOOOOOOOOO...",
		"..WW.WW.WW......",
	},
	// Looking right through a magnifier.
	Scanning: {
		"................",
		".O.......O......",
		".OO.....OO......",
		".OSO...OSO......",
		".OOOOOOOOO.SSS..",
		"OOOOOOOOOOS...S.",
		"OOOKOOOOOKS...S.",
		"OOOKOOOOOKS...S.",
		"OWWWWKWWWWOSSS..",
		".WWWWWWWWW....S.",
		"..OOOOOOO......S",
		".OOWWWWWOO......",
		".OOWWWWWOO......",
		".OOWWWWWOO......",
		".OOOOOOOOO......",
		"..WW.WW.WW......",
	},
	// Happy closed eyes and sparkles.
	Success: {
		"............S...",
		".O.......O.SSS..",
		".OO.....OO..S...",
		".OSO...OSO......",
		".OOOOOOOOO....S.",
		"OOKOOOOOKOO..SSS",
		"OKOKOOOKOKO...S.",
		"OOOOOOOOOOO.....",
		"OWWWWKWWWWO.....",
		".WWWWWWWWW......",
		"..OOOOOOO.......",
		".OOWWWWWOO...O..",
		".OOWWWWWOO...O..",
		".OOWWWWWOOO.OO..",
		".OOOOOOOOOOOO...",
		"..WW.WW.WW......",
	},
	// Alert ears, dot eyes and an exclamation mark.
	Warning: {
		"................",
		".O.......O......",
		".OO.....OO......",
		".OSO...OSO..S...",
		".OOOOOOOOO..S...",
		"OOOOOOOOOOO.S...",
		"OOKOOOOOKOO.....",
		"OOKOOOOOKOO.S...",
		"OWWWWKWWWWO.....",
		".WWWWWWWWW......",
		"..OOOOOOO.......",
		".OOWWWWWOO...O..",
		".OOWWWWWOO...O..",
		".OOWWWWWOOO.OO..",
		".OOOOOOOOOOOO...",
		"..WW.WW.WW......",
	},
	// X eyes, flat ears and a cross mark.
	Error: {
		"................",
		"................",
		"OO.......OO.....",
		"OSOO...OOSO.....",
		".OOOOOOOOO.W...W",
		"OKOKOOOKOKO.W.W.",
		"OOKOOOOOKOO..W..",
		"OKOKOOOKOKO.W.W.",
		"OWWWWKWWWWOW...W",
		".WWWWWWWWW......",
		"..OOOOOOO.......",
		".OOWWWWWOO......",
		".OOWWWWWOO......",
		".OOWWWWWOOO.....",
		".OOOOOOOOOOOO...",
		"..WW.WW.WW......",
	},
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
