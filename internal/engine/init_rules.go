package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/assets"
	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/textdiff"
)

// AddRulesPresets explicitly requests presets independently of the detected stack.
// Callers approve the resulting opt-in actions by their stable IDs.
func (p *InitPlan) AddRulesPresets(names ...string) error {
	for _, name := range names {
		if _, ok := assets.Preset(name); !ok {
			return fmt.Errorf("unknown architecture rules preset %q (valid: %s)", name, strings.Join(assets.Presets(), ", "))
		}
	}
	p.rulesPresets = append(p.rulesPresets, names...)
	p.Actions = nil
	return p.planActions()
}

func (p *InitPlan) planRulesPresets(installed map[string]string) error {
	names := slices.Clone(p.rulesPresets)
	matches, err := filepath.Glob(filepath.Join(p.Root, filepath.FromSlash(assets.ArchitectureDir), "*.md"))
	if err != nil {
		return err
	}
	if len(matches) == 0 && len(names) == 0 {
		switch p.Detection.Stack {
		case detect.StackReactVite:
			names = append(names, "react-features")
		case detect.StackDotnetAPI:
			names = append(names, "dotnet-clean")
		}
	}
	slices.Sort(names)
	names = slices.Compact(names)
	for _, name := range names {
		rel := assets.ArchitectureDir + "/" + name + ".md"
		if slices.ContainsFunc(p.Actions, func(a Action) bool { return a.Path == rel }) {
			continue
		}
		// Existing files are handled by the installed-default mechanism; an
		// explicit preset never replaces an untracked or edited architecture file.
		if _, err := os.Stat(abs(p.Root, rel)); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		data, _ := assets.Preset(name)
		a := newAction(KindCreate, rel, true)
		a.OptIn = true
		a.Reason = "offer: architecture rules preset " + name
		a.data = data
		a.installed = true
		a.Diff = textdiff.Lines(nil, data)
		p.Actions = append(p.Actions, a)
	}
	return nil
}
