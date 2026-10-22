// Package gitx answers read-only questions about a git repository.
package gitx

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

// Repo is a git working directory queried through Run.
type Repo struct {
	Dir string
	Run sys.Runner
}

// FindRoot returns the top-level directory of the repository containing dir.
// ok is false (with a nil error) when dir is not inside a repository. Any
// other git failure (e.g. "detected dubious ownership") is returned as an
// error carrying git's message.
func FindRoot(ctx context.Context, run sys.Runner, dir string) (root string, ok bool, err error) {
	out, stderr, code, err := run(ctx, dir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", false, err
	}
	if code != 0 {
		if strings.Contains(strings.ToLower(stderr), "not a git repository") {
			return "", false, nil
		}
		return "", false, gitError(stderr, code)
	}
	root = strings.TrimSpace(out)
	if root == "" {
		return "", false, nil
	}
	return filepath.Clean(filepath.FromSlash(root)), true, nil
}

// gitError turns a failed git invocation into an error with git's message.
func gitError(stderr string, code int) error {
	msg := strings.TrimSpace(stderr)
	msg = strings.TrimSpace(strings.TrimPrefix(msg, "fatal:"))
	if msg == "" {
		return fmt.Errorf("git failed with exit code %d", code)
	}
	return fmt.Errorf("git: %s", msg)
}

// git runs a git command and returns trimmed stdout and whether it succeeded.
func (r Repo) git(ctx context.Context, args ...string) (string, bool) {
	out, _, code, err := r.Run(ctx, r.Dir, "git", args...)
	if err != nil || code != 0 {
		return "", false
	}
	return strings.TrimSpace(out), true
}

// CurrentBranch returns the checked-out branch name, or detached=true when
// HEAD does not point to a branch (git exits 1). Other failures are errors.
func (r Repo) CurrentBranch(ctx context.Context) (name string, detached bool, err error) {
	out, stderr, code, err := r.Run(ctx, r.Dir, "git", "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		return "", false, err
	}
	switch code {
	case 0:
		return strings.TrimSpace(out), false, nil
	case 1:
		return "", true, nil
	default:
		return "", false, gitError(stderr, code)
	}
}

// HasCommits reports whether HEAD resolves to a commit.
func (r Repo) HasCommits(ctx context.Context) bool {
	_, ok := r.git(ctx, "rev-parse", "--verify", "-q", "HEAD")
	return ok
}

// LocalBranches lists local branch names.
func (r Repo) LocalBranches(ctx context.Context) []string {
	out, ok := r.git(ctx, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if !ok || out == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
}

// DefaultRemoteBranch returns the branch origin/HEAD points to, or "".
func (r Repo) DefaultRemoteBranch(ctx context.Context) string {
	out, ok := r.git(ctx, "symbolic-ref", "--short", "-q", "refs/remotes/origin/HEAD")
	if !ok {
		return ""
	}
	return strings.TrimPrefix(out, "origin/")
}

// RemoteURL returns the URL of origin, or "".
func (r Repo) RemoteURL(ctx context.Context) string {
	out, _ := r.git(ctx, "remote", "get-url", "origin")
	return out
}

// AheadBehind compares HEAD with base (local branch first, then origin/base).
func (r Repo) AheadBehind(ctx context.Context, base string) (ahead, behind int, found bool) {
	for _, ref := range []string{"refs/heads/" + base, "refs/remotes/origin/" + base} {
		if _, ok := r.git(ctx, "rev-parse", "--verify", "-q", ref); !ok {
			continue
		}
		out, ok := r.git(ctx, "rev-list", "--left-right", "--count", ref+"...HEAD")
		if !ok {
			continue
		}
		if _, err := fmt.Sscanf(out, "%d %d", &behind, &ahead); err != nil {
			continue
		}
		return ahead, behind, true
	}
	return 0, 0, false
}

// WorkTree counts staged, unstaged and untracked entries.
func (r Repo) WorkTree(ctx context.Context) (staged, unstaged, untracked int) {
	// Raw output: the leading space of " M file" is significant.
	out, _, code, err := r.Run(ctx, r.Dir, "git", "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil || code != 0 {
		return 0, 0, 0
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]
		switch {
		case x == '?' && y == '?':
			untracked++
		case x == '!' && y == '!':
		default:
			if x != ' ' {
				staged++
			}
			if y != ' ' {
				unstaged++
			}
		}
	}
	return staged, unstaged, untracked
}

// IsPublished reports whether refs/remotes/origin/<branch> exists.
func (r Repo) IsPublished(ctx context.Context, branch string) bool {
	_, ok := r.git(ctx, "rev-parse", "--verify", "-q", "refs/remotes/origin/"+branch)
	return ok
}
