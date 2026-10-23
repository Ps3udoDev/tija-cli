package agents

import (
	"errors"
	"regexp"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/prrules"
)

// ErrUnparseable means the agent output contains no recognisable draft.
var ErrUnparseable = errors.New("agent output has no title and body")

// Draft is a pull request title and body proposed by an agent.
type Draft struct {
	Title, Body string
}

var (
	ansiRx  = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	titleBl = regexp.MustCompile(`(?s)<<<TITLE(.*?)TITLE>>>`)
	bodyBl  = regexp.MustCompile(`(?s)<<<BODY(.*?)BODY>>>`)
)

// ParseDraft extracts the title and body from an agent answer of the form
//
//	<<<TITLE
//	type(scope): subject
//	TITLE>>>
//	<<<BODY
//	## Section
//	BODY>>>
//
// It strips ANSI sequences, code fences around each block, text outside the
// markers and quotes around the title. Without markers it accepts a
// conventional title line followed by a "## " heading.
func ParseDraft(out string) (Draft, error) {
	s := ansiRx.ReplaceAllString(strings.ReplaceAll(out, "\r\n", "\n"), "")

	if m := titleBl.FindStringSubmatch(s); m != nil {
		title := cleanTitle(firstLine(stripFences(m[1])))
		if title == "" {
			return Draft{}, ErrUnparseable
		}
		var body string
		if b := bodyBl.FindStringSubmatch(s); b != nil {
			body = stripFences(b[1])
		}
		return Draft{Title: title, Body: body}, nil
	}

	lines := strings.Split(s, "\n")
	for i, l := range lines {
		title := cleanTitle(l)
		if !prrules.MatchTitle(title) {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			next := strings.TrimSpace(lines[j])
			if next == "" {
				continue
			}
			if strings.HasPrefix(next, "## ") {
				return Draft{Title: title, Body: stripFences(strings.Join(lines[j:], "\n"))}, nil
			}
			break
		}
	}
	return Draft{}, ErrUnparseable
}

// stripFences trims s and removes a ``` fence line at its start and end.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s // a trailing fence then closes a code block inside the body
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	} else {
		s = ""
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s)
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

// cleanTitle trims whitespace and wrapping quotes or backticks.
func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	for len(s) >= 2 {
		f, l := s[0], s[len(s)-1]
		if (f == '"' || f == '\'' || f == '`') && f == l {
			s = strings.TrimSpace(s[1 : len(s)-1])
			continue
		}
		break
	}
	return s
}
