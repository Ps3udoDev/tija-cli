// The parallel review and synthesis follow the idea of the `thermos` skill by
// Cursor (https://github.com/cursor/plugins, MIT), reimplemented in Go.
package review

import (
	"fmt"
	"sort"
)

// Merge joins overlapping agent findings of the same rule. Sorting first makes
// the result independent of parallel call completion order. Range unions also
// join transitive overlaps, even when the bridging finding arrives last.
func Merge(findings []Finding) []Finding {
	var agents, other []Finding
	for _, f := range findings {
		if f.Source == "agent" {
			agents = append(agents, f)
		} else {
			other = append(other, f)
		}
	}
	sort.SliceStable(agents, func(i, j int) bool {
		a, b := agents[i], agents[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.EndLine != b.EndLine {
			return a.EndLine < b.EndLine
		}
		if a.Severity != b.Severity {
			return a.Severity.Rank() > b.Severity.Rank()
		}
		if a.Title != b.Title {
			return a.Title < b.Title
		}
		return a.Explanation < b.Explanation
	})
	for i := 0; i < len(agents); {
		group := []Finding{agents[i]}
		end := max(agents[i].Line, agents[i].EndLine)
		j := i + 1
		for j < len(agents) && agents[j].File == agents[i].File && agents[j].Rule == agents[i].Rule && agents[j].Line <= end {
			group = append(group, agents[j])
			end = max(end, agents[j].EndLine)
			j++
		}
		chosen := group[0]
		passes := map[string]bool{}
		severities := map[Severity]bool{}
		for _, f := range group {
			for _, p := range f.Passes {
				passes[p] = true
			}
			for _, p := range f.ConfirmedBy {
				passes[p] = true
			}
			severities[f.Severity] = true
			for _, s := range f.Severities {
				severities[s] = true
			}
			if f.Severity.Rank() > chosen.Severity.Rank() {
				chosen = f
			}
		}
		chosen.Line = group[0].Line
		chosen.EndLine = end
		chosen.Passes = nil
		chosen.ConfirmedBy = nil
		chosen.Severities = nil
		for p := range passes {
			chosen.Passes = append(chosen.Passes, p)
		}
		sort.Strings(chosen.Passes)
		if len(chosen.Passes) > 1 {
			chosen.ConfirmedBy = append([]string(nil), chosen.Passes...)
		}
		chosen.Disputed = len(severities) > 1
		if chosen.Disputed {
			for s := range severities {
				chosen.Severities = append(chosen.Severities, s)
			}
			sort.Slice(chosen.Severities, func(i, j int) bool { return chosen.Severities[i].Rank() > chosen.Severities[j].Rank() })
		}
		other = append(other, chosen)
		i = j
	}
	return other
}

func Sort(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Severity != b.Severity {
			return a.Severity.Rank() > b.Severity.Rank()
		}
		if len(a.ConfirmedBy) != len(b.ConfirmedBy) {
			return len(a.ConfirmedBy) > len(b.ConfirmedBy)
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.Title < b.Title
	})
	for i := range findings {
		findings[i].ID = fmt.Sprintf("F%d", i+1)
	}
}
