package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

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

// RunWorkflow executes each step of w in-process through the shared step
// loop (workflows.Run). With jsonOut=true, the output of each step is
// captured and embedded in StepResult.Output; without it, "> <run>" is
// printed and the step's output flows to out.
func RunWorkflow(
	ctx context.Context,
	deps Deps,
	in io.Reader,
	w workflows.Workflow,
	jsonOut, yes bool,
	out io.Writer,
	onStep func(i int, r StepResult),
) WorkflowReport {
	return workflows.Run(ctx, w, StepExecutor(deps, in), workflows.Options{JSON: jsonOut, Yes: yes, Out: out, OnStep: onStep})
}

// StepExecutor runs a workflow step as a CLI command with deps, reading
// consent answers from in.
func StepExecutor(deps Deps, in io.Reader) workflows.Executor {
	return func(ctx context.Context, args []string, out, errOut io.Writer) int {
		return ExecuteWithContext(ctx, deps, args, in, out, errOut)
	}
}

func newRunCmd(deps Deps, in io.Reader) *cobra.Command {
	var listFlag bool
	cmd := &cobra.Command{
		Use:   "run [name]",
		Short: "Run a composed workflow of Tija commands",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			o := newOutput(cmd, deps)
			dir, err := deps.workDir()
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
