package review

import "strings"

type VerifyInput struct {
	Pass      string
	Files     map[string]FileDiff
	ReadLines func(path string) ([]string, error)
	Rules     map[string]bool
}

// Verify applies the checks in spec order; the first failure determines the
// discard reason. It only reads files that are present in this chunk's diff.
func Verify(in VerifyInput, raw []AgentFinding) ([]Finding, []Discarded) {
	files := make(map[string]FileDiff, len(in.Files))
	for p, f := range in.Files {
		files[normalizedPath(p)] = f
	}
	var findings []Finding
	var discarded []Discarded
	for _, f := range raw {
		f.File = normalizedPath(f.File)
		if f.EndLine == 0 {
			f.EndLine = f.Line
		}
		file, ok := files[f.File]
		reason := ""
		var severity Severity
		switch {
		case !ok:
			reason = ReasonFileNotInDiff
		case !file.Touches(f.Line, f.EndLine):
			reason = ReasonLineOutsideDiff
		case !verifyEvidence(in.ReadLines, f):
			reason = ReasonEvidenceNotFound
		default:
			severity, ok = ParseSeverity(f.Severity)
			if !ok {
				reason = ReasonInvalidSeverity
			} else if !in.Rules[f.Rule] || f.Rule == "1000-lines-ceiling" {
				reason = ReasonUnknownRule
			}
		}
		if reason != "" {
			discarded = append(discarded, Discarded{Pass: in.Pass, File: f.File, Line: f.Line, Reason: reason})
			continue
		}
		findings = append(findings, Finding{Source: "agent", Passes: []string{in.Pass}, File: f.File, Line: f.Line, EndLine: f.EndLine, Severity: severity, Rule: f.Rule, Title: f.Title, Explanation: f.Explanation, Suggestion: f.Suggestion, Evidence: f.Evidence})
	}
	return findings, discarded
}

func verifyEvidence(read func(string) ([]string, error), f AgentFinding) bool {
	if read == nil || strings.TrimSpace(f.Evidence) == "" {
		return false
	}
	evidence := strings.Split(strings.TrimSuffix(strings.ReplaceAll(f.Evidence, "\r\n", "\n"), "\n"), "\n")
	if len(evidence) > 5 {
		return false
	}
	lines, err := read(f.File)
	if err != nil {
		return false
	}
	low := max(1, f.Line-3)
	high := len(lines)
	// Avoid overflow when malformed agent output contains a huge endLine.
	if f.EndLine <= high-3 {
		high = f.EndLine + 3
	}
	if low > high {
		return false
	}
	normalize := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	for start := low - 1; start+len(evidence) <= high; start++ {
		match := true
		for i, s := range evidence {
			if normalize(s) != normalize(lines[start+i]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
