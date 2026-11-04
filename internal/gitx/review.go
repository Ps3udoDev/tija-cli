package gitx

import (
	"context"
	"strings"
)

func (r Repo) MergeBase(ctx context.Context, base string) (string, bool) {
	return r.git(ctx, "merge-base", base, "HEAD")
}

// WorkingDiff compares the working tree, including staged changes, to rev.
func (r Repo) WorkingDiff(ctx context.Context, rev string) string {
	out, _, code, err := r.Run(ctx, r.Dir, "git", "diff", "--no-color", "--no-ext-diff", "-U3", "-M", rev, "--")
	if err != nil || code != 0 {
		return ""
	}
	return strings.ReplaceAll(out, "\r\n", "\n")
}

func (r Repo) Untracked(ctx context.Context) []string {
	out, _, code, err := r.Run(ctx, r.Dir, "git", "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil || code != 0 {
		return nil
	}
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			paths = append(paths, strings.ReplaceAll(p, "\\", "/"))
		}
	}
	return paths
}

func (r Repo) LineCountAt(ctx context.Context, rev, path string) (int, bool) {
	out, _, code, err := r.Run(ctx, r.Dir, "git", "show", rev+":"+strings.ReplaceAll(path, "\\", "/"))
	if err != nil || code != 0 {
		return 0, false
	}
	if out == "" {
		return 0, true
	}
	count := strings.Count(out, "\n")
	if !strings.HasSuffix(out, "\n") {
		count++
	}
	return count, true
}
