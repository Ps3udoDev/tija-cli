// Package review contains the deterministic parts of diff review.
package review

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Hunk struct {
	NewStart, NewLines int
	Added              []int
}
type FileDiff struct {
	Path            string
	Deleted, Binary bool
	Hunks           []Hunk
	Text            string
	Removed         int
}

func normalizedPath(s string) string { return strings.ReplaceAll(s, "\\", "/") }

func gitPath(s string) string {
	s = strings.TrimSuffix(s, "\r")
	if strings.HasPrefix(s, "\"") {
		if v, err := strconv.Unquote(s); err == nil {
			s = v
		}
	}
	return normalizedPath(s)
}

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
var quotedDiffHeader = regexp.MustCompile(`^diff --git (?:"(?:[^"\\]|\\.)*"|\S+) ("(?:[^"\\]|\\.)*"|.+)$`)

// ParseDiff reads unified git diffs using new-side line numbers. Git's quoted
// paths are C-style strings, including octal bytes for UTF-8 filenames.
func ParseDiff(text string) []FileDiff {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var files []FileDiff
	var current *FileDiff
	var body strings.Builder
	active := -1
	newLine := 0
	flush := func() {
		if current != nil {
			current.Text = body.String()
			if current.Deleted {
				current.Hunks = nil
			}
			files = append(files, *current)
		}
		body.Reset()
	}
	lines := strings.SplitAfter(text, "\n")
	for _, raw := range lines {
		if raw == "" {
			continue
		}
		line := strings.TrimSuffix(raw, "\n")
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			current = &FileDiff{}
			active = -1
			if m := quotedDiffHeader.FindStringSubmatch(line); m != nil {
				current.Path = strings.TrimPrefix(gitPath(m[1]), "b/")
			}
		}
		if current == nil {
			continue
		}
		body.WriteString(raw)
		if m := hunkHeader.FindStringSubmatch(line); m != nil {
			start, _ := strconv.Atoi(m[1])
			count := 1
			if m[2] != "" {
				count, _ = strconv.Atoi(m[2])
			}
			current.Hunks = append(current.Hunks, Hunk{NewStart: start, NewLines: count})
			active = len(current.Hunks) - 1
			newLine = start
			continue
		}
		if active >= 0 {
			switch {
			case strings.HasPrefix(line, "+"):
				current.Hunks[active].Added = append(current.Hunks[active].Added, newLine)
				newLine++
			case strings.HasPrefix(line, "-"):
				current.Removed++
			case strings.HasPrefix(line, " "):
				newLine++
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "rename to "):
			current.Path = gitPath(strings.TrimPrefix(line, "rename to "))
		case strings.HasPrefix(line, "+++ "):
			p := strings.TrimPrefix(line, "+++ ")
			if p == "/dev/null" {
				current.Deleted = true
			} else {
				current.Path = strings.TrimPrefix(gitPath(p), "b/")
			}
		case strings.HasPrefix(line, "Binary files "), line == "GIT binary patch":
			current.Binary = true
		case strings.HasPrefix(line, "deleted file mode "):
			current.Deleted = true
		}
	}
	flush()
	return files
}

func (f FileDiff) Touches(line, endLine int) bool {
	if f.Deleted || f.Binary || line < 1 || endLine < line {
		return false
	}
	for _, h := range f.Hunks {
		if h.NewLines > 0 && line <= h.NewStart+h.NewLines-1 && endLine >= h.NewStart {
			return true
		}
	}
	return false
}
func (f FileDiff) AddedLines() []int {
	var out []int
	for _, h := range f.Hunks {
		out = append(out, h.Added...)
	}
	return out
}
func (f FileDiff) ChangedLines() int {
	n := f.Removed
	for _, h := range f.Hunks {
		n += len(h.Added)
	}
	return n
}

func SyntheticAdded(path, content string) FileDiff {
	path = normalizedPath(path)
	content = strings.ReplaceAll(content, "\r\n", "\n")
	var lines []string
	if content != "" {
		lines = strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	}
	h := Hunk{NewStart: 1, NewLines: len(lines)}
	var b strings.Builder
	fmt.Fprintf(&b, "--- /dev/null\n+++ b/%s\n@@ -0,0 +1,%d @@\n", path, len(lines))
	for i, line := range lines {
		h.Added = append(h.Added, i+1)
		fmt.Fprintf(&b, "+%s\n", line)
	}
	return FileDiff{Path: path, Hunks: []Hunk{h}, Text: b.String()}
}
