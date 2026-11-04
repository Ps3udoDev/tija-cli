package review

// AttachPRComments only adds context; it never changes severity or thresholds.
func AttachPRComments(findings []Finding, comments []PRComment) ([]Finding, []PRComment) {
	out := append([]Finding(nil), findings...)
	for i := range out {
		out[i].PRComments = append([]PRComment(nil), out[i].PRComments...)
	}
	var uncovered []PRComment
	for _, c := range comments {
		c.Path = normalizedPath(c.Path)
		covered := false
		for i, f := range out {
			end := max(f.Line, f.EndLine)
			if c.Line > 0 && f.Line > 0 && c.Path == normalizedPath(f.File) && c.Line >= f.Line-3 && (c.Line <= end || c.Line-end <= 3) {
				out[i].PRComments = append(out[i].PRComments, c)
				covered = true
			}
		}
		if !covered {
			uncovered = append(uncovered, c)
		}
	}
	return out, uncovered
}
