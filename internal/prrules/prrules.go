// Package prrules validates pull request titles and bodies locally, exactly
// like the pr-governance GitHub Action does, so drafts pass on the first try.
package prrules

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Ps3udoDev/tija-cli/internal/manifest"
)

// Problem is a single rule violation. Field is "title", "body" or
// "section:<Name>".
type Problem struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Types are the allowed conventional commit types.
var Types = []string{"feat", "fix", "docs", "style", "refactor", "perf", "test", "build", "ci", "chore", "revert"}

var (
	titleRx     = regexp.MustCompile(`^(\w+)(?:\(([\w$.\-*/ ]+)\))?!?: (.+)$`)
	commentRx   = regexp.MustCompile(`<!--[\s\S]*?-->`)
	checkboxRx  = regexp.MustCompile(`(?m)^\s*[-*]\s+\[[ xX]\]\s*`)
	bulletRx    = regexp.MustCompile(`(?m)^\s*[-*]\s*`)
	nextHeading = regexp.MustCompile(`\n##\s+`)
)

// Normalize converts CRLF to LF, trims the whole text and appends a final "\n".
func Normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.TrimSpace(s) + "\n"
}

// MatchTitle reports whether title has the type(scope): subject shape that
// ValidateTitle requires. It does not check the type, scope or length.
func MatchTitle(title string) bool {
	return titleRx.MatchString(title)
}

// ValidateTitle checks a PR title against the conventional commits rules.
func ValidateTitle(title string, r manifest.PullRequest) []Problem {
	var ps []Problem
	add := func(msg string) { ps = append(ps, Problem{Field: "title", Message: msg}) }

	m := titleRx.FindStringSubmatch(title)
	if m == nil {
		add("title must match type(scope): subject")
		return ps
	}
	typ, scope := m[1], m[2]
	if !slices.Contains(Types, typ) {
		add(fmt.Sprintf("type %q is not allowed (want one of %s)", typ, strings.Join(Types, ", ")))
	}
	if r.Title.RequireScope == nil || *r.Title.RequireScope {
		if scope == "" {
			add("scope is required, e.g. feat(scope): subject")
		}
	}
	if max := r.Title.MaxLength; max > 0 && utf8.RuneCountInString(title) > max {
		add(fmt.Sprintf("title is longer than %d characters", max))
	}
	if strings.HasSuffix(title, ".") {
		add("title must not end with a period")
	}
	return ps
}

// SectionContent ports sectionContent from pr-governance.yml. It reports
// whether the "## title" section exists and returns its cleaned content.
func SectionContent(body, title string) (string, bool) {
	body = Normalize(body)
	head := regexp.MustCompile(`(?i)(?:^|\n)##\s+` + regexp.QuoteMeta(title) + `\s*\n`)
	loc := head.FindStringIndex(body)
	if loc == nil {
		return "", false
	}
	rest := body[loc[1]:]
	if n := nextHeading.FindStringIndex(rest); n != nil {
		rest = rest[:n[0]]
	}
	rest = commentRx.ReplaceAllString(rest, "")
	rest = checkboxRx.ReplaceAllString(rest, "")
	rest = bulletRx.ReplaceAllString(rest, "")
	return strings.TrimSpace(rest), true
}

// ValidateBody checks the PR body against the required sections rules.
func ValidateBody(body string, r manifest.PullRequest) []Problem {
	body = Normalize(body)
	var ps []Problem
	if strings.TrimSpace(commentRx.ReplaceAllString(body, "")) == "" {
		ps = append(ps, Problem{Field: "body", Message: "body is empty"})
	}
	for _, name := range r.RequiredSections {
		field := "section:" + name
		content, found := SectionContent(body, name)
		if !found {
			ps = append(ps, Problem{Field: field, Message: "missing section ## " + name})
			continue
		}
		if utf8.RuneCountInString(content) < r.MinSectionLength || isInvalidValue(content, r.InvalidValues) {
			ps = append(ps, Problem{Field: field, Message: fmt.Sprintf("section ## %s needs useful content", name)})
		}
	}
	return ps
}

func isInvalidValue(content string, values []string) bool {
	c := strings.TrimSpace(content)
	for _, v := range values {
		if strings.EqualFold(c, strings.TrimSpace(v)) {
			return true
		}
	}
	return false
}
