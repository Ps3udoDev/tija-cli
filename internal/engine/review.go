package engine

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/agents"
	"github.com/Ps3udoDev/tija-cli/internal/assets"
	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/review"
	"github.com/Ps3udoDev/tija-cli/internal/rules"
	"github.com/Ps3udoDev/tija-cli/internal/skills"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

var ErrInvalidRules = errors.New("invalid architecture rules")
var ErrNoMergeBase = errors.New("no merge base with the target branch")

type ReviewRequest struct {
	Mode, FailOn, Agent        string
	NoAgent                    bool
	AgentTimeout, CheckTimeout time.Duration
	NoCheck, NoPRComments      bool
}

// ReviewStage shares the existing stage vocabulary with pull request progress.
type ReviewStage = PRStage

const (
	StageQuality    ReviewStage = "quality"
	StageChecks     ReviewStage = "checks"
	StageVerify     ReviewStage = "verify"
	StagePRComments ReviewStage = "pr-comments"
)

type ReviewEvent struct {
	Stage         ReviewStage
	Message, Pass string
	Chunk, Chunks int
	Quality       *QualityEvent
}
type ReviewReport struct {
	review.Result
	Root        string `json:"-"`
	Initialized bool   `json:"-"`
}

func (r ReviewReport) ExitCode() int { return r.Exit }

// RunReview reviews the working tree against its target's merge base. Only the
// review session is persisted in the repository; agent temporary output lives
// in the system temporary directory.
func RunReview(ctx context.Context, dir string, env sys.Env, req ReviewRequest, onEvent func(ReviewEvent)) (ReviewReport, error) {
	if req.Mode == "" {
		req.Mode = "review"
	}
	if !slices.Contains([]string{"review", "quality", "deep"}, req.Mode) {
		return ReviewReport{}, fmt.Errorf("invalid review mode %q", req.Mode)
	}
	if req.FailOn != "" && !slices.Contains(manifest.FailOnValues, req.FailOn) {
		return ReviewReport{}, fmt.Errorf("invalid review.failOn %q", req.FailOn)
	}
	if req.AgentTimeout < 0 || req.CheckTimeout < 0 {
		return ReviewReport{}, errors.New("timeouts must be positive")
	}
	var eventMu sync.Mutex
	emit := func(ev ReviewEvent) {
		eventMu.Lock()
		defer eventMu.Unlock()
		if onEvent != nil {
			onEvent(ev)
		}
	}
	emit(ReviewEvent{Stage: StageContext, Message: "collecting working tree and merge base"})
	root, ok, err := gitx.FindRoot(ctx, env.Run, dir)
	if err != nil {
		return ReviewReport{}, err
	}
	if !ok {
		return ReviewReport{}, ErrNotARepo
	}
	repo := gitx.Repo{Dir: root, Run: env.Run}
	m, initialized := loadValidManifest(root)
	var mp *manifest.Manifest
	if initialized {
		mp = &m
	}
	target := branchRules(ctx, repo, mp).Target
	if !initialized {
		d, e := detect.Detect(root, repo.RemoteURL(ctx))
		if e != nil {
			return ReviewReport{}, e
		}
		m = manifest.FromDetection(d, agents.Available(env.LookPath), target)
	}
	base, ok := repo.ResolveBase(ctx, target)
	if !ok {
		return ReviewReport{}, fmt.Errorf("%w: %s", ErrNoMergeBase, target)
	}
	mergeBase, ok := repo.MergeBase(ctx, base)
	if !ok {
		return ReviewReport{}, fmt.Errorf("%w: %s", ErrNoMergeBase, base)
	}
	// Resolve the agent before any work: an explicitly requested agent that is
	// not installed is an environment error, never a silent deterministic run.
	name := ""
	if !req.NoAgent {
		if name, err = reportProvider(m, env, req.Agent); err != nil {
			return ReviewReport{}, err
		}
	}
	head, _ := repo.Head(ctx)
	a, b, c := repo.WorkTree(ctx)
	cfg := m.Review.WithDefaults()
	if req.FailOn != "" {
		cfg.FailOn = req.FailOn
	}
	rep := ReviewReport{Root: root, Initialized: initialized, Result: review.Result{Mode: req.Mode, Base: base, Commit: head, Dirty: a+b+c > 0, FailOn: cfg.FailOn}}
	files, err := reviewFiles(ctx, repo, mergeBase, cfg.Exclude)
	if err != nil {
		return rep, err
	}
	packs := map[string]skills.Pack{}
	reserved := []string{"1000-lines-ceiling", "quality-check-failed"}
	for _, pass := range []string{"review", "quality"} {
		name := "thermo-" + pass
		p, e := skills.Load(root, m, name)
		if e != nil {
			// An existing but malformed local pack is an environment error.
			if reviewPackExists(root, m, name) {
				return rep, e
			}
			data, _ := assets.Skill(name)
			p, e = skills.Parse(data)
			if e != nil {
				return rep, e
			}
		}
		packs[pass] = p
		reserved = append(reserved, p.Rules...)
	}
	var set rules.Set
	if initialized {
		var errs []error
		set, errs = rules.Load(root, cfg.Rules, reserved)
		if len(errs) > 0 {
			return rep, fmt.Errorf("%w: %v", ErrInvalidRules, errors.Join(errs...))
		}
	}
	if len(files) == 0 {
		rep.Quality.Status = "skipped"
		emit(ReviewEvent{Stage: StageDone, Message: "nothing to review"})
		return finishReview(rep)
	}
	qualityLines, err := reviewQuality(ctx, root, env, req, &rep, emit)
	if err != nil {
		return rep, err
	}
	emit(ReviewEvent{Stage: StageChecks, Message: "running deterministic checks"})
	if err := reviewChecks(ctx, repo, mergeBase, files, set, &rep); err != nil {
		return rep, err
	}
	chunks := review.Chunks(files, cfg.MaxLines)
	rep.Chunks.Total = len(chunks)
	if req.NoAgent || req.Agent == "none" {
		rep.Warnings = append(rep.Warnings, "agent disabled — only deterministic checks ran")
	} else if name == "" {
		rep.Warnings = append(rep.Warnings, "no agent CLI found — only deterministic checks ran")
	} else {
		reviewAgents(ctx, root, env, req, m, name, chunks, packs, set, qualityLines, &rep, emit)
	}
	if req.Mode == "deep" {
		rep.Findings = review.Merge(rep.Findings)
	}
	localizeReviewFindings(m.Content.Language, &rep)
	if !req.NoPRComments {
		emit(ReviewEvent{Stage: StagePRComments, Message: "reading pull request comments"})
		reviewPRComments(ctx, repo, env, &rep)
	}
	emit(ReviewEvent{Stage: StageDone, Message: "review complete"})
	return finishReview(rep)
}

func finishReview(rep ReviewReport) (ReviewReport, error) {
	review.Sort(rep.Findings)
	rep.Finalize()
	if rep.Initialized {
		if err := review.SaveSession(rep.Root, rep.Result); err != nil {
			return rep, err
		}
	}
	return rep, nil
}
