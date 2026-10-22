package render

import (
	"fmt"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/branch"
	"github.com/Ps3udoDev/tija-cli/internal/engine"
)

// Status renders the branch verdict, target, working tree and remote state.
func Status(p Printer, r engine.StatusReport) {
	if p.Mode == JSON {
		_ = p.JSONOut(r)
		return
	}
	if r.Root != "" {
		p.Field("Repository", r.Root)
	}
	v := r.Branch
	name := v.Name
	if v.State == branch.Detached {
		name = "(detached HEAD)"
	}
	p.Field("Branch", name)
	if v.IssueNumber > 0 {
		p.Field("Issue", fmt.Sprintf("#%d", v.IssueNumber))
	}
	p.Field("Target", target(r))
	p.Field("Working tree", workTree(r))
	p.Field("Remote", remote(r))

	p.Line("")
	switch v.State {
	case branch.Conforming:
		p.Ok(fmt.Sprintf("branch %q follows the naming rules", v.Name))
	case branch.Base:
		p.Ok(fmt.Sprintf("on base branch %q", v.Name))
	case branch.Detached:
		p.Warn("detached HEAD — check out a branch before opening a pull request")
	case branch.Nonconforming:
		msg := fmt.Sprintf("branch %q", v.Name)
		if v.Reason != "" {
			msg += " — " + v.Reason
		}
		p.Warn(msg)
		if v.Suggested != "" {
			p.Line("  suggested: " + v.Suggested)
			p.Line("  rename: git branch -m " + v.Name + " " + v.Suggested)
		}
		if r.HuskyWarning {
			p.Warn("this repo's pre-push hook will reject this branch")
		}
	}
}

func target(r engine.StatusReport) string {
	switch {
	case r.Target == "":
		return none
	case !r.HasCommits:
		return r.Target + " (no commits yet)"
	case !r.TargetFound:
		return r.Target + " (not found)"
	}
	return fmt.Sprintf("%s (%d ahead, %d behind)", r.Target, r.Ahead, r.Behind)
}

func workTree(r engine.StatusReport) string {
	var parts []string
	for _, c := range []struct {
		n    int
		what string
	}{{r.Staged, "staged"}, {r.Unstaged, "unstaged"}, {r.Untracked, "untracked"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, c.what))
		}
	}
	if len(parts) == 0 {
		return "clean"
	}
	return strings.Join(parts, ", ")
}

func remote(r engine.StatusReport) string {
	switch {
	case !r.HasRemote:
		return "none"
	case r.Branch.State == branch.Detached:
		return "origin"
	case r.Published:
		return "origin, branch published"
	}
	return "origin, branch not published"
}
