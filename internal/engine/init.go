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
	existing     *manifest.Manifest // parsed manifest on disk, nil if absent or invalid
	existingData []byte             // its raw bytes
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
		if m, perr := manifest.Parse(data); perr != nil {
			p.notes = append(p.notes, "manifest is invalid, fix it manually: "+perr.Error())
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
	if p.Actions, err = p.planActions(); err != nil {
		return InitPlan{}, err
	}
	return p, nil
}

func (p InitPlan) planActions() ([]Action, error) {
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

	var actions []Action
	for _, path := range append(paths, extra...) {
		data, _ := source(files, path)
		a, ok, err := fileAction(p.Root, path, data, installed[path])
		if err != nil {
			return nil, err
		}
		if ok {
			actions = append(actions, a)
		}
	}

	switch {
	case !p.AlreadyInitialized:
		actions = append(actions, newAction(KindCreate, manifest.Path, false))
	case p.existing != nil:
		if fields := manifest.MissingFields(*p.existing, p.Manifest); len(fields) > 0 {
			merged, err := manifest.FillMissing(p.existingData, p.Manifest, fields)
			if err != nil {
				return nil, err
			}
			a := newAction(KindUpdate, manifest.Path, true)
			a.Reason = "add missing fields: " + strings.Join(fields, ", ")
			a.Diff = diffLines(p.existingData, merged)
			actions = append(actions, a)
		}
	}

	cur, err := readOptional(abs(p.Root, gitignorePath))
	if err != nil {
		return nil, err
	}
	if out, added := mergeGitignore(cur, gitignoreEntries); len(added) > 0 {
		kind := KindUpdate
		if cur == nil {
			kind = KindCreate
		}
		a := newAction(kind, gitignorePath, true)
		a.Reason = "ignore local .tija/ paths"
		a.Diff = diffLines(cur, out)
		a.data = out
		actions = append(actions, a)
	}

	if p.Detection.PRTemplate == "" && !slices.Contains(extra, githubTemplate) {
		if _, err := os.Stat(abs(p.Root, githubTemplate)); errors.Is(err, fs.ErrNotExist) {
			a := newAction(KindCreate, githubTemplate, true)
			a.Reason = offerReason
			a.data = files[assets.TemplatePath]
			a.Diff = diffLines(nil, a.data)
			a.installed = true
			actions = append(actions, a)
		}
	}
	return actions, nil
}

// source returns the embedded content installed at path.
func source(files map[string][]byte, path string) ([]byte, bool) {
	if path == githubTemplate {
		path = assets.TemplatePath
	}
	data, ok := files[path]
	return data, ok
}

// fileAction plans installing data at path. installedHash is the sha256
// recorded when Tija last wrote the file ("" if unknown). ok is false when
// the file already holds data.
func fileAction(root, path string, data []byte, installedHash string) (Action, bool, error) {
	cur, err := readOptional(abs(root, path))
	if err != nil {
		return Action{}, false, err
	}
	outside := !strings.HasPrefix(path, ".tija/")
	var a Action
	switch {
	case cur == nil:
		a = newAction(KindCreate, path, outside)
		a.Reason = "install Tija default"
	case bytes.Equal(cur, data):
		return Action{}, false, nil
	case installedHash != "" && hash(cur) == installedHash:
		a = newAction(KindReplace, path, outside)
		a.Reason = "upgrade unedited Tija default to v" + assets.DefaultsVersion
		a.Diff = diffLines(cur, data)
	default:
		a = newAction(KindReplace, path, true)
		a.Reason = "file differs from Tija default v" + assets.DefaultsVersion + " (edited?)"
		a.Diff = diffLines(cur, data)
	}
	a.data, a.installed = data, true
	return a, true, nil
}

func newAction(kind ActionKind, path string, consent bool) Action {
	return Action{ID: string(kind) + ":" + path, Kind: kind, Path: path, NeedsConsent: consent}
}

// ApplyInit runs the actions that need no consent plus the approved ones;
// the rest are returned as Pending. Choices are resolved from answers or
// their defaults.
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
	installed := map[string]string{}
	writeManifest := false
	for _, a := range plan.Actions {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if a.NeedsConsent && !approved[a.ID] {
			res.Pending = append(res.Pending, a)
			continue
		}
		if a.Path == manifest.Path {
			writeManifest = true
			continue
		}
		if err := writeFile(plan.Root, a.Path, a.data); err != nil {
			return res, err
		}
		if a.installed {
			installed[a.Path] = hash(a.data)
		}
		res.record(a)
	}

	switch {
	case !plan.AlreadyInitialized:
		m.Tija = manifest.Meta{DefaultsVersion: assets.DefaultsVersion, Installed: installed}
		if err := manifest.Save(plan.Root, m); err != nil {
			return res, err
		}
		res.Created = append(res.Created, manifest.Path)
	case plan.existing != nil:
		out := plan.existingData
		var err error
		if writeManifest {
			if out, err = manifest.FillMissing(out, m, manifest.MissingFields(*plan.existing, m)); err != nil {
				return res, err
			}
		}
		if len(installed) > 0 {
			if out, err = manifest.RecordInstalled(out, assets.DefaultsVersion, installed); err != nil {
				return res, err
			}
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

func hash(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// diffLines renders the changed middle of old→new as "-"/"+" lines, after
// dropping the lines both share at the start and the end.
func diffLines(old, new []byte) string {
	a, b := splitLines(old), splitLines(new)
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	var out []string
	for _, l := range a[pre : len(a)-suf] {
		out = append(out, "-"+l)
	}
	for _, l := range b[pre : len(b)-suf] {
		out = append(out, "+"+l)
	}
	return strings.Join(out, "\n")
}

func splitLines(b []byte) []string {
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
