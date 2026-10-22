package cli

import (
	"errors"
	"io"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/render"
	"github.com/spf13/cobra"
)

func addQualityFlags(cmd *cobra.Command) {
	cmd.Flags().Duration("timeout", 10*time.Minute, "Timeout for quality checks")
	cmd.Flags().Bool("verbose", false, "Stream process output to stderr")
}

func runQuality(cmd *cobra.Command, deps Deps, kinds []quality.Kind, mode quality.Mode) error {
	o := newOutput(cmd, deps)
	dir, err := workDir()
	if err != nil {
		return err
	}
	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}
	verbose, _ := cmd.Flags().GetBool("verbose")
	var reqVerbose io.Writer
	if !o.json && verbose {
		reqVerbose = cmd.ErrOrStderr()
	}
	req := engine.QualityRequest{
		Kinds:   kinds,
		Mode:    mode,
		Timeout: timeout,
		Verbose: reqVerbose,
	}
	rep, err := engine.RunQuality(cmd.Context(), dir, deps.Env, req, nil)
	if errors.Is(err, engine.ErrNotARepo) {
		return &ExitError{Code: 2, Err: err}
	}
	if err != nil {
		return err
	}
	render.Quality(o.printer, rep)
	if code := rep.ExitCode(); code != 0 {
		return &ExitError{Code: code}
	}
	return nil
}

func newCheckCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run all quality checks (lint, format, tests)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runQuality(cmd, deps, nil, quality.CheckMode)
		},
	}
	addQualityFlags(cmd)
	return cmd
}

func newLintCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Run linter checks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fix, _ := cmd.Flags().GetBool("fix")
			if fix {
				return runQuality(cmd, deps, []quality.Kind{quality.Lint, quality.Format}, quality.FixMode)
			}
			return runQuality(cmd, deps, []quality.Kind{quality.Lint}, quality.CheckMode)
		},
	}
	cmd.Flags().Bool("fix", false, "Automatically fix lint and format issues")
	addQualityFlags(cmd)
	return cmd
}

func newFormatCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "format",
		Short: "Format source code",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runQuality(cmd, deps, []quality.Kind{quality.Format}, quality.FixMode)
		},
	}
	addQualityFlags(cmd)
	return cmd
}

func newTestCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run tests",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runQuality(cmd, deps, []quality.Kind{quality.Tests}, quality.CheckMode)
		},
	}
	addQualityFlags(cmd)
	return cmd
}
