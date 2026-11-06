package workflows

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Executor runs one step's argv (without the leading "tija") and returns its
// exit code. The CLI and the MCP server provide their own, so both share the
// step loop below and every step reaches the engine through the CLI command.
type Executor func(ctx context.Context, args []string, out, errOut io.Writer) int

// Options configures Run.
type Options struct {
	// JSON adds --json to every step and captures each step's output into
	// StepResult.Output; nothing is written to Out.
	JSON bool
	// Yes adds --yes so steps that ask for consent proceed.
	Yes bool
	// Out receives "> <run>" and the steps' output when JSON is false.
	Out io.Writer
	// OnStep is called after each step.
	OnStep func(i int, r StepResult)
}

// Run executes the steps of w in order with exec. It stops at the first
// failing step unless the step sets ContinueOnError; the report's Exit is the
// highest step exit code.
func Run(ctx context.Context, w Workflow, exec Executor, o Options) WorkflowReport {
	out := o.Out
	if out == nil {
		out = io.Discard
	}
	report := WorkflowReport{Name: w.Name, Steps: make([]StepResult, 0, len(w.Steps))}
	for i, step := range w.Steps {
		res := runStep(ctx, step, exec, o, out)
		report.Steps = append(report.Steps, res)
		report.Exit = max(report.Exit, res.Exit)
		if o.OnStep != nil {
			o.OnStep(i, res)
		}
		if res.Exit != 0 && !step.ContinueOnError {
			break
		}
	}
	return report
}

func runStep(ctx context.Context, step Step, exec Executor, o Options, out io.Writer) StepResult {
	args, err := Args(step)
	if err != nil {
		if !o.JSON {
			fmt.Fprintf(out, "✗ %s\n", err)
		}
		return StepResult{Run: step.Run, Exit: 2}
	}
	if o.JSON && !slices.Contains(args, "--json") {
		args = append(args, "--json")
	}
	if o.Yes && !slices.Contains(args, "--yes") {
		args = append(args, "--yes")
	}
	if !o.JSON {
		fmt.Fprintf(out, "> %s\n", step.Run)
		return StepResult{Run: step.Run, Exit: exec(ctx, args, out, out)}
	}
	var buf bytes.Buffer
	res := StepResult{Run: step.Run, Exit: exec(ctx, args, &buf, &buf)}
	if trimmed := bytes.TrimSpace(buf.Bytes()); len(trimmed) > 0 {
		if json.Valid(trimmed) {
			res.Output = json.RawMessage(trimmed)
		} else {
			raw, _ := json.Marshal(string(trimmed))
			res.Output = json.RawMessage(raw)
		}
	}
	return res
}

// Effect is what a step can change, from nothing to publishing; the order
// matches the MCP effect classes.
type Effect int

const (
	EffectRead Effect = iota
	EffectLocal
	EffectRepo
	EffectExternal
)

// String is the name used in MCP results.
func (e Effect) String() string {
	switch e {
	case EffectRead:
		return "read"
	case EffectLocal:
		return "local"
	case EffectRepo:
		return "repo"
	case EffectExternal:
		return "external"
	}
	return "unknown"
}

// Classify says what a step can change and whether it applies unsafe
// changes. Steps that need a terminal (pr create --edit opens an editor)
// are an error: they cannot run without one.
func Classify(s Step) (Effect, bool, error) {
	args, err := Args(s)
	if err != nil {
		return 0, false, err
	}
	cmd := command(args)
	switch cmd {
	case "status", "doctor":
		return EffectRead, false, nil
	case "review", "check", "test", "pr prepare":
		return EffectLocal, false, nil
	case "lint":
		if hasFlag(args, "fix") {
			return EffectRepo, false, nil
		}
		return EffectLocal, false, nil
	case "format":
		return EffectRepo, false, nil
	case "clean comments":
		if hasFlag(args, "write") {
			return EffectRepo, hasFlag(args, "unsafe"), nil
		}
		return EffectLocal, false, nil
	case "pr create":
		if hasFlag(args, "edit") {
			return 0, false, fmt.Errorf("step %q opens an editor and needs a terminal", s.Run)
		}
		return EffectExternal, false, nil
	}
	return 0, false, fmt.Errorf("step %q is not an allowed command", s.Run)
}

// StepEffect is one step and what it can change; Effect is the class name
// (read, local, repo, external) and Class the same value as an Effect.
type StepEffect struct {
	Run    string `json:"run"`
	Effect string `json:"effect"`
	Unsafe bool   `json:"unsafe,omitempty"`
	Class  Effect `json:"-"`
}

// Needs is what running a workflow requires: write for repository changes
// or publishing, unsafe for unsafe changes, and publish for a pull request.
type Needs struct {
	Write, Unsafe, Publish bool
	Steps                  []StepEffect
}

// Requirements classifies every step of w before anything runs.
func Requirements(w Workflow) (Needs, error) {
	var n Needs
	for _, s := range w.Steps {
		e, unsafe, err := Classify(s)
		if err != nil {
			return Needs{}, err
		}
		n.Steps = append(n.Steps, StepEffect{Run: s.Run, Effect: e.String(), Unsafe: unsafe, Class: e})
		n.Write = n.Write || e >= EffectRepo
		n.Unsafe = n.Unsafe || unsafe
		n.Publish = n.Publish || e == EffectExternal
	}
	return n, nil
}

// command is the allowed command an argv starts with ("clean comments",
// "pr create", "check"…), or "" when it starts with none.
func command(args []string) string {
	best := ""
	for _, allowed := range Allowed {
		parts := strings.Fields(allowed)
		if len(args) >= len(parts) && slices.Equal(args[:len(parts)], parts) && len(allowed) > len(best) {
			best = allowed
		}
	}
	return best
}

// hasFlag reports whether args turn --name on (--name or --name=<not false>).
func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == "--"+name {
			return true
		}
		if v, ok := strings.CutPrefix(a, "--"+name+"="); ok && v != "false" {
			return true
		}
	}
	return false
}
