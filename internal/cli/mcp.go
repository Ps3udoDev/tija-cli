package cli

import (
	"fmt"
	"log/slog"

	"github.com/Ps3udoDev/tija-cli/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

func newMCPCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "mcp", Short: "Expose Tija through the Model Context Protocol"}
	var dir, logLevel string
	serve := &cobra.Command{
		Use: "serve", Short: "Serve MCP over stdin/stdout for a fixed Git repository", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			levels := map[string]slog.Level{"error": slog.LevelError, "warn": slog.LevelWarn, "info": slog.LevelInfo, "debug": slog.LevelDebug}
			level, ok := levels[logLevel]
			if !ok {
				return fmt.Errorf("invalid log level %q: expected error, warn, info or debug", logLevel)
			}
			root, err := mcpserver.ResolveRoot(cmd.Context(), deps.Env, dir)
			if err != nil {
				return err
			}
			s := mcpserver.New(root, mcpserver.Deps{Env: deps.Env, Version: currentVersion(), Log: cmd.ErrOrStderr(), LogLevel: level})
			return s.Serve(cmd.Context(), &mcp.StdioTransport{})
		},
	}
	serve.Flags().StringVar(&dir, "dir", "", "Repository directory (defaults to the working directory)")
	serve.Flags().StringVar(&logLevel, "log-level", "warn", "Diagnostic level: error, warn, info or debug")
	cmd.AddCommand(serve)
	return cmd
}
