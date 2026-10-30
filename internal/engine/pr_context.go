package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/agents"
	"github.com/Ps3udoDev/tija-cli/internal/assets"
	"github.com/Ps3udoDev/tija-cli/internal/branch"
	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/prrules"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

// Errors returned by CollectPRContext. ErrBaseBranch, ErrDetached and
// ErrTargetNotFound are usage errors (exit 2); ErrNoCommits is exit 1.
var (
	ErrBaseBranch     = errors.New("current branch is a base branch")
	ErrDetached       = errors.New("HEAD is detached")
	ErrTargetNotFound = errors.New("target branch not found")
	ErrNoCommits      = errors.New("no commits ahead of the target branch")
)

// maxDiffBytes caps the diff sent to the agent.
const maxDiffBytes = 60_000

// PRContext is everything Tija computes before drafting a pull request.
type PRContext struct {
	Root, Branch, Target, BaseRef, Head string
	Verdict                             branch.Verdict
	IssueNumber                         int
	Commits                             []gitx.Commit
	Files                               []gitx.FileStat
	// Diff is the concatenated per-file diff, at most maxDiffBytes long.
	Diff string
	// Truncated lists the paths left out of Diff.
	Truncated []string
	// Tests holds only the quality results of HEAD with a clean tree.
	Tests           []quality.StepResult
	Rules, Template string
	// Manifest is the repo's manifest, or one built in memory from detection.
	Manifest    manifest.Manifest
	Initialized bool
	Dirty       bool
}

// CollectPRContext gathers branch, commits, diff, quality results, rules and
// template for a pull request from the repository containing dir.
func CollectPRContext(ctx context.Context, dir string, env sys.Env) (PRContext, error) {
	root, ok, err := gitx.FindRoot(ctx, env.Run, dir)
	if err != nil {
		return PRContext{}, err
	}
	if !ok {
		return PRContext{}, fmt.Errorf("%w: %s", ErrNotARepo, dir)
	}
	repo := gitx.Repo{Dir: root, Run: env.Run}
	c := PRContext{Root: root}

	m, initialized := loadValidManifest(root)
	var mp *manifest.Manifest
	if initialized {
		mp = &m
	}
	rules := branchRules(ctx, repo, mp)
	c.Target = rules.Target

	name, detached, err := repo.CurrentBranch(ctx)
	if err != nil {
		return PRContext{}, err
	}
	if detached {
		return PRContext{}, ErrDetached
	}
	c.Branch = name
	c.Verdict = branch.Classify(name, false, rules)
	if c.Verdict.State == branch.Base {
		return PRContext{}, fmt.Errorf("%w: %s", ErrBaseBranch, name)
	}
	c.IssueNumber = c.Verdict.IssueNumber

	if !initialized {
		d, err := detect.Detect(root, repo.RemoteURL(ctx))
		if err != nil {
			return PRContext{}, err
		}
		m = manifest.FromDetection(d, agents.Available(env.LookPath), c.Target)
	}
	c.Manifest, c.Initialized = m, initialized

	base, ok := repo.ResolveBase(ctx, c.Target)
	if !ok {
		return PRContext{}, fmt.Errorf("%w: %s", ErrTargetNotFound, c.Target)
	}
	c.BaseRef = base
	c.Head, _ = repo.Head(ctx)
	c.Commits = repo.Commits(ctx, base)
	if len(c.Commits) == 0 {
		return PRContext{}, fmt.Errorf("%w: %s..%s", ErrNoCommits, c.Target, name)
	}
	if c.Manifest.Content.AgentAttribution.StripEnabled() {
		if pats, err := prrules.AttributionPatterns(c.Manifest.Content.AgentAttribution.Patterns); err == nil {
			for i := range c.Commits {
				c.Commits[i].Body, _ = prrules.StripAttribution(c.Commits[i].Body, pats)
			}
		}
	}
	c.Files = repo.DiffStat(ctx, base)
	c.Diff, c.Truncated = buildDiff(ctx, repo, base, c.Files)

	staged, unstaged, untracked := repo.WorkTree(ctx)
	c.Dirty = staged+unstaged+untracked > 0
	// A corrupt session only means no verified results.
	if s, err := quality.LoadSession(root); err == nil {
		c.Tests = FreshTests(s, c.Head)
	}

	if c.Rules, err = readOrDefault(root, m.PullRequest.Rules, assets.RulesPath); err != nil {
		return PRContext{}, err
	}
	if c.Template, err = readOrDefault(root, m.PullRequest.Template, assets.TemplatePath); err != nil {
		return PRContext{}, err
	}
	return c, nil
}

// buildDiff concatenates the per-file diffs in order while the total stays
// within maxDiffBytes. Excluded and oversized files go to truncated.
func buildDiff(ctx context.Context, repo gitx.Repo, base string, files []gitx.FileStat) (string, []string) {
	var b strings.Builder
	var truncated []string
	for _, f := range files {
		if excludedFromDiff(f) {
			truncated = append(truncated, f.Path)
			continue
		}
		d := repo.DiffFile(ctx, base, f.Path)
		if b.Len()+len(d) > maxDiffBytes {
			truncated = append(truncated, f.Path)
			continue
		}
		b.WriteString(d)
	}
	return b.String(), truncated
}

var (
	lockfiles   = []string{"package-lock.json", "bun.lock", "bun.lockb", "yarn.lock", "pnpm-lock.yaml", "packages.lock.json"}
	excludedDir = []string{"bin", "obj", "dist", "node_modules", ".tija"}
)

// excludedFromDiff reports whether f only belongs in the diffstat: lockfiles,
// build output and dependency folders, minified and binary files.
func excludedFromDiff(f gitx.FileStat) bool {
	if f.Binary {
		return true
	}
	segs := strings.Split(f.Path, "/")
	name := segs[len(segs)-1]
	if slices.Contains(lockfiles, name) || strings.Contains(name, ".min.") {
		return true
	}
	for _, s := range segs[:len(segs)-1] {
		if slices.Contains(excludedDir, s) {
			return true
		}
	}
	return false
}

// readOrDefault reads the repo-relative file rel, or returns the embedded
// default at assetPath when rel is empty or does not exist. CRLF becomes LF.
func readOrDefault(root, rel, assetPath string) (string, error) {
	if rel != "" {
		data, err := os.ReadFile(abs(root, path.Clean(rel)))
		if err == nil {
			return strings.ReplaceAll(string(data), "\r\n", "\n"), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return strings.ReplaceAll(string(assets.Files()[assetPath]), "\r\n", "\n"), nil
}

// FreshTests returns the session results recorded for head on a clean tree,
// once per run (a run covering several kinds appears once), in kind order.
func FreshTests(s quality.Session, head string) []quality.StepResult {
	kinds := make([]quality.Kind, 0, len(s.Results))
	for k := range s.Results {
		kinds = append(kinds, k)
	}
	slices.SortFunc(kinds, func(a, b quality.Kind) int {
		ia, ib := kindRank(a), kindRank(b)
		if ia != ib {
			return ia - ib
		}
		return strings.Compare(string(a), string(b))
	})
	type key struct{ provider, command string }
	seen := map[key]bool{}
	var out []quality.StepResult
	for _, k := range kinds {
		r := s.Results[k]
		if head == "" || r.Commit != head || r.Dirty {
			continue
		}
		id := key{r.Provider, r.Command}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, r)
	}
	return out
}

func kindRank(k quality.Kind) int {
	if i := slices.Index(quality.Order, k); i >= 0 {
		return i
	}
	return len(quality.Order)
}
