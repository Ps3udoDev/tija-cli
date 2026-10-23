package gitx

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
)

// Commit is one commit ahead of a base ref.
type Commit struct {
	SHA, Subject, Body string
}

// FileStat is one entry of a diff --numstat.
type FileStat struct {
	Path           string
	Added, Deleted int
	Binary         bool
}

// ResolveBase returns the ref to compare against for target: the remote
// tracking branch when it exists, otherwise the local branch.
func (r Repo) ResolveBase(ctx context.Context, target string) (ref string, ok bool) {
	for _, c := range []string{"refs/remotes/origin/" + target, "refs/heads/" + target} {
		if _, ok := r.git(ctx, "rev-parse", "--verify", "-q", c); ok {
			return c, true
		}
	}
	return "", false
}

// Commits lists the commits in base..HEAD, oldest first.
func (r Repo) Commits(ctx context.Context, base string) []Commit {
	out, _, code, err := r.Run(ctx, r.Dir, "git", "log", "--reverse", "--format=%H%x1f%s%x1f%b%x1e", base+"..HEAD")
	if err != nil || code != 0 {
		return nil
	}
	out = strings.ReplaceAll(out, "\r\n", "\n")
	var commits []Commit
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.TrimLeft(rec, "\n")
		if strings.TrimSpace(rec) == "" {
			continue
		}
		f := strings.SplitN(rec, "\x1f", 3)
		if len(f) < 3 {
			continue
		}
		commits = append(commits, Commit{SHA: f[0], Subject: f[1], Body: strings.TrimSpace(f[2])})
	}
	return commits
}

// DiffStat returns per-file line counts of base...HEAD. For renames the
// destination path is used.
func (r Repo) DiffStat(ctx context.Context, base string) []FileStat {
	out, _, code, err := r.Run(ctx, r.Dir, "git", "diff", "--numstat", "-z", base+"...HEAD")
	if err != nil || code != 0 || out == "" {
		return nil
	}
	parts := strings.Split(out, "\x00")
	var stats []FileStat
	for i := 0; i < len(parts); i++ {
		f := strings.SplitN(parts[i], "\t", 3)
		if len(f) < 3 {
			continue
		}
		path := f[2]
		if path == "" { // rename/copy: old and new paths follow as separate entries
			if i+2 >= len(parts) {
				break
			}
			path = parts[i+2]
			i += 2
		}
		fs := FileStat{Path: filepath.ToSlash(path)}
		if f[0] == "-" && f[1] == "-" {
			fs.Binary = true
		} else {
			fs.Added, _ = strconv.Atoi(f[0])
			fs.Deleted, _ = strconv.Atoi(f[1])
		}
		stats = append(stats, fs)
	}
	return stats
}

// DiffFile returns the unified diff of path in base...HEAD.
func (r Repo) DiffFile(ctx context.Context, base, path string) string {
	out, _, code, err := r.Run(ctx, r.Dir, "git", "diff", "--no-color", "--no-ext-diff", base+"...HEAD", "--", path)
	if err != nil || code != 0 {
		return ""
	}
	return strings.ReplaceAll(out, "\r\n", "\n")
}
