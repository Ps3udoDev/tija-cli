package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/render"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/spf13/cobra"
)

var prAgents = []string{"claude", "codex", "gemini", "none"}

func addPRFlags(cmd *cobra.Command) {
	cmd.Flags().String("agent", "", "Agent that drafts the pull request: claude, codex, gemini or none (default: manifest)")
	cmd.Flags().Bool("no-agent", false, "Do not use an agent (same as --agent none)")
	cmd.Flags().Bool("no-check", false, "Do not run check when there are no verified results")
	cmd.Flags().Duration("agent-timeout", engine.DefaultAgentTimeout, "Timeout of each agent attempt")
	cmd.Flags().Duration("check-timeout", engine.DefaultCheckTimeout, "Timeout of each check step")
}

func prRequest(cmd *cobra.Command) (engine.PRRequest, error) {
	agent, _ := cmd.Flags().GetString("agent")
	if noAgent, _ := cmd.Flags().GetBool("no-agent"); noAgent {
		agent = "none"
	}
	if agent != "" && !slices.Contains(prAgents, agent) {
		return engine.PRRequest{}, &ExitError{Code: 2, Err: fmt.Errorf("unknown agent %q (use claude, codex, gemini or none)", agent)}
	}
	noCheck, _ := cmd.Flags().GetBool("no-check")
	at, _ := cmd.Flags().GetDuration("agent-timeout")
	ct, _ := cmd.Flags().GetDuration("check-timeout")
	return engine.PRRequest{Agent: agent, NoCheck: noCheck, AgentTimeout: at, CheckTimeout: ct}, nil
}

// prExit maps engine errors to exit codes: no commits is 1, everything
// else (usage, environment, git or gh failures) is 2.
func prExit(err error) error {
	if errors.Is(err, engine.ErrNoCommits) {
		return &ExitError{Code: 1, Err: err}
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return err
	}
	return &ExitError{Code: 2, Err: err}
}

// progress prints stage messages to stderr unless the output is JSON.
func progress(cmd *cobra.Command, o output) func(engine.PREvent) {
	if o.json {
		return nil
	}
	w := cmd.ErrOrStderr()
	return func(e engine.PREvent) {
		if e.Stage != engine.StageDone {
			fmt.Fprintf(w, "> %s\n", e.Message)
		}
	}
}

func newPRCmd(deps Deps, in io.Reader) *cobra.Command {
	pr := &cobra.Command{
		Use:   "pr",
		Short: "Prepare and create pull requests",
	}
	pr.AddCommand(newPRPrepareCmd(deps), newPRCreateCmd(deps, in))
	return pr
}

func newPRPrepareCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Draft the pull request title and body for the current branch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			o := newOutput(cmd, deps)
			dir, err := workDir()
			if err != nil {
				return err
			}
			if printPrompt, _ := cmd.Flags().GetBool("print-prompt"); printPrompt {
				prompt, err := engine.PRPrompt(cmd.Context(), dir, deps.Env)
				if err != nil {
					return prExit(err)
				}
				_, err = io.WriteString(cmd.OutOrStdout(), prompt)
				return err
			}
			req, err := prRequest(cmd)
			if err != nil {
				return err
			}
			rep, err := engine.PreparePR(cmd.Context(), dir, deps.Env, req, progress(cmd, o))
			if err != nil {
				return prExit(err)
			}
			render.PRPrepare(o.printer, rep)
			if code := rep.ExitCode(); code != 0 {
				return &ExitError{Code: code}
			}
			return nil
		},
	}
	addPRFlags(cmd)
	cmd.Flags().Bool("print-prompt", false, "Print the agent prompt and exit")
	return cmd
}

func newPRCreateCmd(deps Deps, in io.Reader) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Push the branch and open (or update) the pull request",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPRCreate(cmd, deps, in)
		},
	}
	addPRFlags(cmd)
	cmd.Flags().Bool("draft", false, "Open the pull request as a draft")
	cmd.Flags().Bool("edit", false, "Edit the draft in your editor before confirming")
	return cmd
}

func runPRCreate(cmd *cobra.Command, deps Deps, in io.Reader) error {
	o := newOutput(cmd, deps)
	dir, err := workDir()
	if err != nil {
		return err
	}
	prep, err := prRequest(cmd)
	if err != nil {
		return err
	}
	draft, _ := cmd.Flags().GetBool("draft")
	edit, _ := cmd.Flags().GetBool("edit")
	req := engine.CreateRequest{Prepare: prep, Draft: draft}
	interactive := !o.json && isTTY(deps, in) && isTTY(deps, cmd.OutOrStdout())
	if edit && !interactive {
		return &ExitError{Code: 2, Err: errors.New("--edit needs an interactive terminal")}
	}

	ctx := cmd.Context()
	plan, err := engine.PlanCreatePR(ctx, dir, deps.Env, req, progress(cmd, o))
	if err != nil {
		return prExit(err)
	}
	if edit {
		if plan.Report.DraftPath == "" {
			return &ExitError{Code: 2, Err: errors.New("--edit needs an initialized repository (run: tija init)")}
		}
		root := plan.Report.Root
		if err := editDraft(ctx, deps, root, filepath.Join(root, plan.Report.DraftPath)); err != nil {
			return prExit(err)
		}
		if plan, err = engine.PlanCreatePR(ctx, dir, deps.Env, req, progress(cmd, o)); err != nil {
			return prExit(err)
		}
	}

	if len(plan.Report.Problems) > 0 {
		render.PRPrepare(o.printer, plan.Report)
		return &ExitError{Code: 1}
	}

	if !o.yes {
		if !interactive {
			if !o.json {
				render.PRPrepare(o.printer, plan.Report)
			}
			return &ExitError{Code: 2, Err: errors.New("confirmation required: rerun with --yes")}
		}
		render.PRPrepare(o.printer, plan.Report)
		o.printer.Line("")
		question := fmt.Sprintf("Push %s and open a pull request to %s?", plan.Report.Branch, plan.Report.Target)
		if plan.Existing != "" {
			question = fmt.Sprintf("Update pull request %s?", plan.Existing)
		}
		if !newStdinPrompter(in, o.printer).Yes(question) {
			o.printer.Line("Nothing done.")
			return nil
		}
	}

	res, err := engine.ApplyCreatePR(ctx, dir, deps.Env, plan, req)
	if err != nil {
		return prExit(err)
	}
	render.PRCreate(o.printer, plan, res)
	return nil
}

// editDraft opens path in the editor git would use, attached to the terminal.
func editDraft(ctx context.Context, deps Deps, root, path string) error {
	c, err := sys.EditorCommand(ctx, deps.Env, root, path)
	if err != nil {
		return err
	}
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("editor %s: %w", c.Args[0], err)
	}
	return nil
}
