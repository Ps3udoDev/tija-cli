package review

import (
	"path"
	"sort"
)

// Chunks keeps directory groups together when they fit, splitting larger groups
// only at file boundaries. A single oversized file gets its own chunk.
func Chunks(files []FileDiff, maxLines int) [][]FileDiff {
	if maxLines <= 0 {
		maxLines = 3000
	}
	sorted := make([]FileDiff, 0, len(files))
	for _, f := range files {
		if !f.Deleted && !f.Binary {
			sorted = append(sorted, f)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	var groups [][]FileDiff
	for _, f := range sorted {
		if len(groups) == 0 || path.Dir(groups[len(groups)-1][0].Path) != path.Dir(f.Path) {
			groups = append(groups, nil)
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], f)
	}
	var out [][]FileDiff
	var part []FileDiff
	used := 0
	flush := func() {
		if len(part) > 0 {
			out = append(out, part)
			part = nil
			used = 0
		}
	}
	for _, group := range groups {
		total := 0
		for _, f := range group {
			total += f.ChangedLines()
		}
		if total <= maxLines {
			if used+total > maxLines {
				flush()
			}
			part = append(part, group...)
			used += total
			continue
		}
		flush()
		for _, f := range group {
			n := f.ChangedLines()
			if used+n > maxLines {
				flush()
			}
			part = append(part, f)
			used += n
			if n > maxLines {
				flush()
			}
		}
	}
	flush()
	return out
}
