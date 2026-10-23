package engine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/agents"
	"github.com/Ps3udoDev/tija-cli/internal/branch"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/prrules"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

// Default timeouts of PreparePR.
const (
	DefaultCheckTimeout = 10 * time.Minute
	DefaultAgentTimeout = 3 * time.Minute
)

// maxAgentAttempts is how many times the agent may answer before the last
// draft is reported with its problems.
const maxAgentAttempts = 2

// SourceDeterministic is PRReport.Source for drafts built without an agent.
const SourceDeterministic = "deterministic"

// PRRequest configures PreparePR.
type PRRequest struct {
	Agent        string        // "" = manifest agents.default; "none" = deterministic
	NoCheck      bool          // never run check, even without verified results
	CheckTimeout time.Duration // per check step; default 10m
	AgentTimeout time.Duration // per agent attempt; default 3m
}

// PRStage is the phase PreparePR is in.
type PRStage string

const (
	StageContext  PRStage = "context"
	StageCheck    PRStage = "check"
	StageAgent    PRStage = "agent"
	StageValidate PRStage = "validate"
	StageDone     PRStage = "done"
)

// PREvent reports progress. Quality is set only for check events.
type PREvent struct {
	Stage   PRStage
	Message string
	Quality *QualityEvent
}

// PRReport is the outcome of PreparePR.
type PRReport struct {
	Root     string         `json:"root"`
	Branch   string         `json:"branch"`
	Target   string         `json:"target"`
	Verdict  branch.Verdict `json:"verdict"`
	Title    string         `json:"title"`
	Body     string         `json:"body"`
	Source   string         `json:"source"` // claude | codex | gemini | deterministic
	Attempts int            `json:"attempts"`
	// Problems are the rule violations of the final draft.
	Problems  []prrules.Problem    `json:"problems"`
	Commits   int                  `json:"commits"`
	Files     int                  `json:"files"`
	Tests     []quality.StepResult `json:"tests"`
	Warnings  []string             `json:"warnings"`
	DraftPath string               `json:"draftPath"` // "" when the draft was not saved
}

// ExitCode is 1 when the draft breaks a rule, 0 otherwise.
func (r PRReport) ExitCode() int {
	if len(r.Problems) > 0 {
		return 1
	}
	return 0
}

// PRPrompt returns the agent prompt for the repository containing dir. It
// never runs check or an agent.
func PRPrompt(ctx context.Context, dir string, env sys.Env) (string, error) {
	c, err := CollectPRContext(ctx, dir, env)
	if err != nil {
		return "", err
	}
	return BuildPRPrompt(c, nil), nil
}

// PreparePR collects the pull request context, runs check when there are no
// verified results for HEAD, drafts title and body with an agent (up to two
// attempts) or deterministically, validates the result and saves it when
// the repository is initialized. A draft that breaks rules is still
// returned (and saved); see PRReport.ExitCode.
func PreparePR(ctx context.Context, dir string, env sys.Env, req PRRequest, onEvent func(PREvent)) (PRReport, error) {
	emit := func(e PREvent) {
		if onEvent != nil {
			onEvent(e)
		}
	}
	root, ok, err := gitx.FindRoot(ctx, env.Run, dir)
	if err != nil {
		return PRReport{}, err
	}
	if !ok {
		return PRReport{}, fmt.Errorf("%w: %s", ErrNotARepo, dir)
	}

	emit(PREvent{Stage: StageContext, Message: "collecting branch, commits and diff"})
	c, err := CollectPRContext(ctx, root, env)
	if err != nil {
		return PRReport{}, err
	}

	warnings := contextWarnings(c)

	switch {
	case len(c.Tests) > 0 || req.NoCheck:
		// verified results already, or the user asked not to check
	case c.Dirty:
		// check would run on the working tree, not on HEAD: its results
		// could never be cited.
		warnings = append(warnings, "uncommitted changes — skipping checks (results would not be citable)")
	default:
		tests, err := runPRCheck(ctx, c, env, req, emit)
		if err != nil {
			return PRReport{}, err
		}
		c.Tests = tests
	}

	draft, source, attempts, problems, agentWarnings := draftPR(ctx, c, env, req, emit)
	warnings = append(warnings, agentWarnings...)

	rep := PRReport{
		Root:     c.Root,
		Branch:   c.Branch,
		Target:   c.Target,
		Verdict:  c.Verdict,
		Title:    draft.Title,
		Body:     draft.Body,
		Source:   source,
		Attempts: attempts,
		Problems: problems,
		Commits:  len(c.Commits),
		Files:    len(c.Files),
		Tests:    c.Tests,
		Warnings: warnings,
	}
	if c.Initialized {
		meta := DraftMeta{Head: c.Head, Branch: c.Branch, Target: c.Target, Source: source, Problems: problems}
		if err := SaveDraft(c.Root, draft, meta); err != nil {
			return PRReport{}, err
		}
		rep.DraftPath = DraftPath
	}
	if rep.Problems == nil {
		rep.Problems = []prrules.Problem{}
	}
	if rep.Tests == nil {
		rep.Tests = []quality.StepResult{}
	}
	if rep.Warnings == nil {
		rep.Warnings = []string{}
	}
	emit(PREvent{Stage: StageDone, Message: "draft ready"})
	return rep, nil
}

// contextWarnings returns the warnings every pull request report carries
// for c: an unsaved draft, a nonconforming branch and uncommitted changes.
func contextWarnings(c PRContext) []string {
	var warnings []string
	if !c.Initialized {
		if manifestExists(c.Root) {
			warnings = append(warnings, "manifest invalid — using defaults; draft is not saved")
		} else {
			warnings = append(warnings, "not initialized — draft is not saved (run: tija init)")
		}
	}
	if c.Verdict.State == branch.Nonconforming {
		warnings = append(warnings, fmt.Sprintf("branch %s does not follow the convention (suggested: %s)", c.Branch, c.Verdict.Suggested))
	}
	if c.Dirty {
		warnings = append(warnings, "uncommitted changes are not part of the pull request")
	}
	return warnings
}

// manifestExists reports whether root has a manifest file, valid or not.
func manifestExists(root string) bool {
	_, err := os.Stat(abs(root, manifest.Path))
	return !errors.Is(err, fs.ErrNotExist)
}

// runPRCheck runs every quality kind in check mode and returns its results
// when they belong to HEAD on a clean tree. A failing step is a result, not
// an error.
func runPRCheck(ctx context.Context, c PRContext, env sys.Env, req PRRequest, emit func(PREvent)) ([]quality.StepResult, error) {
	timeout := req.CheckTimeout
	if timeout <= 0 {
		timeout = DefaultCheckTimeout
	}
	emit(PREvent{Stage: StageCheck, Message: "no verified checks for this commit — running check"})
	rep, err := RunQuality(ctx, c.Root, env, QualityRequest{Mode: quality.CheckMode, Timeout: timeout}, func(e QualityEvent) {
		ev := e
		emit(PREvent{Stage: StageCheck, Message: e.Step.Command, Quality: &ev})
	})
	if err != nil {
		return nil, err
	}
	if rep.Dirty || rep.Commit == "" || rep.Commit != c.Head {
		return nil, nil
	}
	return rep.Steps, nil
}

// draftPR produces the final draft, its source, the number of agent
// attempts made, its problems and any warnings.
func draftPR(ctx context.Context, c PRContext, env sys.Env, req PRRequest, emit func(PREvent)) (agents.Draft, string, int, []prrules.Problem, []string) {
	pr := c.Manifest.PullRequest
	validate := func(d agents.Draft) []prrules.Problem {
		emit(PREvent{Stage: StageValidate, Message: "checking title and sections"})
		return append(prrules.ValidateTitle(d.Title, pr), prrules.ValidateBody(d.Body, pr)...)
	}
	deterministic := func() (agents.Draft, []prrules.Problem) {
		d := DeterministicDraft(c)
		return d, validate(d)
	}

	name := req.Agent
	if name == "" {
		name = c.Manifest.Agents.Default
	}
	if name == "none" {
		d, ps := deterministic()
		return d, SourceDeterministic, 0, ps, nil
	}
	if name == "" {
		d, ps := deterministic()
		return d, SourceDeterministic, 0, ps, []string{"no agent available — using deterministic draft"}
	}
	if agents.Binary(name, env.LookPath) == "" {
		d, ps := deterministic()
		return d, SourceDeterministic, 0, ps, []string{fmt.Sprintf("agent %s is not installed — using deterministic draft", name)}
	}

	tmpDir, cleanup, err := agentTmpDir(c)
	if err != nil {
		d, ps := deterministic()
		return d, SourceDeterministic, 0, ps, []string{fmt.Sprintf("agent %s failed: %v — using deterministic draft", name, err)}
	}
	defer cleanup()
	timeout := req.AgentTimeout
	if timeout <= 0 {
		timeout = DefaultAgentTimeout
	}

	var (
		draft    agents.Draft
		problems []prrules.Problem
		attempts int
	)
	for attempts < maxAgentAttempts {
		attempts++
		emit(PREvent{Stage: StageAgent, Message: fmt.Sprintf("%s is drafting (attempt %d)", name, attempts)})
		out, err := agents.Generate(ctx, env, name, agents.Request{
			Prompt:  BuildPRPrompt(c, problems),
			Tier:    c.Manifest.Agents.Tasks["pullRequest"].Tier,
			Dir:     c.Root,
			TmpDir:  tmpDir,
			Timeout: timeout,
		})
		if err == nil {
			draft, err = agents.ParseDraft(out)
		}
		if err != nil {
			d, ps := deterministic()
			return d, SourceDeterministic, attempts, ps, []string{fmt.Sprintf("agent %s failed: %v — using deterministic draft", name, oneLine(err))}
		}
		draft.Title = strings.TrimSpace(draft.Title)
		draft.Body = prrules.Normalize(draft.Body)
		// Anti-invention: ## Pruebas cites only what Tija verified.
		draft.Body = prrules.Normalize(enforceVerifiedTests(draft.Body, c.Manifest.Content.Language, c.Tests))
		if problems = validate(draft); len(problems) == 0 {
			break
		}
	}
	return draft, name, attempts, problems, nil
}

// agentTmpDir returns .tija/tmp/pr/ in an initialized repo, or a fresh
// system temp dir that cleanup removes.
func agentTmpDir(c PRContext) (string, func(), error) {
	if c.Initialized {
		d := abs(c.Root, ".tija/tmp/pr")
		if err := os.MkdirAll(d, 0o755); err != nil {
			return "", nil, err
		}
		return d, func() {}, nil
	}
	d, err := os.MkdirTemp("", "tija-pr-")
	if err != nil {
		return "", nil, err
	}
	return d, func() { _ = os.RemoveAll(d) }, nil
}

// oneLine joins a multi-line error (e.g. an agent's stderr tail) so the
// warning stays on one line.
func oneLine(err error) string {
	return strings.Join(strings.Fields(err.Error()), " ")
}
