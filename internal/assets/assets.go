// Package assets holds the default rules and templates embedded in the binary.
package assets

import (
	"bytes"
	"embed"
	"io/fs"
	"path"
	"strings"
)

// DefaultsVersion identifies the revision of the embedded defaults.
const DefaultsVersion = "2"

// Repo-relative paths of the embedded defaults.
const (
	RulesPath         = ".tija/rules/pull-request.md"
	TemplatePath      = ".tija/templates/pull-request.md"
	WorkflowsKeep     = ".tija/workflows/.gitkeep"
	SkillsDir         = ".tija/skills"
	PrePRWorkflowPath = ".tija/workflows/pre-pr.yaml"
)

//go:embed defaults
var defaultsFS embed.FS

// Files returns a fresh copy of the embedded defaults keyed by their
// repo-relative install path.
func Files() map[string][]byte {
	out := make(map[string][]byte)

	_ = fs.WalkDir(defaultsFS, "defaults", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := defaultsFS.ReadFile(p)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, "defaults/")
		target := ".tija/" + rel
		out[target] = bytes.Clone(data)
		return nil
	})

	out[WorkflowsKeep] = []byte{}
	return out
}

// Skill returns the embedded SKILL.md content for a given skill pack name,
// or false if not found.
func Skill(name string) ([]byte, bool) {
	clean := strings.TrimSuffix(name, "/SKILL.md")
	clean = strings.TrimPrefix(clean, ".tija/skills/")
	clean = strings.TrimPrefix(clean, "defaults/skills/")
	clean = strings.Trim(clean, "/")
	p := path.Join("defaults", "skills", clean, "SKILL.md")
	data, err := defaultsFS.ReadFile(p)
	if err != nil {
		return nil, false
	}
	return bytes.Clone(data), true
}
