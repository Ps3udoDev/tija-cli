package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/render"
)

func newCleanCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean slop and unwanted patterns from the codebase",
	}
	cmd.AddCommand(newCleanCommentsCmd(deps))
	return cmd
}

func newCleanCommentsCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comments [paths...]",
		Short: "Clean comment slop from JS/TS code with verified edits",
		RunE: func(cmd *cobra.Command, args []string) error {
			o := newOutput(cmd, deps)
			dir, err := workDir()
			if err != nil {
				return err
			}

			concurrency, err := cmd.Flags().GetInt("concurrency")
			if err != nil {
				return &ExitError{Code: 2, Err: err}
			}
			if concurrency <= 0 {
				return &ExitError{Code: 2, Err: fmt.Errorf("--concurrency must be > 0")}
			}

			agentTimeout, err := cmd.Flags().GetDuration("agent-timeout")
			if err != nil {
				return &ExitError{Code: 2, Err: err}
			}

			write, _ := cmd.Flags().GetBool("write")
			unsafe, _ := cmd.Flags().GetBool("unsafe")
			diff, _ := cmd.Flags().GetBool("diff")
			all, _ := cmd.Flags().GetBool("all")
			noAgent, _ := cmd.Flags().GetBool("no-agent")
			force, _ := cmd.Flags().GetBool("force")

			req := engine.CleanRequest{
				Paths:        args,
				All:          all,
				Write:        write,
				Unsafe:       unsafe,
				Diff:         diff,
				NoAgent:      noAgent,
				Force:        force,
				Concurrency:  concurrency,
				AgentTimeout: agentTimeout,
			}

			var onEvent func(engine.CleanEvent)
			if !o.json {
				onEvent = func(ev engine.CleanEvent) {
					render.CleanProgress(o.printer, ev)
				}
			}

			report, err := engine.CleanComments(cmd.Context(), dir, deps.Env, req, onEvent)
			if errors.Is(err, engine.ErrNotARepo) {
				return &ExitError{Code: 2, Err: err}
			}
			if err != nil {
				return err
			}

			render.CleanComments(o.printer, report)

			if code := report.ExitCode(); code != 0 {
				return &ExitError{Code: code}
			}
			return nil
		},
	}

	cmd.Flags().Bool("write", false, "Apply safe changes")
	cmd.Flags().Bool("unsafe", false, "Include or apply unsafe changes")
	cmd.Flags().Bool("diff", false, "Show unified diff of changes")
	cmd.Flags().Bool("all", false, "Include all files matching manifest include/exclude")
	cmd.Flags().Bool("no-agent", false, "Only run deterministic rules")
	cmd.Flags().Bool("force", false, "Include untracked files")
	cmd.Flags().Int("concurrency", 2, "Number of concurrent agent batches")
	cmd.Flags().Duration("agent-timeout", 3*time.Minute, "Timeout for agent batch calls")

	return cmd
}
