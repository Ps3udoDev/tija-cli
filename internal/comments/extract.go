package comments

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type Comment struct {
	ID         string // "<path>#<offset>" (offset is Start)
	Path       string
	Kind       string // "line" | "block" | "jsx"
	Line, Col  int    // 1-based
	Text       string
	FullLine   bool     // only whitespace before and after on its line(s)
	Before     []string // up to 3 lines of preceding context
	After      []string // up to 3 lines of following context
	Start, End int      // byte span (for "jsx", outer brace span)
}

func Extract(path string, src []byte, lang Lang) ([]Comment, error) {
	tokens, err := Lex(src, lang)
	if err != nil {
		return nil, err
	}

	// Index lines
	var lineStarts []int
	lineStarts = append(lineStarts, 0)
	for i, b := range src {
		if b == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}

	findLine := func(offset int) int {
		// Binary search for line index (0-based)
		low, high := 0, len(lineStarts)-1
		ans := 0
		for low <= high {
			mid := (low + high) / 2
			if lineStarts[mid] <= offset {
				ans = mid
				low = mid + 1
			} else {
				high = mid - 1
			}
		}
		return ans
	}

	getLineString := func(lineIdx int) string {
		start := lineStarts[lineIdx]
		var end int
		if lineIdx+1 < len(lineStarts) {
			end = lineStarts[lineIdx+1]
		} else {
			end = len(src)
		}
		lineBytes := src[start:end]
		// Trim trailing \r\n or \n
		lineBytes = bytes.TrimRight(lineBytes, "\r\n")
		return string(lineBytes)
	}

	slashPath := filepath.ToSlash(path)
	var result []Comment

	for _, t := range tokens {
		if t.Kind != TokenLineComment && t.Kind != TokenBlockComment {
			continue
		}

		var kind string
		var start, end int
		if t.InJSXChild {
			kind = "jsx"
			start = t.OuterStart
			end = t.OuterEnd
		} else if t.Kind == TokenLineComment {
			kind = "line"
			start = t.Start
			end = t.End
		} else {
			kind = "block"
			start = t.Start
			end = t.End
		}

		startLineIdx := findLine(start)
		endLineIdx := findLine(end)

		line := startLineIdx + 1
		col := start - lineStarts[startLineIdx] + 1

		// FullLine: only spaces/tabs before start on startLine, and only spaces/tabs followed by \r?\n or EOF after end on endLine
		beforeStartBytes := src[lineStarts[startLineIdx]:start]
		onlyWhitespaceBefore := true
		for _, b := range beforeStartBytes {
			if b != ' ' && b != '\t' {
				onlyWhitespaceBefore = false
				break
			}
		}

		var lineEnd int
		if endLineIdx+1 < len(lineStarts) {
			lineEnd = lineStarts[endLineIdx+1]
		} else {
			lineEnd = len(src)
		}
		afterEndBytes := src[end:lineEnd]
		onlyWhitespaceAfter := true
		for _, b := range bytes.TrimRight(afterEndBytes, "\r\n") {
			if b != ' ' && b != '\t' {
				onlyWhitespaceAfter = false
				break
			}
		}

		fullLine := onlyWhitespaceBefore && onlyWhitespaceAfter

		// Context lines Before (up to 3)
		var before []string
		bStart := startLineIdx - 3
		if bStart < 0 {
			bStart = 0
		}
		for l := bStart; l < startLineIdx; l++ {
			before = append(before, getLineString(l))
		}
		if before == nil {
			before = []string{}
		}

		// Context lines After (up to 3)
		var after []string
		aEnd := endLineIdx + 4
		if aEnd > len(lineStarts) {
			aEnd = len(lineStarts)
		}
		for l := endLineIdx + 1; l < aEnd; l++ {
			after = append(after, getLineString(l))
		}
		if after == nil {
			after = []string{}
		}

		id := fmt.Sprintf("%s#%d", slashPath, start)
		text := string(src[start:end])

		result = append(result, Comment{
			ID:       id,
			Path:     slashPath,
			Kind:     kind,
			Line:     line,
			Col:      col,
			Text:     text,
			FullLine: fullLine,
			Before:   before,
			After:    after,
			Start:    start,
			End:      end,
		})
	}

	return result, nil
}

var (
	reWebpackMagic = regexp.MustCompile(`\bwebpack[A-Z]\w*`)
	reJSDocTypes   = regexp.MustCompile(`@(?:type|typedef|param|returns?|template|satisfies)\b`)
	reJSXAnnot     = regexp.MustCompile(`@jsx(?:ImportSource)?\b`)
)

func Protected(c Comment, ext string, extra []*regexp.Regexp) bool {
	text := c.Text
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)

	// Shebang
	if strings.HasPrefix(trimmed, "#!") {
		return true
	}

	// Triple slash
	if strings.HasPrefix(trimmed, "/// <reference") || strings.HasPrefix(trimmed, "///<reference") {
		return true
	}

	// Licenses
	if strings.HasPrefix(trimmed, "/*!") || strings.HasPrefix(strings.TrimPrefix(trimmed, "{"), "/*!") {
		return true
	}
	if strings.Contains(lower, "@license") || strings.Contains(lower, "@preserve") || strings.Contains(lower, "@copyright") {
		return true
	}

	// Directives
	directives := []string{
		"biome-ignore",
		"eslint-disable",
		"eslint-enable",
		"prettier-ignore",
		"@ts-ignore",
		"@ts-expect-error",
		"@ts-nocheck",
		"@ts-check",
		"istanbul ignore",
		"c8 ignore",
		"v8 ignore",
		"@vite-ignore",
	}
	for _, d := range directives {
		if strings.Contains(text, d) {
			return true
		}
	}

	// Webpack magic comments
	if reWebpackMagic.MatchString(text) {
		return true
	}

	// Pure annotations
	if strings.Contains(text, "#__PURE__") || strings.Contains(text, "@__PURE__") {
		return true
	}

	// JSX annotations
	if reJSXAnnot.MatchString(text) {
		return true
	}

	// Flow
	if strings.Contains(text, "@flow") {
		return true
	}

	// JSDoc types in JS (.js or .jsx, not .ts, .tsx, .mts, .cts)
	normalizedExt := strings.ToLower(ext)
	if idx := strings.LastIndex(normalizedExt, "."); idx >= 0 {
		normalizedExt = normalizedExt[idx:]
	} else if normalizedExt != "" {
		normalizedExt = "." + normalizedExt
	}
	if normalizedExt == ".js" || normalizedExt == ".jsx" {
		if reJSDocTypes.MatchString(text) {
			return true
		}
	}

	// Extra regex patterns
	for _, r := range extra {
		if r != nil && r.MatchString(text) {
			return true
		}
	}

	return false
}
