// Package assets holds the default rules and templates embedded in the binary.
package assets

import _ "embed"

// DefaultsVersion identifies the revision of the embedded defaults.
const DefaultsVersion = "1"

// Repo-relative paths of the embedded defaults.
const (
	RulesPath     = ".tija/rules/pull-request.md"
	TemplatePath  = ".tija/templates/pull-request.md"
	WorkflowsKeep = ".tija/workflows/.gitkeep"
)

//go:embed defaults/rules/pull-request.md
var rules string

//go:embed defaults/templates/pull-request.md
var template string

// Files returns a fresh copy of the embedded defaults keyed by their
// repo-relative install path.
func Files() map[string][]byte {
	return map[string][]byte{
		RulesPath:     []byte(rules),
		TemplatePath:  []byte(template),
		WorkflowsKeep: {},
	}
}
