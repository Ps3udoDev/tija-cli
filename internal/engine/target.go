package engine

import (
	"context"
	"os"
	"slices"

	"github.com/Ps3udoDev/tija-cli/internal/branch"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
)

// defaultTarget picks the PR target branch: origin's default branch, else
// local main, local master, origin/main, origin/master, the current branch
// and finally "main". It never returns "", so the manifest stays valid.
func defaultTarget(ctx context.Context, repo gitx.Repo) string {
	if b := repo.DefaultRemoteBranch(ctx); b != "" {
		return b
	}
	local := repo.LocalBranches(ctx)
	for _, b := range []string{"main", "master"} {
		if slices.Contains(local, b) {
			return b
		}
	}
	for _, b := range []string{"main", "master"} {
		if repo.IsPublished(ctx, b) {
			return b
		}
	}
	if cur, detached, err := repo.CurrentBranch(ctx); err == nil && !detached && cur != "" {
		return cur
	}
	return "main"
}

// targetChoice offers the base branches that exist locally or on origin
// when there are at least two of them. ok is false otherwise.
func targetChoice(ctx context.Context, repo gitx.Repo, def string) (Choice, bool) {
	local := repo.LocalBranches(ctx)
	var opts []string
	for _, b := range branch.DefaultBases {
		if slices.Contains(local, b) || repo.IsPublished(ctx, b) {
			opts = append(opts, b)
		}
	}
	if len(opts) < 2 {
		return Choice{}, false
	}
	if def != "" && !slices.Contains(opts, def) {
		opts = append([]string{def}, opts...)
	}
	if def == "" {
		def = opts[0]
	}
	return Choice{ID: ChoiceTarget, Question: "Target branch for pull requests?", Options: opts, Default: def}, true
}

// loadValidManifest reads the repo manifest. ok is false when it is missing,
// unreadable or invalid.
func loadValidManifest(root string) (manifest.Manifest, bool) {
	data, err := os.ReadFile(abs(root, manifest.Path))
	if err != nil {
		return manifest.Manifest{}, false
	}
	m, problem := parseValid(data)
	return m, problem == ""
}

// branchRules builds the classification rules from the manifest (nil when
// the repo has no valid one) plus the defaults: base branches always include
// branch.DefaultBases and origin's default branch, and the target falls back
// to defaultTarget.
func branchRules(ctx context.Context, repo gitx.Repo, m *manifest.Manifest) branch.Rules {
	bases := slices.Clone(branch.DefaultBases)
	prefixes, target := branch.DefaultPrefixes, ""
	if m != nil {
		if len(m.Git.BranchPrefixes) > 0 {
			prefixes = m.Git.BranchPrefixes
		}
		bases = append(bases, m.Git.BaseBranches...)
		target = m.PullRequest.Target
	}
	if target == "" {
		target = defaultTarget(ctx, repo)
	}
	if rb := repo.DefaultRemoteBranch(ctx); rb != "" && !slices.Contains(bases, rb) {
		bases = append(bases, rb)
	}
	return branch.Rules{Prefixes: prefixes, Bases: bases, Target: target}
}
