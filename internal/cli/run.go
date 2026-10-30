package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/spf13/cobra"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/render"
	"github.com/Ps3udoDev/tija-cli/internal/workflows"
)

// StepResult captures the outcome of one executed step.
type StepResult = workflows.StepResult

// WorkflowReport is the outcome of running an entire workflow.
type WorkflowReport = workflows.WorkflowReport

// RunWorkflow executes each step of w in-process. With jsonOut=true, output of each
// step is captured and embedded in StepResult.Output. Without jsonOut, "> <run>" is
// printed and the step's stdout flows to out.
// Stops at the first failing step unless step.ContinueOnError is true.
func RunWorkflow(
	ctx context.Context,
	deps Deps,
	in io.Reader,
	w workflows.Workflow,
	jsonOut, yes bool,
	out io.Writer,
	onStep func(i int, r StepResult),
) WorkflowReport {
	report := WorkflowReport{
		Name:  w.Name,
		Steps: make([]StepResult, 0, len(w.Steps)),
	}

	maxExit := 0
	for i, step := range w.Steps {
		stepArgs, err := workflows.Args(step)
		if err != nil {
			res := StepResult{
				Run:  step.Run,
				Exit: 2,
			}
			if !jsonOut {
				fmt.Fprintf(out, "✗ %s\n", err)
			}
			report.Steps = append(report.Steps, res)
			if res.Exit > maxExit {
				maxExit = res.Exit
			}
			if onStep != nil {
				onStep(i, res)
			}
			if !step.ContinueOnError {
				break
			}
			continue
		}

		var args []string
		args = append(args, stepArgs...)
		if jsonOut && !slices.Contains(args, "--json") {
			args = append(args, "--json")
		}
		if yes && !slices.Contains(args, "--yes") {
			args = append(args, "--yes")
		}

		var res StepResult
		if jsonOut {
			var stepBuf bytes.Buffer
			code := ExecuteWithContext(ctx, deps, args, in, &stepBuf, &stepBuf)
			res = StepResult{
				Run:  step.Run,
				Exit: code,
			}
			trimmed := bytes.TrimSpace(stepBuf.Bytes())
			if len(trimmed) > 0 {
				if json.Valid(trimmed) {
					res.Output = json.RawMessage(trimmed)
				} else {
					raw, _ := json.Marshal(string(trimmed))
					res.Output = json.RawMessage(raw)
				}
			}
		} else {
			fmt.Fprintf(out, "> %s\n", step.Run)
			code := ExecuteWithContext(ctx, deps, args, in, out, out)
			res = StepResult{
				Run:  step.Run,
				Exit: code,
			}
		}

		report.Steps = append(report.Steps, res)
		if res.Exit > maxExit {
			maxExit = res.Exit
		}
		if onStep != nil {
			onStep(i, res)
		}
		if res.Exit != 0 && !step.ContinueOnError {
			break
		}
	}

	report.Exit = maxExit
	return report
}

func newRunCmd(deps Deps, in io.Reader) *cobra.Command {
	var listFlag bool
	cmd := &cobra.Command{
		Use:   "run [name]",
		Short: "Run a composed workflow of Tija commands",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			o := newOutput(cmd, deps)
			dir, err := workDir()
			if err != nil {
				return err
			}
			root, inRepo, err := gitx.FindRoot(cmd.Context(), deps.Env.Run, dir)
			if err != nil || !inRepo {
				return &ExitError{Code: 2, Err: engine.ErrNotARepo}
			}

			wfs, loadErrs := workflows.Load(root)

			if listFlag {
				if wfs == nil {
					wfs = []workflows.Workflow{}
				}
				render.Workflows(o.printer, wfs)
				return nil
			}

			if len(args) == 0 {
				return &ExitError{Code: 2, Err: errors.New("specify a workflow name or use --list")}
			}

			name := args[0]
			var target *workflows.Workflow
			for _, w := range wfs {
				if w.Name == name {
					wCopy := w
					target = &wCopy
					break
				}
			}

			if target == nil {
				// Check if there was a load error for this name
				for _, lerr := range loadErrs {
					if lerr != nil {
						return &ExitError{Code: 2, Err: fmt.Errorf("workflow %q is invalid: %w", name, lerr)}
					}
				}
				return &ExitError{Code: 2, Err: fmt.Errorf("unknown workflow %q", name)}
			}

			valErrs := workflows.Validate(*target)
			if len(valErrs) > 0 {
				return &ExitError{Code: 2, Err: fmt.Errorf("workflow %q is invalid: %w", name, valErrs[0])}
			}

			rep := RunWorkflow(cmd.Context(), deps, in, *target, o.json, o.yes, cmd.OutOrStdout(), nil)
			if o.json {
				render.WorkflowReport(o.printer, rep)
			}

			if rep.Exit != 0 {
				return &ExitError{Code: rep.Exit}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&listFlag, "list", false, "List available workflows")
	return cmd
}
