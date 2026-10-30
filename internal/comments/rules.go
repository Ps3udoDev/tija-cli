package comments

import (
	"bytes"
	"strings"
	"unicode"
)

type Verdict struct {
	Class  string
	Action string
}

func Rule(c Comment, src []byte) (Verdict, bool) {
	// Rule 1: separator
	if isSeparator(c) {
		return Verdict{Class: "separator", Action: "remove"}, true
	}

	// Rule 2: end-marker
	if isEndMarker(c, src) {
		return Verdict{Class: "end-marker", Action: "remove"}, true
	}

	// Rule 3: empty-line
	if isEmptyLineComment(c) {
		return Verdict{Class: "empty-line", Action: "remove"}, true
	}

	return Verdict{}, false
}

func isSeparator(c Comment) bool {
	raw := c.Text
	var inner string
	switch c.Kind {
	case "line":
		inner = strings.TrimPrefix(raw, "//")
	case "block":
		inner = strings.TrimPrefix(raw, "/*")
		inner = strings.TrimSuffix(inner, "*/")
	case "jsx":
		inner = strings.TrimPrefix(raw, "{")
		inner = strings.TrimSuffix(inner, "}")
		inner = strings.TrimSpace(inner)
		inner = strings.TrimPrefix(inner, "/*")
		inner = strings.TrimSuffix(inner, "*/")
	default:
		return false
	}

	trimmed := strings.TrimSpace(inner)
	if trimmed == "" {
		return false
	}

	// Must have only punctuation characters from "-=*#_~/" (and optional spaces)
	// Must not have letters or digits
	punctCount := 0
	for _, r := range trimmed {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
		if strings.ContainsRune("-=*#_~/", r) {
			punctCount++
		} else if !unicode.IsSpace(r) {
			return false
		}
	}

	return punctCount >= 3
}

func isEndMarker(c Comment, src []byte) bool {
	if c.Kind != "line" {
		return false
	}

	// Line must have closing brace '}' before c.Start
	lineStart := 0
	for i := c.Start - 1; i >= 0; i-- {
		if src[i] == '\n' {
			lineStart = i + 1
			break
		}
	}

	beforeComment := src[lineStart:c.Start]
	if !bytes.ContainsRune(beforeComment, '}') {
		return false
	}

	trimmed := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
	lower := strings.ToLower(trimmed)

	return lower == "end" || strings.HasPrefix(lower, "end ") || strings.HasPrefix(lower, "end:") || strings.HasPrefix(lower, "end of ")
}

func isEmptyLineComment(c Comment) bool {
	if c.Kind != "line" || !c.FullLine {
		return false
	}

	trimmed := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
	if trimmed != "" {
		return false
	}

	hasComment := func(line string) bool {
		t := strings.TrimSpace(line)
		return strings.Contains(t, "//") || strings.Contains(t, "/*") || strings.Contains(t, "*/") || strings.HasPrefix(t, "*")
	}

	if len(c.Before) > 0 {
		if hasComment(c.Before[len(c.Before)-1]) {
			return false
		}
	}

	if len(c.After) > 0 {
		if hasComment(c.After[0]) {
			return false
		}
	}

	return true
}
