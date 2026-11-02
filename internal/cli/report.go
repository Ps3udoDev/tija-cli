package cli

import (
	"errors"
	"time"

	"github.com/spf13/cobra"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/render"
)

func newReportCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate PDF reports",
	}
	cmd.AddCommand(newReportActivityCmd(deps))
	return cmd
}

func newReportActivityCmd(deps Deps) *cobra.Command {
	var req engine.ReportRequest
	var noAgent bool
	cmd := &cobra.Command{
		Use:   "activity --since YYYY-MM-DD --until YYYY-MM-DD",
		Short: "Generate the activity report of a period from commits and pull requests",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			o := newOutput(cmd, deps)
			dir, err := workDir()
			if err != nil {
				return err
			}
			if noAgent {
				req.Agent = "none"
			}
			res, err := engine.ActivityReport(cmd.Context(), dir, deps.Env, req, nil)
			if errors.Is(err, engine.ErrReportSetup) || errors.Is(err, engine.ErrNotARepo) {
				return &ExitError{Code: 2, Err: err}
			}
			if err != nil {
				return &ExitError{Code: 1, Err: err}
			}
			render.ActivityReport(o.printer, res)
			return nil
		},
	}
	cmd.Flags().StringVar(&req.Since, "since", "", "First day of the period (YYYY-MM-DD)")
	cmd.Flags().StringVar(&req.Until, "until", "", "Last day of the period (YYYY-MM-DD)")
	cmd.Flags().StringVar(&req.Agent, "agent", "", "Agent that writes the report (default: agents.default of the manifest)")
	cmd.Flags().BoolVar(&noAgent, "no-agent", false, "Stop after collecting the activity")
	cmd.Flags().DurationVar(&req.AgentTimeout, "agent-timeout", 5*time.Minute, "Timeout of each agent attempt")
	_ = cmd.MarkFlagRequired("since")
	_ = cmd.MarkFlagRequired("until")
	return cmd
}
