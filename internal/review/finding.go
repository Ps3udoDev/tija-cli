package review

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Severity string

const (
	Critical Severity = "Critical"
	High     Severity = "High"
	Medium   Severity = "Medium"
	Low      Severity = "Low"
)

func ParseSeverity(s string) (Severity, bool) {
	switch strings.ToLower(s) {
	case "critical":
		return Critical, true
	case "high":
		return High, true
	case "medium":
		return Medium, true
	case "low":
		return Low, true
	}
	return "", false
}
func (s Severity) Rank() int {
	switch s {
	case Critical:
		return 4
	case High:
		return 3
	case Medium:
		return 2
	case Low:
		return 1
	}
	return 0
}
func AtLeast(s Severity, failOn string) bool {
	threshold, ok := ParseSeverity(failOn)
	return ok && s.Rank() >= threshold.Rank()
}

type PRComment struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Author string `json:"author"`
	Body   string `json:"body"`
	URL    string `json:"url"`
}
type Finding struct {
	ID          string      `json:"id"`
	Source      string      `json:"source"`
	Passes      []string    `json:"passes"`
	File        string      `json:"file"`
	Line        int         `json:"line"`
	EndLine     int         `json:"endLine"`
	Severity    Severity    `json:"severity"`
	Rule        string      `json:"rule"`
	Title       string      `json:"title"`
	Explanation string      `json:"explanation"`
	Suggestion  string      `json:"suggestion"`
	Evidence    string      `json:"evidence"`
	ConfirmedBy []string    `json:"confirmedBy"`
	Disputed    bool        `json:"disputed"`
	Severities  []Severity  `json:"severities"`
	PRComments  []PRComment `json:"prComments"`
}
type Discarded struct {
	Pass   string `json:"pass"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

const (
	ReasonFileNotInDiff    = "file-not-in-diff"
	ReasonLineOutsideDiff  = "line-outside-diff"
	ReasonEvidenceNotFound = "evidence-not-found"
	ReasonInvalidSeverity  = "invalid-severity"
	ReasonUnknownRule      = "unknown-rule"
)

type AgentFinding struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	EndLine     int    `json:"endLine"`
	Severity    string `json:"severity"`
	Rule        string `json:"rule"`
	Title       string `json:"title"`
	Explanation string `json:"explanation"`
	Suggestion  string `json:"suggestion"`
	Evidence    string `json:"evidence"`
}

// ParseResponse tolerates prose and fences around the JSON object, but demands
// an actual findings array so malformed output can trigger a format retry.
func ParseResponse(out string) ([]AgentFinding, error) {
	start, end := strings.IndexByte(out, '{'), strings.LastIndexByte(out, '}')
	if start < 0 || end < start {
		return nil, fmt.Errorf("respond with a JSON object containing a findings array")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out[start:end+1]), &envelope); err != nil {
		return nil, fmt.Errorf("invalid review JSON: %w", err)
	}
	raw, ok := envelope["findings"]
	if !ok || len(raw) == 0 || strings.TrimSpace(string(raw))[0] != '[' {
		return nil, fmt.Errorf("findings must be a JSON array (use [] for no findings)")
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("invalid findings array: %w", err)
	}
	findings := make([]AgentFinding, 0, len(entries))
	for i, entry := range entries {
		if len(entry) == 0 || entry[0] != '{' {
			return nil, fmt.Errorf("findings[%d] must be an object", i)
		}
		var f AgentFinding
		if err := json.Unmarshal(entry, &f); err != nil {
			return nil, fmt.Errorf("invalid findings[%d]: %w", i, err)
		}
		findings = append(findings, f)
	}
	return findings, nil
}
