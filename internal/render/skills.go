package render

import (
	"fmt"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/skills"
)

// Skills renders a list of skill packs as a table or JSON.
func Skills(p Printer, packs []skills.Pack) {
	if p.Mode == JSON {
		if packs == nil {
			packs = []skills.Pack{}
		}
		_ = p.JSONOut(packs)
		return
	}

	headers := []string{"NAME", "ORIGIN", "AUTHOR", "LICENSE", "COMMIT", "EDITED"}
	rows := make([][]string, len(packs))
	for i, pack := range packs {
		author := "-"
		license := "-"
		commit := "-"
		if pack.Origin == "third-party" && pack.Source != nil {
			if pack.Source.Author != "" {
				author = pack.Source.Author
			}
			if pack.Source.License != "" {
				license = pack.Source.License
			}
			if pack.Source.Commit != "" {
				commit = pack.Source.Commit
				if len(commit) > 7 {
					commit = commit[:7]
				}
			}
		} else if pack.Source != nil {
			if pack.Source.Author != "" {
				author = pack.Source.Author
			}
			if pack.Source.License != "" {
				license = pack.Source.License
			}
		}

		edited := "no"
		if pack.Edited {
			edited = "yes"
		}

		rows[i] = []string{pack.Name, pack.Origin, author, license, commit, edited}
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	formatRow := func(cols []string) string {
		var parts []string
		for i, col := range cols {
			if i == len(cols)-1 {
				parts = append(parts, col)
			} else {
				parts = append(parts, fmt.Sprintf("%-*s", widths[i], col))
			}
		}
		return strings.Join(parts, "  ")
	}

	headerLine := formatRow(headers)
	if p.Mode == Styled {
		headerLine = headingStyle.Render(headerLine)
	}
	p.Line(headerLine)

	for _, row := range rows {
		p.Line(formatRow(row))
	}
}

// SkillShow renders a single skill pack's origin and metadata, followed by its body.
func SkillShow(p Printer, pack skills.Pack) {
	if p.Mode == JSON {
		_ = p.JSONOut(pack)
		return
	}

	p.Field("Skill", pack.Name)
	p.Field("Origin", pack.Origin)
	if pack.Source != nil {
		if pack.Source.Author != "" {
			p.Field("Author", pack.Source.Author)
		}
		if pack.Source.License != "" {
			p.Field("License", pack.Source.License)
		}
		if pack.Source.Commit != "" {
			c := pack.Source.Commit
			if len(c) > 7 {
				c = c[:7]
			}
			p.Field("Commit", c)
		}
	}
	p.Field("Path", pack.Path)
	edited := "no"
	if pack.Edited {
		edited = "yes"
	}
	p.Field("Edited", edited)
	p.Line("")
	p.Line(strings.TrimRight(pack.Body, "\r\n"))
}
