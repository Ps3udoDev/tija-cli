// Package engine implements Tija's commands as typed functions; the CLI and
// TUI are thin layers on top of it.
package engine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/agents"
	"github.com/Ps3udoDev/tija-cli/internal/assets"
	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/textdiff"
)

// ActionKind says what an Action does to its file.
type ActionKind string

const (
	KindCreate  ActionKind = "create"
	KindUpdate  ActionKind = "update"
	KindReplace ActionKind = "replace"
)

// Choice IDs.
const (
	ChoiceSolution = "solution"
	ChoiceTarget   = "target"
)

const (
	githubTemplate = ".github/PULL_REQUEST_TEMPLATE.md"
	gitignorePath  = ".gitignore"
	offerReason    = "offer: copy PR template to .github"
)

// ErrNotARepo is returned when init runs outside a git repository.
var ErrNotARepo = errors.New("not a git repository")

// Action is one planned file change. ID is "<kind>:<path>" and is stable, so
// callers can approve actions by ID.
type Action struct {
	ID           string     `json:"id"`
	Kind         ActionKind `json:"kind"`
	Path         string     `json:"path"`
	Diff         string     `json:"diff,omitempty"`
	Reason       string     `json:"reason,omitempty"`
	NeedsConsent bool       `json:"needsConsent"`

	data      []byte // content written by file actions
	installed bool   // record the written content in tija.installed
}

// Choice is a question init needs answered; Default is used when unanswered.
type Choice struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Options  []string `json:"options"`
	Default  string   `json:"default"`
}

// InitPlan is what init would do. It is computed without writing anything.
type InitPlan struct {
	Root               string            `json:"root"`
	Detection          detect.Detection  `json:"detection"`
	Manifest           manifest.Manifest `json:"manifest"`
	Actions            []Action          `json:"actions"`
	Choices            []Choice          `json:"choices"`
	AlreadyInitialized bool              `json:"alreadyInitialized"`

	agents       []string           // installed agent providers
	existing     *manifest.Manifest // manifest on disk, nil if absent or invalid
	existingData []byte             // its raw bytes
	refresh      map[string]string  // stale tija.installed entries of files equal to the default
	adoptOffer   bool               // an earlier run copied the .github template; point the manifest to it
	notes        []string
}

// InitResult reports what ApplyInit did. Paths are repo-relative with '/'.
type InitResult struct {
	Detection detect.Detection `json:"detection"`
	Created   []string         `json:"created"`
	Updated   []string         `json:"updated"`
	Pending   []Action         `json:"pending"`
	Notes     []string         `json:"notes"`
}

// PlanInit inspects the repository containing dir and plans init at its root.
func PlanInit(ctx context.Context, dir string, env sys.Env) (InitPlan, error) {
	root, ok, err := gitx.FindRoot(ctx, env.Run, dir)
	if err != nil {
		return InitPlan{}, err
	}
	if !ok {
		return InitPlan{}, fmt.Errorf("%w: %s", ErrNotARepo, dir)
	}
	repo := gitx.Repo{Dir: root, Run: env.Run}
	d, err := detect.Detect(root, repo.RemoteURL(ctx))
	if err != nil {
		return InitPlan{}, err
	}
	p := InitPlan{Root: root, agents: agents.Available(env.LookPath)}

	data, err := os.ReadFile(abs(root, manifest.Path))
	switch {
	case err == nil:
		p.AlreadyInitialized = true
		if m, problem := parseValid(data); problem != "" {
			p.notes = append(p.notes, "manifest is invalid, fix it manually: "+problem)
		} else {
			p.existing, p.existingData = &m, data
		}
	case !errors.Is(err, fs.ErrNotExist):
		return InitPlan{}, err
	}

	if len(d.Solutions) > 1 {
		sol := d.Solutions[0]
		if p.existing != nil && slices.Contains(d.Solutions, p.existing.Project.Solution) {
			sol = p.existing.Project.Solution
		} else {
			p.Choices = append(p.Choices, Choice{ID: ChoiceSolution, Question: "Which solution should Tija use?", Options: slices.Clone(d.Solutions), Default: sol})
		}
		if d, err = d.WithSolution(sol); err != nil {
			return InitPlan{}, err
		}
	}
	target := defaultTarget(ctx, repo)
	if p.existing == nil || p.existing.PullRequest.Target == "" {
		if c, ok := targetChoice(ctx, repo, target); ok {
			p.Choices = append(p.Choices, c)
		}
	}
	if len(d.PRTemplateCandidates) > 1 {
		p.notes = append(p.notes, fmt.Sprintf("multiple PR templates found; using %s", d.PRTemplate))
	}
	p.Detection = d
	p.Manifest = manifest.FromDetection(d, p.agents, target)
	if err = p.planActions(); err != nil {
		return InitPlan{}, err
	}
	return p, nil
}

// parseValid parses a manifest and validates it. problem is "" when the
// manifest is usable.
func parseValid(data []byte) (manifest.Manifest, string) {
	m, err := manifest.Parse(data)
	if err != nil {
		return m, err.Error()
	}
	if errs := m.Validate(); len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return m, strings.Join(msgs, "; ")
	}
	return m, ""
}

func (p *InitPlan) planActions() error {
	files := assets.Files()
	var installed map[string]string
	if p.existing != nil {
		installed = p.existing.Tija.Installed
	}

	// Defaults are installed only where the repo lacks its own; files Tija
	// installed before are kept up to date.
	var paths []string
	if p.Detection.PRGuide == "" {
		paths = append(paths, assets.RulesPath)
	}
	if p.Detection.PRTemplate == "" {
		paths = append(paths, assets.TemplatePath)
	}
	paths = append(paths, assets.WorkflowsKeep)
	var extra []string
	for path := range installed {
		if _, ok := source(files, path); ok && !slices.Contains(paths, path) {
			extra = append(extra, path)
		}
	}
	slices.Sort(extra)

	p.refresh = map[string]string{}
	for _, path := range append(paths, extra...) {
		data, _ := source(files, path)
		a, ok, stale, err := fileAction(p.Root, path, data, installed[path])
		if err != nil {
			return err
		}
		if ok {
			p.Actions = append(p.Actions, a)
		}
		if stale {
			p.refresh[path] = hash(data)
		}
	}

	// The offer is planned before the manifest so that its bookkeeping
	// (tija.installed and pullRequest.template) is part of the manifest diff.
	if p.Detection.PRTemplate == "" && !slices.Contains(extra, githubTemplate) {
		if _, err := os.Stat(abs(p.Root, githubTemplate)); errors.Is(err, fs.ErrNotExist) {
			a := newAction(KindCreate, githubTemplate, true)
			a.Reason = offerReason
			a.data = files[assets.TemplatePath]
			a.Diff = textdiff.Lines(nil, a.data)
			a.installed = true
			p.Actions = append(p.Actions, a)
		}
	}
	if err := p.planAdoptedOffer(files[assets.TemplatePath], installed); err != nil {
		return err
	}

	switch {
	case !p.AlreadyInitialized:
		p.Actions = append(p.Actions, newAction(KindCreate, manifest.Path, false))
	case p.existing != nil:
		// Planned as if every file action ran and the choices keep their
		// defaults; ApplyInit recomputes the bytes from what actually ran.
		recorded := p.installedBy(p.Actions)
		out, fields, err := p.updatedManifest(p.Manifest, recorded, p.templateBy(p.Actions))
		if err != nil {
			return err
		}
		if !bytes.Equal(out, p.existingData) {
			a := newAction(KindUpdate, manifest.Path, true)
			var why []string
			if len(fields) > 0 {
				why = append(why, "add missing fields: "+strings.Join(fields, ", "))
			}
			if len(recorded) > 0 {
				why = append(why, "record installed defaults")
			}
			if t := p.templateBy(p.Actions); t != "" && t != p.existing.PullRequest.Template {
				why = append(why, "point pullRequest.template to "+t)
			}
			a.Reason = strings.Join(why, "; ")
			a.Diff = textdiff.Lines(p.existingData, out)
			p.Actions = append(p.Actions, a)
		}
	}

	cur, err := readOptional(abs(p.Root, gitignorePath))
	if err != nil {
		return err
	}
	if out, added := mergeGitignore(cur, gitignoreEntries); len(added) > 0 {
		kind := KindUpdate
		if cur == nil {
			kind = KindCreate
		}
		a := newAction(kind, gitignorePath, true)
		a.Reason = "ignore local .tija/ paths"
		a.Diff = textdiff.Lines(cur, out)
		p.Actions = append(p.Actions, a)
	}
	return nil
}

// planAdoptedOffer handles a .github template copied by an earlier run
// whose manifest update was declined: when the manifest still points to the
// .tija default and the .github file is the unedited default, the manifest
// is pointed to it and its hash recorded.
func (p *InitPlan) planAdoptedOffer(def []byte, installed map[string]string) error {
	if p.existing == nil || p.Detection.PRTemplate != githubTemplate ||
		p.existing.PullRequest.Template != manifest.DefaultTemplate {
		return nil
	}
	cur, err := readOptional(abs(p.Root, githubTemplate))
	if err != nil || cur == nil || hash(cur) != hash(def) {
		return err
	}
	p.adoptOffer = true
	if installed[githubTemplate] != hash(def) {
		p.refresh[githubTemplate] = hash(def)
	}
	return nil
}

// templateBy returns the pullRequest.template the manifest must point to
// after running actions: the .github template when the offer runs (or was
// adopted), else "" (keep the current value).
func (p InitPlan) templateBy(actions []Action) string {
	if p.adoptOffer {
		return githubTemplate
	}
	for _, a := range actions {
		if a.Path == githubTemplate && a.Reason == offerReason {
			return githubTemplate
		}
	}
	return ""
}

// installedBy returns the tija.installed entries produced by running actions,
// plus the stale entries to refresh.
func (p InitPlan) installedBy(actions []Action) map[string]string {
	out := map[string]string{}
	for path, h := range p.refresh {
		out[path] = h
	}
	for _, a := range actions {
		if a.installed {
			out[a.Path] = hash(a.data)
		}
	}
	return out
}

// updatedManifest returns the existing manifest bytes with the fields missing
// from fresh filled in and the installed entries recorded. A non-empty
// template replaces pullRequest.template when it still is the .tija default.
func (p InitPlan) updatedManifest(fresh manifest.Manifest, installed map[string]string, template string) ([]byte, []string, error) {
	out := p.existingData
	fields := manifest.MissingFields(*p.existing, fresh)
	var err error
	if len(fields) > 0 {
		if out, err = manifest.FillMissing(out, fresh, fields); err != nil {
			return nil, nil, err
		}
	}
	if len(installed) > 0 {
		if out, err = manifest.RecordInstalled(out, assets.DefaultsVersion, installed); err != nil {
			return nil, nil, err
		}
	}
	if cur := p.existing.PullRequest.Template; template != "" && (cur == "" || cur == manifest.DefaultTemplate) {
		if out, err = manifest.SetString(out, "pullRequest.template", template); err != nil {
			return nil, nil, err
		}
	}
	return out, fields, nil
}

// source returns the embedded content installed at path.
func source(files map[string][]byte, path string) ([]byte, bool) {
	if path == githubTemplate {
		path = assets.TemplatePath
	}
	data, ok := files[path]
	return data, ok
}

// fileAction plans installing the default data at path. recorded is the
// hash in tija.installed ("" if unknown). Content is compared and hashed
// with CRLF normalized to LF. ok is false when nothing must be written; stale
// reports a file equal to the default whose recorded hash is outdated.
//
// A file the user edited is only offered for replacement (with consent)
// when a newer default exists; an unedited one is upgraded silently inside
// .tija/.
func fileAction(root, path string, data []byte, recorded string) (a Action, ok, stale bool, err error) {
	cur, err := readOptional(abs(root, path))
	if err != nil {
		return Action{}, false, false, err
	}
	outside := !strings.HasPrefix(path, ".tija/")
	hCur, hDef := hash(cur), hash(data)
	switch {
	case cur == nil:
		a = newAction(KindCreate, path, outside)
		a.Reason = "install Tija default"
	case hCur == hDef:
		return Action{}, false, recorded != "" && recorded != hDef, nil
	case recorded == hDef:
		return Action{}, false, false, nil // edited by the user, no newer default
	case recorded != "" && hCur == recorded:
		a = newAction(KindReplace, path, outside)
		a.Reason = "upgrade unedited Tija default to v" + assets.DefaultsVersion
		a.Diff = textdiff.Lines(cur, data)
	default:
		a = newAction(KindReplace, path, true)
		a.Reason = "file differs from Tija default v" + assets.DefaultsVersion + " (edited?)"
		a.Diff = textdiff.Lines(cur, data)
	}
	a.data, a.installed = data, true
	return a, true, false, nil
}

func newAction(kind ActionKind, path string, consent bool) Action {
	return Action{ID: string(kind) + ":" + path, Kind: kind, Path: path, NeedsConsent: consent}
}

// ApplyInit runs the actions that need no consent plus the approved ones;
// the rest are returned as Pending. Choices are resolved from answers or
// their defaults.
//
// The manifest update and .gitignore merge are recomputed from what actually
// runs: with default answers and every action approved, the manifest bytes
// written are exactly those shown in the plan's Diff.
func ApplyInit(ctx context.Context, plan InitPlan, answers map[string]string, approved map[string]bool) (InitResult, error) {
	d, m := plan.Detection, plan.Manifest
	if sol := answers[ChoiceSolution]; sol != "" && sol != d.Solution {
		var err error
		if d, err = d.WithSolution(sol); err != nil {
			return InitResult{}, err
		}
		m = manifest.FromDetection(d, plan.agents, m.PullRequest.Target)
	}
	if t := answers[ChoiceTarget]; t != "" {
		m.PullRequest.Target = t
	}

	res := InitResult{Detection: d, Notes: slices.Clone(plan.notes)}
	var ran []Action
	writeManifest := false
	for _, a := range plan.Actions {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if a.NeedsConsent && !approved[a.ID] {
			res.Pending = append(res.Pending, a)
			continue
		}
		switch a.Path {
		case manifest.Path:
			writeManifest = true
			continue
		case gitignorePath:
			cur, err := readOptional(abs(plan.Root, gitignorePath))
			if err != nil {
				return res, err
			}
			out, added := mergeGitignore(cur, gitignoreEntries)
			if len(added) == 0 {
				continue
			}
			a.Kind = KindUpdate
			if cur == nil {
				a.Kind = KindCreate
			}
			a.data = out
		}
		if err := writeFile(plan.Root, a.Path, a.data); err != nil {
			return res, err
		}
		ran = append(ran, a)
		res.record(a)
	}

	installed, template := plan.installedBy(ran), plan.templateBy(ran)
	switch {
	case !plan.AlreadyInitialized:
		m.Tija = manifest.Meta{DefaultsVersion: assets.DefaultsVersion, Installed: installed}
		if template != "" {
			m.PullRequest.Template = template
		}
		if err := manifest.Save(plan.Root, m); err != nil {
			return res, err
		}
		res.Created = append(res.Created, manifest.Path)
	case plan.existing != nil && writeManifest:
		out, _, err := plan.updatedManifest(m, installed, template)
		if err != nil {
			return res, err
		}
		if !bytes.Equal(out, plan.existingData) {
			if err := writeFile(plan.Root, manifest.Path, out); err != nil {
				return res, err
			}
			res.Updated = append(res.Updated, manifest.Path)
		}
	}
	return res, nil
}

func (r *InitResult) record(a Action) {
	if a.Kind == KindCreate {
		r.Created = append(r.Created, a.Path)
	} else {
		r.Updated = append(r.Updated, a.Path)
	}
}

func abs(root, rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

// readOptional returns the file's content, or nil when it does not exist.
func readOptional(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if b == nil && err == nil {
		b = []byte{}
	}
	return b, err
}

// writeFile writes data as-is (no line-ending conversion).
func writeFile(root, rel string, data []byte) error {
	p := abs(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	return nil
}

// hash is the sha256 hex of b with CRLF normalized to LF, so that a
// checkout with core.autocrlf does not look like an edit.
func hash(b []byte) string {
	s := sha256.Sum256(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")))
	return hex.EncodeToString(s[:])
}
