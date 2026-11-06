package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/prrules"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

// ErrGHUnavailable means gh is not on PATH or not authenticated (exit 2).
var ErrGHUnavailable = errors.New("GitHub CLI unavailable — install GitHub CLI and run: gh auth login")

// bodyFileName is the --body-file written under the agent temp dir.
const bodyFileName = "body.md"

// CreateRequest configures PlanCreatePR and ApplyCreatePR.
type CreateRequest struct {
	Prepare PRRequest
	Draft   bool // gh --draft
}

// CreatePlan is what ApplyCreatePR will do, shown before asking consent.
type CreatePlan struct {
	Report PRReport `json:"report"`
	// Push is true when the branch is not published or has unpushed commits.
	Push bool `json:"push"`
	// Existing is the URL of an open pull request for the branch, or "".
	Existing string `json:"existing"`
	// ExistingBase is the base branch of the Existing pull request. When it
	// differs from Report.Target the plan warns; the base is never changed.
	ExistingBase string `json:"existingBase"`
	Assign       bool   `json:"assign"`
}

// Question is the consent question for p, shared by the CLI and the TUI:
// it names the push only when there is one, the pull request to open or
// update, and the assignment when there is one.
func (p CreatePlan) Question() string {
	var parts []string
	if p.Push {
		parts = append(parts, "push "+p.Report.Branch)
	}
	if p.Existing != "" {
		update := "update pull request " + p.Existing
		if p.ExistingBase != "" && p.ExistingBase != p.Report.Target {
			update += " (base: " + p.ExistingBase + ")"
		}
		parts = append(parts, update)
	} else {
		parts = append(parts, "open a pull request to "+p.Report.Target)
	}
	if p.Assign {
		parts = append(parts, "assign it to you")
	}
	return capitalize(strings.Join(parts, " and ")) + "?"
}

// CreateReport is the outcome of ApplyCreatePR.
type CreateReport struct {
	URL      string `json:"url"`
	Pushed   bool   `json:"pushed"`
	Updated  bool   `json:"updated"`
	Assigned bool   `json:"assigned"`
	// Number is the pull request number taken from URL; 0 when it has none.
	Number int `json:"number"`
	// Evidence is the normalized evidence the published body cites.
	Evidence []EvidenceRef `json:"evidence,omitempty"`
}

// PlanCreatePR checks gh, reuses the draft saved by prepare when it belongs
// to the current HEAD, branch and target (revalidating it, so hand edits
// count) or prepares a new one, and works out whether the branch needs a
// push and whether an open pull request already exists. It changes nothing
// outside .tija/. A plan whose Report has problems must not be applied.
func PlanCreatePR(ctx context.Context, dir string, env sys.Env, req CreateRequest, onEvent func(PREvent)) (CreatePlan, error) {
	emit := func(e PREvent) {
		if onEvent != nil {
			onEvent(e)
		}
	}
	root, ok, err := gitx.FindRoot(ctx, env.Run, dir)
	if err != nil {
		return CreatePlan{}, err
	}
	if !ok {
		return CreatePlan{}, fmt.Errorf("%w: %s", ErrNotARepo, dir)
	}
	emit(PREvent{Stage: StageContext, Message: "collecting branch, commits and diff"})
	c, err := CollectPRContext(ctx, root, env)
	if err != nil {
		return CreatePlan{}, err
	}
	if err := checkGH(ctx, root, env); err != nil {
		return CreatePlan{}, err
	}

	incoming, err := NormalizeEvidence(req.Prepare.EvidenceRefs)
	if err != nil {
		return CreatePlan{}, err
	}
	rep, fresh := savedDraftReport(c, emit, incoming, req.Prepare.EvidenceRefs != nil)
	if !fresh {
		if rep, err = PreparePR(ctx, root, env, req.Prepare, onEvent); err != nil {
			return CreatePlan{}, err
		}
	}

	p := CreatePlan{Report: rep, Assign: autoAssign(c)}
	if p.Push, err = needsPush(ctx, root, env, c.Branch); err != nil {
		return CreatePlan{}, err
	}
	if p.Existing, p.ExistingBase, err = openPRFor(ctx, root, env, c.Branch); err != nil {
		return CreatePlan{}, err
	}
	if p.Existing != "" && p.ExistingBase != "" && p.ExistingBase != p.Report.Target {
		p.Report.Warnings = append(p.Report.Warnings, fmt.Sprintf(
			"existing pull request targets %s, not %s — its diff differs from this description", p.ExistingBase, p.Report.Target))
	}
	return p, nil
}

// ApplyCreatePR pushes the branch when the plan says so (never forced) and
// creates the pull request with gh, or edits the existing one.
func ApplyCreatePR(ctx context.Context, dir string, env sys.Env, p CreatePlan, req CreateRequest) (CreateReport, error) {
	r := p.Report
	if len(r.Problems) > 0 {
		return CreateReport{}, fmt.Errorf("the draft breaks %d rule(s) — fix it before creating the pull request", len(r.Problems))
	}
	root := r.Root
	if root == "" {
		var ok bool
		var err error
		if root, ok, err = gitx.FindRoot(ctx, env.Run, dir); err != nil {
			return CreateReport{}, err
		} else if !ok {
			return CreateReport{}, fmt.Errorf("%w: %s", ErrNotARepo, dir)
		}
	}

	var out CreateReport
	// fail reports a failure after the push step, saying when the branch
	// was pushed already so the user knows the remote changed.
	fail := func(err error) (CreateReport, error) {
		if out.Pushed {
			return out, fmt.Errorf("pushed %s; %w", r.Branch, err)
		}
		return out, err
	}
	if p.Push {
		if _, stderr, code, err := env.Run(ctx, root, "git", "push", "-u", "origin", r.Branch); err != nil || code != 0 {
			return CreateReport{}, commandError("git push", stderr, code, err)
		}
		out.Pushed = true
	}

	_, initialized := loadValidManifest(root)
	tmpDir, cleanup, err := agentTmpDir(PRContext{Root: root, Initialized: initialized})
	if err != nil {
		return fail(err)
	}
	defer cleanup()
	bodyFile := filepath.Join(tmpDir, bodyFileName)
	if err := os.WriteFile(bodyFile, []byte(prrules.Normalize(r.Body)), 0o644); err != nil {
		return fail(err)
	}

	if p.Existing != "" {
		args := []string{"pr", "edit", p.Existing, "--title", r.Title, "--body-file", bodyFile}
		if p.Assign {
			args = append(args, "--add-assignee", "@me")
		}
		if _, stderr, code, err := env.Run(ctx, root, "gh", args...); err != nil || code != 0 {
			return fail(commandError("gh pr edit", stderr, code, err))
		}
		out.URL, out.Updated, out.Assigned = p.Existing, true, p.Assign
		out.Number, out.Evidence = prNumber(out.URL), r.Evidence
		return out, nil
	}

	args := []string{"pr", "create", "--base", r.Target, "--head", r.Branch, "--title", r.Title, "--body-file", bodyFile}
	if p.Assign {
		args = append(args, "--assignee", "@me")
	}
	if req.Draft {
		args = append(args, "--draft")
	}
	stdout, stderr, code, err := env.Run(ctx, root, "gh", args...)
	if err != nil || code != 0 {
		return fail(commandError("gh pr create", stderr, code, err))
	}
	out.URL = lastURL(stdout)
	if out.URL == "" {
		return fail(fmt.Errorf("gh pr create printed no pull request URL"))
	}
	out.Assigned = p.Assign
	out.Number, out.Evidence = prNumber(out.URL), r.Evidence
	return out, nil
}

// checkGH requires gh on PATH and an authenticated session.
func checkGH(ctx context.Context, root string, env sys.Env) error {
	if _, err := env.LookPath("gh"); err != nil {
		return fmt.Errorf("%w (%v)", ErrGHUnavailable, err)
	}
	stdout, stderr, code, err := env.Run(ctx, root, "gh", "auth", "status")
	if err != nil {
		return fmt.Errorf("%w (%v)", ErrGHUnavailable, err)
	}
	if code != 0 {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = strings.TrimSpace(stdout)
		}
		if msg == "" {
			msg = fmt.Sprintf("gh auth status exited with code %d", code)
		}
		return fmt.Errorf("%w (%s)", ErrGHUnavailable, strings.Join(strings.Fields(msg), " "))
	}
	return nil
}

// savedDraftReport builds the report from the saved draft when it was made
// for c's HEAD, branch and target. fresh is false otherwise.
func savedDraftReport(c PRContext, emit func(PREvent), incoming []EvidenceRef, provided bool) (PRReport, bool) {
	if !c.Initialized || c.Head == "" {
		return PRReport{}, false
	}
	d, meta, err := LoadDraft(c.Root)
	if err != nil || meta.Head != c.Head || meta.Branch != c.Branch || meta.Target != c.Target {
		return PRReport{}, false
	}
	// Evidence belongs to the draft it was prepared with: different (or
	// explicitly absent) references, or a stored set that no longer
	// validates, mean the draft cannot be reused.
	stored, err := revalidateStoredEvidence(meta.Evidence)
	if err != nil || (provided && !sameEvidence(incoming, stored)) {
		return PRReport{}, false
	}
	if len(stored) > 0 {
		d.Body = prrules.Normalize(enforceEvidence(d.Body, stored))
	}
	emit(PREvent{Stage: StageValidate, Message: "checking title and sections of the saved draft"})
	pr := c.Manifest.PullRequest
	problems := append(prrules.ValidateTitle(d.Title, pr), prrules.ValidateBody(d.Body, pr)...)
	if problems == nil {
		problems = []prrules.Problem{}
	}
	tests := c.Tests
	if tests == nil {
		tests = []quality.StepResult{}
	}
	warnings := contextWarnings(c)
	if warnings == nil {
		warnings = []string{}
	}
	emit(PREvent{Stage: StageDone, Message: "using saved draft"})
	var validation *EvidenceValidation
	if len(stored) > 0 {
		validation = &EvidenceValidation{Structural: "passed", ExternalExistence: "not_checked"}
	}
	return PRReport{
		Evidence:           stored,
		EvidenceValidation: validation,
		Root:               c.Root,
		Branch:             c.Branch,
		Target:             c.Target,
		Verdict:            c.Verdict,
		Title:              d.Title,
		Body:               d.Body,
		Source:             meta.Source,
		Problems:           problems,
		Commits:            len(c.Commits),
		Files:              len(c.Files),
		Tests:              tests,
		Warnings:           warnings,
		DraftPath:          DraftPath,
	}, true
}

// autoAssign is pullRequest.autoAssign, true when absent.
func autoAssign(c PRContext) bool {
	a := c.Manifest.PullRequest.AutoAssign
	return a == nil || *a
}

// needsPush reports whether origin/<branch> is missing or behind HEAD.
func needsPush(ctx context.Context, root string, env sys.Env, branch string) (bool, error) {
	repo := gitx.Repo{Dir: root, Run: env.Run}
	if !repo.IsPublished(ctx, branch) {
		return true, nil
	}
	stdout, stderr, code, err := env.Run(ctx, root, "git", "rev-list", "--count", "refs/remotes/origin/"+branch+"..HEAD")
	if err != nil || code != 0 {
		return false, commandError("git rev-list", stderr, code, err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return false, fmt.Errorf("git rev-list: unexpected output %q", strings.TrimSpace(stdout))
	}
	return n > 0, nil
}

// openPRFor returns the URL and base branch of the open pull request whose
// head is branch in this repository, or "" when there is none. Pull
// requests from forks with a branch of the same name are ignored.
func openPRFor(ctx context.Context, root string, env sys.Env, branch string) (string, string, error) {
	stdout, stderr, code, err := env.Run(ctx, root, "gh", "pr", "list", "--head", branch, "--state", "open",
		"--json", "url,baseRefName,isCrossRepository", "--limit", "5")
	if err != nil || code != 0 {
		return "", "", commandError("gh pr list", stderr, code, err)
	}
	var prs []struct {
		URL               string `json:"url"`
		BaseRefName       string `json:"baseRefName"`
		IsCrossRepository bool   `json:"isCrossRepository"`
	}
	if s := strings.TrimSpace(stdout); s != "" {
		if err := json.Unmarshal([]byte(s), &prs); err != nil {
			return "", "", fmt.Errorf("gh pr list: %w", err)
		}
	}
	for _, pr := range prs {
		if !pr.IsCrossRepository {
			return pr.URL, pr.BaseRefName, nil
		}
	}
	return "", "", nil
}

// lastURL returns the last stdout line that starts with https://.
func lastURL(stdout string) string {
	lines := strings.Split(strings.ReplaceAll(stdout, "\r\n", "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); strings.HasPrefix(l, "https://") {
			return l
		}
	}
	return ""
}

// commandError reports a failed external command with its stderr.
func commandError(what, stderr string, code int, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	msg := strings.TrimSpace(stderr)
	if msg == "" {
		return fmt.Errorf("%s failed with exit code %d", what, code)
	}
	return fmt.Errorf("%s failed: %s", what, msg)
}

var prNumberRx = regexp.MustCompile(`/pull/(\d+)/?$`)

// prNumber is the number at the end of a pull request URL, 0 without one.
func prNumber(url string) int {
	m := prNumberRx.FindStringSubmatch(strings.TrimSpace(url))
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

// revalidateStoredEvidence checks references read back from the draft
// metadata. They were normalized when saved, so they carry ExportName and
// no ExportPath; they are checked as stored data, never as fresh input, and
// a set that was edited into something invalid (or that carries a path
// again) is rejected so the draft is prepared anew.
func revalidateStoredEvidence(refs []EvidenceRef) ([]EvidenceRef, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	probe := make([]EvidenceRef, len(refs))
	for i, ref := range refs {
		if ref.ExportPath != "" {
			return nil, evidenceError(fmt.Sprintf("[%d].exportPath", i), "must not be stored")
		}
		if ref.ExportName != "" && baseName(ref.ExportName) != ref.ExportName {
			return nil, evidenceError(fmt.Sprintf("[%d].exportName", i), "must be a file name")
		}
		// Re-run the input rules with the stored name standing in for the
		// path; NormalizeEvidence reduces it back to the same name.
		ref.ExportPath, ref.ExportName = ref.ExportName, ""
		probe[i] = ref
	}
	return NormalizeEvidence(probe)
}

// sameEvidence compares normalized reference sets by their JSON, the form
// they are stored in, so nil and empty lists (and empty case lists) match.
func sameEvidence(a, b []EvidenceRef) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(ja) == string(jb)
}
