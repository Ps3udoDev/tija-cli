package cli

import (
	"fmt"
	"slices"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/render"
	"github.com/spf13/cobra"
)

func newReviewCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "review", Short: "Review the working diff with verified findings", Args: cobra.NoArgs}
	cmd.Flags().Bool("quality", false, "Review code quality")
	cmd.Flags().Bool("deep", false, "Run review and quality passes in parallel")
	cmd.Flags().String("fail-on", "", "Finding threshold: critical, high, medium, low or none")
	cmd.Flags().String("agent", "", "Agent: claude, codex, gemini or none")
	cmd.Flags().Bool("no-agent", false, "Run only deterministic checks")
	cmd.Flags().Duration("agent-timeout", 0, "Override the tier timeout for each agent call")
	cmd.Flags().Duration("check-timeout", engine.DefaultCheckTimeout, "Timeout of each check step")
	cmd.Flags().Bool("no-check", false, "Skip quality checks")
	cmd.Flags().Bool("no-pr-comments", false, "Skip reading pull request comments")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		q, _ := cmd.Flags().GetBool("quality")
		d, _ := cmd.Flags().GetBool("deep")
		fail, _ := cmd.Flags().GetString("fail-on")
		name, _ := cmd.Flags().GetString("agent")
		at, _ := cmd.Flags().GetDuration("agent-timeout")
		ct, _ := cmd.Flags().GetDuration("check-timeout")
		bad := func(msg string) error { return &ExitError{Code: 2, Err: fmt.Errorf("%s", msg)} }
		if q && d {
			return bad("--quality and --deep are mutually exclusive")
		}
		if cmd.Flags().Changed("fail-on") && !slices.Contains(manifest.FailOnValues, fail) {
			return bad("--fail-on must be critical, high, medium, low or none")
		}
		if name != "" && !slices.Contains(prAgents, name) {
			return bad("unknown agent (use claude, codex, gemini or none)")
		}
		if (cmd.Flags().Changed("agent-timeout") && at <= 0) || ct <= 0 {
			return bad("timeouts must be positive")
		}
		req := engine.ReviewRequest{Mode: "review", FailOn: fail, Agent: name, AgentTimeout: at, CheckTimeout: ct}
		if q {
			req.Mode = "quality"
		}
		if d {
			req.Mode = "deep"
		}
		req.NoAgent, _ = cmd.Flags().GetBool("no-agent")
		req.NoCheck, _ = cmd.Flags().GetBool("no-check")
		req.NoPRComments, _ = cmd.Flags().GetBool("no-pr-comments")
		dir, err := workDir()
		if err != nil {
			return prExit(err)
		}
		o := newOutput(cmd, deps)
		var progress func(engine.ReviewEvent)
		if !o.json {
			progress = func(ev engine.ReviewEvent) {
				render.ReviewProgress(render.Printer{W: cmd.ErrOrStderr(), Mode: o.printer.Mode}, ev)
			}
		}
		r, err := engine.RunReview(cmd.Context(), dir, deps.Env, req, progress)
		if err != nil {
			return prExit(err)
		}
		render.Review(o.printer, r)
		if r.ExitCode() != 0 {
			return &ExitError{Code: r.ExitCode()}
		}
		return nil
	}
	return cmd
}
