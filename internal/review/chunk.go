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
	byDirectory := map[string][]FileDiff{}
	var directories []string
	for _, f := range sorted {
		dir := path.Dir(f.Path)
		if _, ok := byDirectory[dir]; !ok {
			directories = append(directories, dir)
		}
		byDirectory[dir] = append(byDirectory[dir], f)
	}
	sort.Strings(directories)
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
	for _, dir := range directories {
		group := byDirectory[dir]
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
