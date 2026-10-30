package prrules

import (
	"fmt"
	"regexp"
	"strings"
)

// BuiltinAttributionPatterns matches common AI assistant footers, trailers,
// and co-author tags.
var BuiltinAttributionPatterns = []string{
	`(?i)^\s*🤖.*$`,
	`(?i)^\s*\**\s*generated (with|by) \[?(claude( code)?|codex|openai codex|chatgpt|gemini( cli)?|copilot|cursor|antigravity)\b.*$`,
	`(?i)^\s*co-authored-by:.*(claude|codex|chatgpt|gemini|copilot|cursor|noreply@anthropic\.com|noreply@openai\.com).*$`,
	`(?i)^\s*(made|created|written) (with|by) \[?(claude( code)?|codex|chatgpt|gemini|copilot|cursor)\b.*$`,
}

// AttributionPatterns compiles the builtin attribution patterns plus any extra patterns.
func AttributionPatterns(extra []string) ([]*regexp.Regexp, error) {
	all := make([]string, 0, len(BuiltinAttributionPatterns)+len(extra))
	all = append(all, BuiltinAttributionPatterns...)
	all = append(all, extra...)
	out := make([]*regexp.Regexp, 0, len(all))
	for _, p := range all {
		rx, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid attribution pattern %q: %w", p, err)
		}
		out = append(out, rx)
	}
	return out, nil
}

// StripAttribution strips any lines matching any pattern in pats.
// After stripping, it collapses trailing empty lines.
// Returns the cleaned text and the count of removed lines.
func StripAttribution(text string, pats []*regexp.Regexp) (string, int) {
	if len(pats) == 0 {
		return text, 0
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var kept []string
	removed := 0
	for _, line := range lines {
		matched := false
		for _, pat := range pats {
			if pat.MatchString(line) {
				matched = true
				break
			}
		}
		if matched {
			removed++
		} else {
			kept = append(kept, line)
		}
	}
	// Collapse trailing empty lines
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	return strings.Join(kept, "\n"), removed
}

// CheckAttribution scans body line by line; for any matching line, returns
// Problem{Field: "body", Message: "agent attribution: " + strings.TrimSpace(line)}.
func CheckAttribution(body string, pats []*regexp.Regexp) []Problem {
	var ps []Problem
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		for _, pat := range pats {
			if pat.MatchString(line) {
				ps = append(ps, Problem{
					Field:   "body",
					Message: "agent attribution: " + trimmed,
				})
				break
			}
		}
	}
	return ps
}
