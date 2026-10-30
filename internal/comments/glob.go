package comments

import (
	"path/filepath"
	"regexp"
	"strings"
)

func Match(pattern, path string) bool {
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	subPatterns := expandBraces(pattern)
	for _, p := range subPatterns {
		re, err := globToRegex(p)
		if err != nil {
			continue
		}
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

func expandBraces(pattern string) []string {
	start := strings.IndexByte(pattern, '{')
	if start == -1 {
		return []string{pattern}
	}

	depth := 0
	end := -1
	for i := start; i < len(pattern); i++ {
		if pattern[i] == '{' {
			depth++
		} else if pattern[i] == '}' {
			depth--
			if depth == 0 {
				end = i
				break
			}
		}
	}
	if end == -1 {
		return []string{pattern}
	}

	inner := pattern[start+1 : end]
	var options []string
	optStart := 0
	subDepth := 0
	for i := 0; i < len(inner); i++ {
		if inner[i] == '{' {
			subDepth++
		} else if inner[i] == '}' {
			subDepth--
		} else if inner[i] == ',' && subDepth == 0 {
			options = append(options, inner[optStart:i])
			optStart = i + 1
		}
	}
	options = append(options, inner[optStart:])

	prefix := pattern[:start]
	suffix := pattern[end+1:]

	var result []string
	for _, opt := range options {
		expanded := expandBraces(prefix + opt + suffix)
		result = append(result, expanded...)
	}
	return result
}

func globToRegex(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	i := 0
	n := len(pattern)

	for i < n {
		if i+3 <= n && pattern[i:i+3] == "/**" && (i+3 == n || pattern[i+3] == '/') {
			if i+3 == n {
				b.WriteString("(?:/.*)?")
				i += 3
			} else { // /**/
				b.WriteString("/(?:[^/]+/)*")
				i += 4 // consume "/**/"
			}
			continue
		}
		if i == 0 && i+3 <= n && pattern[i:i+3] == "**/" {
			b.WriteString("(?:.+/)?")
			i += 3
			continue
		}
		if pattern[i] == '*' {
			if i+1 < n && pattern[i+1] == '*' {
				b.WriteString(".*")
				i += 2
			} else {
				b.WriteString("[^/]*")
				i++
			}
			continue
		}
		if pattern[i] == '?' {
			b.WriteString("[^/]")
			i++
			continue
		}

		ch := pattern[i]
		switch ch {
		case '.', '+', '^', '$', '(', ')', '[', ']', '|', '{', '}', '\\':
			b.WriteByte('\\')
			b.WriteByte(ch)
		default:
			b.WriteByte(ch)
		}
		i++
	}

	b.WriteString("$")
	return regexp.Compile(b.String())
}
