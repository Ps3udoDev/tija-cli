package comments

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

type Edit struct {
	Start, End  int
	Replacement string
}

func PlanEdit(src []byte, c Comment, action, text string) Edit {
	switch action {
	case "rewrite":
		return planRewrite(c, text)
	case "remove":
		return planRemove(src, c)
	default:
		return Edit{Start: c.Start, End: c.End, Replacement: text}
	}
}

func planRewrite(c Comment, text string) Edit {
	var repl string
	switch c.Kind {
	case "line":
		clean := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "//"))
		if clean == "" {
			repl = "//"
		} else {
			repl = "// " + clean
		}
	case "block":
		clean := strings.TrimSpace(text)
		clean = strings.TrimPrefix(clean, "/*")
		clean = strings.TrimSuffix(clean, "*/")
		clean = strings.TrimSpace(clean)
		if clean == "" {
			repl = "/**/"
		} else {
			repl = "/* " + clean + " */"
		}
	case "jsx":
		clean := strings.TrimSpace(text)
		clean = strings.TrimPrefix(clean, "{")
		clean = strings.TrimSuffix(clean, "}")
		clean = strings.TrimSpace(clean)
		clean = strings.TrimPrefix(clean, "/*")
		clean = strings.TrimSuffix(clean, "*/")
		clean = strings.TrimSpace(clean)
		if clean == "" {
			repl = "{/**/}"
		} else {
			repl = "{/* " + clean + " */}"
		}
	default:
		repl = text
	}

	return Edit{
		Start:       c.Start,
		End:         c.End,
		Replacement: repl,
	}
}

func planRemove(src []byte, c Comment) Edit {
	// If JSX and not full line (inline: <a/>{/* x */}<b/>)
	if c.Kind == "jsx" && !c.FullLine {
		return Edit{
			Start:       c.Start,
			End:         c.End,
			Replacement: "",
		}
	}

	// Full line removal (for line, block, or jsx)
	if c.FullLine {
		// Find start of line containing c.Start
		lineStart := 0
		for i := c.Start - 1; i >= 0; i-- {
			if src[i] == '\n' {
				lineStart = i + 1
				break
			}
		}

		// Find end of line containing c.End (including \n)
		lineEnd := len(src)
		for i := c.End; i < len(src); i++ {
			if src[i] == '\n' {
				lineEnd = i + 1
				break
			}
		}

		// Check if removing leaves two consecutive empty lines.
		// If line before lineStart is empty AND line after lineEnd is empty,
		// collapse them into a single empty line by consuming lineEnd to the next line's end.
		prevLineEmpty := false
		if lineStart > 0 {
			// Find start of prev line
			prevLineStart := 0
			// lineStart-1 is '\n'
			prevEnd := lineStart - 1
			if prevEnd > 0 && src[prevEnd-1] == '\r' {
				prevEnd--
			}
			for i := prevEnd - 1; i >= 0; i-- {
				if src[i] == '\n' {
					prevLineStart = i + 1
					break
				}
			}
			prevBytes := src[prevLineStart:prevEnd]
			prevLineEmpty = isWhitespaceOnly(prevBytes)
		}

		nextLineEmpty := false
		var nextLineEnd int
		if lineEnd < len(src) {
			nextLineEnd = len(src)
			for i := lineEnd; i < len(src); i++ {
				if src[i] == '\n' {
					nextLineEnd = i + 1
					break
				}
			}
			nextBytes := src[lineEnd:nextLineEnd]
			nextBytesTrimmed := bytes.TrimRight(nextBytes, "\r\n")
			nextLineEmpty = isWhitespaceOnly(nextBytesTrimmed)
		}

		if prevLineEmpty && nextLineEmpty {
			// Collapse into single empty line by eating the next empty line
			lineEnd = nextLineEnd
		}

		return Edit{
			Start:       lineStart,
			End:         lineEnd,
			Replacement: "",
		}
	}

	// Trailing comment after code on same line:
	// remove whitespace before comment + comment span up to newline.
	// Find line start
	lineStart := 0
	for i := c.Start - 1; i >= 0; i-- {
		if src[i] == '\n' {
			lineStart = i + 1
			break
		}
	}

	// Walk backwards from c.Start to find where spaces/tabs began
	editStart := c.Start
	for editStart > lineStart && (src[editStart-1] == ' ' || src[editStart-1] == '\t') {
		editStart--
	}

	// Walk forwards from c.End eating spaces/tabs (stop before \r or \n)
	editEnd := c.End
	for editEnd < len(src) && (src[editEnd] == ' ' || src[editEnd] == '\t') {
		editEnd++
	}

	return Edit{
		Start:       editStart,
		End:         editEnd,
		Replacement: "",
	}
}

func isWhitespaceOnly(b []byte) bool {
	for _, c := range b {
		if c != ' ' && c != '\t' {
			return false
		}
	}
	return true
}

func Apply(src []byte, edits []Edit) ([]byte, error) {
	if len(edits) == 0 {
		return src, nil
	}

	// Copy and validate spans
	sorted := make([]Edit, len(edits))
	copy(sorted, edits)

	for _, e := range sorted {
		if e.Start < 0 || e.End > len(src) || e.Start > e.End {
			return nil, fmt.Errorf("invalid edit span [%d, %d)", e.Start, e.End)
		}
	}

	// Sort ascending by Start, then End
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Start == sorted[j].Start {
			return sorted[i].End < sorted[j].End
		}
		return sorted[i].Start < sorted[j].Start
	})

	// Check overlapping edits
	for i := 1; i < len(sorted); i++ {
		if sorted[i].Start < sorted[i-1].End {
			return nil, fmt.Errorf("overlapping edits: [%d, %d) and [%d, %d)",
				sorted[i-1].Start, sorted[i-1].End, sorted[i].Start, sorted[i].End)
		}
	}

	var buf bytes.Buffer
	buf.Grow(len(src))

	last := 0
	for _, e := range sorted {
		buf.Write(src[last:e.Start])
		buf.WriteString(e.Replacement)
		last = e.End
	}
	buf.Write(src[last:])

	return buf.Bytes(), nil
}
