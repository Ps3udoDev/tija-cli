package review

import "fmt"

type LineCount struct {
	Path          string
	Before, After int
}

func Ceiling(counts []LineCount) []Finding {
	var out []Finding
	for _, c := range counts {
		if c.Before <= 1000 && c.After > 1000 {
			out = append(out, Finding{Source: "check", Rule: "1000-lines-ceiling", Severity: Medium, File: normalizedPath(c.Path), Line: 1001, EndLine: 1001, Title: "File exceeds 1000 lines", Explanation: fmt.Sprintf("The file grew from %d to %d lines, crossing the 1000-line ceiling.", c.Before, c.After), Suggestion: "Split the file along cohesive responsibilities."})
		}
	}
	return out
}
