// Package manifest defines .tija/manifest.yaml: its schema, defaults,
// loading, validation and writing.
package manifest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/Ps3udoDev/tija-cli/internal/branch"
	"github.com/Ps3udoDev/tija-cli/internal/detect"
)

// Path is the repo-relative location of the manifest.
const Path = ".tija/manifest.yaml"

// Default install locations used when the repository has no own PR files.
const (
	DefaultTemplate = ".tija/templates/pull-request.md"
	DefaultRules    = ".tija/rules/pull-request.md"
)

const (
	version          = 1
	defaultMaxLength = 100
)

var (
	projectTypes = []string{string(detect.StackDotnetAPI), string(detect.StackReactVite), string(detect.StackUnknown)}
	agentNames   = []string{"claude", "codex", "gemini"}
	tiers        = []string{"fast", "balanced", "deep"}
)

// QualityProviders lists valid quality provider names.
var QualityProviders = []string{"biome", "eslint", "prettier", "dotnet", "dotnet-format", "dotnet-test", "vitest", "jest", "playwright", "script", "none"}

// QualityKinds lists valid quality kinds in standard order.
var QualityKinds = []string{"build", "lint", "format", "tests", "e2e"}

// Manifest is the content of .tija/manifest.yaml (schema version 1).
type Manifest struct {
	Version     int                     `yaml:"version"`
	Project     Project                 `yaml:"project"`
	Content     Content                 `yaml:"content"`
	Git         Git                     `yaml:"git"`
	PullRequest PullRequest             `yaml:"pullRequest"`
	Quality     map[string]QualityEntry `yaml:"quality"`
	Agents      Agents                  `yaml:"agents"`
	Tija        Meta                    `yaml:"tija"`
}

type Project struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Solution string `yaml:"solution,omitempty"`
}

type Content struct {
	Language string `yaml:"language"`
}

type Git struct {
	Host             string   `yaml:"host"`
	BranchConvention string   `yaml:"branchConvention"`
	BranchPrefixes   []string `yaml:"branchPrefixes"`
}

type PullRequest struct {
	Target           string   `yaml:"target"`
	Template         string   `yaml:"template"`
	Rules            string   `yaml:"rules"`
	Title            Title    `yaml:"title"`
	RequiredSections []string `yaml:"requiredSections"`
	MinSectionLength int      `yaml:"minSectionLength"`
	InvalidValues    []string `yaml:"invalidValues"`
	// AutoAssign is a pointer so that an explicit false differs from absent.
	AutoAssign *bool `yaml:"autoAssign"`
}

type Title struct {
	Convention string `yaml:"convention"`
	// RequireScope is a pointer so that an explicit false differs from absent.
	RequireScope *bool `yaml:"requireScope"`
	MaxLength    int   `yaml:"maxLength"`
}

type QualityEntry struct {
	Provider string `yaml:"provider"`
	Script   string `yaml:"script,omitempty"` // solo con provider "script"
}

type Agents struct {
	Default string          `yaml:"default"`
	Tasks   map[string]Task `yaml:"tasks"`
}

type Task struct {
	Tier string `yaml:"tier"`
}

// Meta is Tija's own bookkeeping.
type Meta struct {
	DefaultsVersion string `yaml:"defaultsVersion"`
	// Installed maps repo-relative '/' paths of files Tija installed to the
	// sha256 hex of the content it wrote.
	Installed map[string]string `yaml:"installed"`
}

// qualityByTool maps a detected tool to the quality entries it provides.
var qualityByTool = map[string]map[string]string{
	"biome":         {"lint": "biome", "format": "biome"},
	"eslint":        {"lint": "eslint"},
	"prettier":      {"format": "prettier"},
	"dotnet-format": {"format": "dotnet-format", "build": "dotnet"},
	"xunit":         {"tests": "dotnet-test"},
	"nunit":         {"tests": "dotnet-test"},
	"mstest":        {"tests": "dotnet-test"},
	"vitest":        {"tests": "vitest"},
	"jest":          {"tests": "jest"},
	"playwright":    {"e2e": "playwright"},
}

// DetectedQuality returns the quality mapping inferred from detected tools.
func DetectedQuality(d detect.Detection) map[string]QualityEntry {
	quality := map[string]QualityEntry{}
	tools := slices.Clone(d.Tools)
	slices.Sort(tools)
	for _, tool := range tools {
		for kind, provider := range qualityByTool[tool] {
			if _, taken := quality[kind]; !taken {
				quality[kind] = QualityEntry{Provider: provider}
			}
		}
	}
	return quality
}

// FromDetection builds a fresh manifest. agents are the installed agent
// providers in preference order; target is the PR target branch.
func FromDetection(d detect.Detection, agents []string, target string) Manifest {
	prefixes := slices.Clone(d.BranchPrefixes)
	if len(prefixes) == 0 {
		prefixes = slices.Clone(branch.DefaultPrefixes)
	}
	maxLen := d.CommitlintHeaderMax
	if maxLen <= 0 {
		maxLen = defaultMaxLength
	}
	tpl := d.PRTemplate
	if tpl == "" {
		tpl = DefaultTemplate
	}
	rules := d.PRGuide
	if rules == "" {
		rules = DefaultRules
	}
	quality := DetectedQuality(d)
	def := ""
	if len(agents) > 0 {
		def = agents[0]
	}
	yes, assign := true, true
	return Manifest{
		Version: version,
		Project: Project{Name: d.ProjectName, Type: string(d.Stack), Solution: d.Solution},
		Content: Content{Language: "es"},
		Git:     Git{Host: d.GitHost, BranchConvention: "conventional", BranchPrefixes: prefixes},
		PullRequest: PullRequest{
			Target:           target,
			Template:         tpl,
			Rules:            rules,
			Title:            Title{Convention: "conventional-commits", RequireScope: &yes, MaxLength: maxLen},
			RequiredSections: []string{"Resumen", "Objetivo", "Cambios realizados", "Pruebas"},
			MinSectionLength: 10,
			InvalidValues:    []string{"n/a", "na", "no aplica", "pendiente", "todo", "por definir", "...", "-"},
			AutoAssign:       &assign,
		},
		Quality: quality,
		Agents: Agents{Default: def, Tasks: map[string]Task{
			"pullRequest":  {Tier: "fast"},
			"review":       {Tier: "balanced"},
			"architecture": {Tier: "deep"},
		}},
	}
}

func file(root string) string { return filepath.Join(root, filepath.FromSlash(Path)) }

// Load reads the manifest of the repository at root. A missing file yields
// an error satisfying errors.Is(err, fs.ErrNotExist).
func Load(root string) (Manifest, error) {
	data, err := os.ReadFile(file(root))
	if err != nil {
		return Manifest{}, err
	}
	return Parse(data)
}

// Parse decodes manifest bytes. Decoding is lenient (unknown keys are
// allowed); schema problems are reported by Validate. Errors are prefixed
// with Path.
func Parse(data []byte) (Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", Path, err)
	}
	return m, nil
}

// Save writes m to root/.tija/manifest.yaml, creating .tija/ if needed.
func Save(root string, m Manifest) error {
	data, err := encode(m)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file(root)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file(root), data, 0o644)
}

func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Validate reports every schema violation, or nil when m is valid.
func (m Manifest) Validate() []error {
	var errs []error
	if m.Version != version {
		errs = append(errs, fmt.Errorf("version: unsupported value %d (want %d)", m.Version, version))
	}
	if !slices.Contains(projectTypes, m.Project.Type) {
		errs = append(errs, fmt.Errorf("project.type: invalid value %q (want one of %v)", m.Project.Type, projectTypes))
	}
	if m.PullRequest.Target == "" {
		errs = append(errs, fmt.Errorf("pullRequest.target: required"))
	}
	if m.Agents.Default != "" && !slices.Contains(agentNames, m.Agents.Default) {
		errs = append(errs, fmt.Errorf("agents.default: invalid value %q (want one of %v)", m.Agents.Default, agentNames))
	}
	names := make([]string, 0, len(m.Agents.Tasks))
	for name := range m.Agents.Tasks {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if tier := m.Agents.Tasks[name].Tier; !slices.Contains(tiers, tier) {
			errs = append(errs, fmt.Errorf("agents.tasks.%s.tier: invalid value %q (want one of %v)", name, tier, tiers))
		}
	}

	qualityKeys := make([]string, 0, len(m.Quality))
	for k := range m.Quality {
		qualityKeys = append(qualityKeys, k)
	}
	slices.Sort(qualityKeys)
	for _, k := range qualityKeys {
		if !slices.Contains(QualityKinds, k) {
			errs = append(errs, fmt.Errorf("quality.%s: unknown kind %q (want one of %v)", k, k, QualityKinds))
			continue
		}
		entry := m.Quality[k]
		if !slices.Contains(QualityProviders, entry.Provider) {
			errs = append(errs, fmt.Errorf("quality.%s.provider: invalid value %q (want one of %v)", k, entry.Provider, QualityProviders))
		}
		if entry.Provider == "script" && entry.Script == "" {
			errs = append(errs, fmt.Errorf("quality.%s.script: required for provider \"script\"", k))
		}
	}

	return errs
}
