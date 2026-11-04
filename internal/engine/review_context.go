package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/comments"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/review"
	"github.com/Ps3udoDev/tija-cli/internal/rules"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

func reviewPackExists(root string, m manifest.Manifest, name string) bool {
	p := ".tija/skills/" + name + "/SKILL.md"
	if ref, ok := m.Skills[name]; ok && ref.Path != "" {
		p = ref.Path
	}
	_, err := os.Stat(abs(root, p))
	return err == nil
}

func reviewFiles(ctx context.Context, repo gitx.Repo, base string, exclude []string) ([]review.FileDiff, error) {
	excluded := func(p string) bool {
		for _, g := range exclude {
			if comments.Match(g, p) {
				return true
			}
		}
		return false
	}
	files := review.ParseDiff(repo.WorkingDiff(ctx, base))
	for _, p := range repo.Untracked(ctx) {
		if excluded(p) {
			continue
		}
		data, err := os.ReadFile(abs(repo.Dir, p))
		if err != nil {
			return nil, err
		}
		if bytes.IndexByte(data[:min(len(data), 8192)], 0) >= 0 {
			continue
		}
		files = append(files, review.SyntheticAdded(p, string(data)))
	}
	var out []review.FileDiff
	for _, f := range files {
		if !excluded(f.Path) && !f.Binary && !f.Deleted {
			out = append(out, f)
		}
	}
	return out, nil
}

func reviewReadLines(root, path string) ([]string, error) {
	// Only paths already parsed from git are ever passed by verification.
	data, err := os.ReadFile(abs(root, path))
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if text == "" {
		return []string{}, nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n"), nil
}

func reviewQuality(ctx context.Context, root string, env sys.Env, req ReviewRequest, rep *ReviewReport, emit func(ReviewEvent)) ([]string, error) {
	rep.Quality.Status = "skipped"
	if req.NoCheck {
		return nil, nil
	}
	emit(ReviewEvent{Stage: StageQuality, Message: "checking quality results"})
	var steps []quality.StepResult
	if !rep.Dirty {
		if s, e := quality.LoadSession(root); e == nil {
			steps = FreshTests(s, rep.Commit)
			rep.Quality.Reused = len(steps) > 0
		}
	}
	if len(steps) == 0 {
		r, err := RunQuality(ctx, root, env, QualityRequest{Mode: quality.CheckMode, Timeout: req.CheckTimeout, Ephemeral: true}, func(ev QualityEvent) { emit(ReviewEvent{Stage: StageQuality, Message: ev.Step.Command, Quality: &ev}) })
		if err != nil {
			return nil, err
		}
		steps = r.Steps
		rep.Warnings = append(rep.Warnings, r.Warnings...)
	}
	lines := []string{}
	if len(steps) > 0 {
		rep.Quality.Status = "passed"
	}
	for _, s := range steps {
		lines = append(lines, describeResult(s))
		if s.Status == quality.StatusFailed {
			rep.Quality.Status = "failed"
			rep.Findings = append(rep.Findings, review.Finding{Source: "quality", File: filepath.ToSlash(s.Log), Severity: review.High, Rule: "quality-check-failed", Title: "Quality check failed", Explanation: describeResult(s)})
		} else if s.Status != quality.StatusPassed && rep.Quality.Status != "failed" {
			rep.Quality.Status = "unavailable"
		}
	}
	return lines, nil
}

func reviewChecks(ctx context.Context, repo gitx.Repo, base string, files []review.FileDiff, set rules.Set, rep *ReviewReport) error {
	var changed []rules.Changed
	var counts []review.LineCount
	for _, f := range files {
		lines, err := reviewReadLines(repo.Dir, f.Path)
		if err != nil {
			return err
		}
		added := map[int]string{}
		for _, n := range f.AddedLines() {
			if n > 0 && n <= len(lines) {
				added[n] = lines[n-1]
			}
		}
		changed = append(changed, rules.Changed{Path: f.Path, Added: added})
		if rep.Mode != "review" {
			before, _ := repo.LineCountAt(ctx, base, f.Path)
			counts = append(counts, review.LineCount{Path: f.Path, Before: before, After: len(lines)})
		}
	}
	for _, v := range set.Run(changed) {
		sev, _ := review.ParseSeverity(v.Severity)
		rep.Findings = append(rep.Findings, review.Finding{Source: "check", Rule: v.CheckID, Severity: sev, File: v.File, Line: v.Line, EndLine: v.Line, Title: v.Message, Explanation: v.Message, Evidence: v.Text})
	}
	rep.Findings = append(rep.Findings, review.Ceiling(counts)...)
	return nil
}

func reviewPRComments(ctx context.Context, repo gitx.Repo, env sys.Env, rep *ReviewReport) {
	warn := func(err error) { rep.Warnings = append(rep.Warnings, "PR comments skipped: "+oneLine(err)) }
	if err := checkGH(ctx, repo.Dir, env); err != nil {
		warn(err)
		return
	}
	branch, _, err := repo.CurrentBranch(ctx)
	if err != nil {
		warn(err)
		return
	}
	url, _, err := openPRFor(ctx, repo.Dir, env, branch)
	if err != nil {
		warn(err)
		return
	}
	if url == "" {
		rep.Warnings = append(rep.Warnings, "PR comments skipped: no open pull request")
		return
	}
	// A PR URL supplies its actual owner/repo even when origin uses SSH.
	parts := strings.Split(strings.TrimSuffix(url, "/"), "/")
	if len(parts) < 5 || parts[len(parts)-2] != "pull" {
		warn(fmt.Errorf("invalid pull request URL %q", url))
		return
	}
	n := len(parts)
	endpoint := fmt.Sprintf("repos/%s/%s/pulls/%s/comments", parts[n-4], parts[n-3], parts[n-1])
	out, stderr, code, err := env.Run(ctx, repo.Dir, "gh", "api", endpoint, "--paginate", "--slurp")
	if err != nil || code != 0 {
		warn(commandError("gh api", stderr, code, err))
		return
	}
	type comment struct {
		Path         string
		Line         *int
		OriginalLine *int `json:"original_line"`
		Body         string
		URL          string `json:"html_url"`
		User         struct{ Login string }
	}
	// gh --slurp returns an array of page arrays. Accept a single page too.
	var pages [][]comment
	if err = json.Unmarshal([]byte(out), &pages); err != nil {
		var page []comment
		if err = json.Unmarshal([]byte(out), &page); err != nil {
			warn(err)
			return
		}
		pages = [][]comment{page}
	}
	var all []review.PRComment
	for _, page := range pages {
		for _, c := range page {
			line := c.Line
			if line == nil {
				line = c.OriginalLine
			}
			if line == nil || *line <= 0 {
				continue
			}
			all = append(all, review.PRComment{Path: c.Path, Line: *line, Author: c.User.Login, Body: c.Body, URL: c.URL})
		}
	}
	rep.Findings, rep.PRComments.Uncovered = review.AttachPRComments(rep.Findings, all)
	rep.PRComments.Read = true
}

// Agent prose and rule messages already carry their requested language. These
// two built-in checks have no user-authored text, so localize them here.
func localizeReviewFindings(language string, rep *ReviewReport) {
	if language != "" && !strings.EqualFold(language, "es") {
		return
	}
	for i := range rep.Findings {
		f := &rep.Findings[i]
		switch f.Rule {
		case "quality-check-failed":
			f.Title = "Falló una verificación de calidad"
		case "1000-lines-ceiling":
			f.Title = "El archivo supera las 1000 líneas"
			f.Explanation = "El archivo pasó de 1000 líneas o menos a más de 1000 líneas."
			f.Suggestion = "Divide el archivo según sus responsabilidades."
		}
	}
}
