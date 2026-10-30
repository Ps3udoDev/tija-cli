// Package workflows loads, validates, and manages composite Tija workflows.
package workflows

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

// Step is a single action in a workflow.
type Step struct {
	Run             string `yaml:"run" json:"run"`
	ContinueOnError bool   `yaml:"continueOnError" json:"continueOnError"`
}

// Workflow is a sequence of Tija commands.
type Workflow struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Path        string `yaml:"-" json:"path,omitempty"`
	Steps       []Step `yaml:"steps" json:"steps"`
}

// StepResult captures the execution outcome of one step.
type StepResult struct {
	Run    string          `json:"run"`
	Exit   int             `json:"exit"`
	Output json.RawMessage `json:"output,omitempty"`
}

// WorkflowReport is the outcome of running an entire workflow.
type WorkflowReport struct {
	Name  string       `json:"name"`
	Steps []StepResult `json:"steps"`
	Exit  int          `json:"exit"`
}

// Allowed holds the allowed command prefixes for workflow steps.
var Allowed = []string{
	"check",
	"lint",
	"format",
	"test",
	"status",
	"doctor",
	"clean comments",
	"pr prepare",
	"pr create",
}

// Args splits the step's Run command string into arguments using sys.SplitArgs.
func Args(s Step) ([]string, error) {
	args := sys.SplitArgs(s.Run)
	if len(args) == 0 {
		return nil, errors.New("empty command")
	}
	return args, nil
}

// Load reads all .yaml and .yml files from .tija/workflows under root,
// ignoring .gitkeep.
func Load(root string) ([]Workflow, []error) {
	dir := filepath.Join(root, ".tija", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, []error{err}
	}

	var wfs []Workflow
	var errs []error

	for _, d := range entries {
		if d.IsDir() || d.Name() == ".gitkeep" {
			continue
		}
		ext := filepath.Ext(d.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		fullPath := filepath.Join(dir, d.Name())
		data, rerr := os.ReadFile(fullPath)
		if rerr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.Name(), rerr))
			continue
		}
		var w Workflow
		if uerr := yaml.Unmarshal(data, &w); uerr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.Name(), uerr))
			continue
		}
		w.Path = filepath.ToSlash(filepath.Join(".tija", "workflows", d.Name()))
		wfs = append(wfs, w)
	}

	slices.SortFunc(wfs, func(a, b Workflow) int {
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})

	return wfs, errs
}

// Validate checks that w has a non-empty name, at least one step, no nested "run",
// and all commands match Allowed prefixes.
func Validate(w Workflow) []error {
	var errs []error
	if strings.TrimSpace(w.Name) == "" {
		errs = append(errs, errors.New("workflow name is required"))
	}
	if len(w.Steps) == 0 {
		errs = append(errs, errors.New("workflow must have at least one step"))
		return errs
	}

	for _, step := range w.Steps {
		args, err := Args(step)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if args[0] == "run" {
			errs = append(errs, errors.New("nested run is not allowed"))
			continue
		}
		matched := false
		for _, allowed := range Allowed {
			parts := strings.Fields(allowed)
			if len(args) >= len(parts) {
				match := true
				for i := range parts {
					if args[i] != parts[i] {
						match = false
						break
					}
				}
				if match {
					matched = true
					break
				}
			}
		}
		if !matched {
			if (args[0] == "clean" || args[0] == "pr") && len(args) > 1 && !strings.HasPrefix(args[1], "-") {
				errs = append(errs, fmt.Errorf("unknown command %q", args[0]+" "+args[1]))
			} else {
				errs = append(errs, fmt.Errorf("unknown command %q", args[0]))
			}
		}
	}
	return errs
}
