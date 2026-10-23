package engine

import (
	"context"
	"fmt"

	"github.com/Ps3udoDev/tija-cli/internal/branch"
	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

// StatusReport describes the current branch and working tree.
type StatusReport struct {
	Root        string         `json:"root"`
	Branch      branch.Verdict `json:"branch"`
	Target      string         `json:"target"`
	TargetFound bool           `json:"targetFound"`
	Ahead       int            `json:"ahead"`
	Behind      int            `json:"behind"`
	Staged      int            `json:"staged"`
	Unstaged    int            `json:"unstaged"`
	Untracked   int            `json:"untracked"`
	HasRemote   bool           `json:"hasRemote"`
	Published   bool           `json:"published"`
	HasCommits  bool           `json:"hasCommits"`
	// HuskyWarning is set when a husky pre-push branch hook exists and the
	// branch does not conform.
	HuskyWarning bool `json:"huskyWarning"`
}

// ExitCode is 1 only in strict mode when the branch is nonconforming or
// detached, else 0.
func (r StatusReport) ExitCode(strict bool) int {
	if strict && (r.Branch.State == branch.Nonconforming || r.Branch.State == branch.Detached) {
		return 1
	}
	return 0
}

// Status inspects the repository containing dir. It never writes anything.
func Status(ctx context.Context, dir string, env sys.Env) (StatusReport, error) {
	root, ok, err := gitx.FindRoot(ctx, env.Run, dir)
	if err != nil {
		return StatusReport{}, err
	}
	if !ok {
		return StatusReport{}, fmt.Errorf("%w: %s", ErrNotARepo, dir)
	}
	repo := gitx.Repo{Dir: root, Run: env.Run}
	rep := StatusReport{Root: root, HasCommits: repo.HasCommits(ctx)}

	var mp *manifest.Manifest
	if m, ok := loadValidManifest(root); ok {
		mp = &m
	}
	rules := branchRules(ctx, repo, mp)
	target := rules.Target
	rep.Target = target

	name, detached, err := repo.CurrentBranch(ctx)
	if err != nil {
		return StatusReport{}, err
	}
	rep.Branch = branch.Classify(name, detached, rules)

	if rep.HasCommits {
		rep.Ahead, rep.Behind, rep.TargetFound = repo.AheadBehind(ctx, target)
		if !detached {
			rep.Published = repo.IsPublished(ctx, name)
		}
	}
	rep.Staged, rep.Unstaged, rep.Untracked = repo.WorkTree(ctx)
	rep.HasRemote = repo.RemoteURL(ctx) != ""

	if rep.Branch.State == branch.Nonconforming {
		// Detect errors are intentionally ignored: the husky warning is best-effort.
		if d, err := detect.Detect(root, ""); err == nil && d.HuskyBranchHook {
			rep.HuskyWarning = true
		}
	}
	return rep, nil
}
