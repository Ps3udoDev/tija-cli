package mcpserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

// ResolveRoot resolves the repository once, before accepting MCP requests.
func ResolveRoot(ctx context.Context, env sys.Env, dir string) (string, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	root, ok, err := gitx.FindRoot(ctx, env.Run, dir)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%s is not a git repository: %w", dir, engine.ErrNotARepo)
	}
	return root, nil
}
