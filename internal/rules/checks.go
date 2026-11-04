package rules

import "slices"

type Changed struct {
	Path  string
	Added map[int]string
}
type Violation struct {
	CheckID, Severity, Message, File, Text string
	Line                                   int
}

// Run evaluates checks exclusively against added lines in matching files.
func (s Set) Run(files []Changed) []Violation {
	var out []Violation
	for _, f := range s.Files {
		for _, c := range f.Checks {
			for _, changed := range files {
				if !matches(f.AppliesTo, changed.Path) || !matches(c.Files, changed.Path) {
					continue
				}
				var lines []int
				for line := range changed.Added {
					lines = append(lines, line)
				}
				slices.Sort(lines)
				for _, line := range lines {
					text := changed.Added[line]
					if c.Forbid != nil && c.Forbid.MatchString(text) {
						out = append(out, Violation{CheckID: c.ID, Severity: c.Severity, Message: c.Message, File: normalize(changed.Path), Text: text, Line: line})
					}
				}
			}
		}
	}
	return out
}
