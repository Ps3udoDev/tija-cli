package sys

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// EditorCommand returns the editor git would use (git var GIT_EDITOR),
// ready to edit path from dir. The command is not started and has no
// stdio set: callers attach it to the terminal.
func EditorCommand(ctx context.Context, env Env, dir, path string) (*exec.Cmd, error) {
	stdout, stderr, code, err := env.Run(ctx, dir, "git", "var", "GIT_EDITOR")
	if err != nil {
		return nil, fmt.Errorf("git var GIT_EDITOR: %w", err)
	}
	if code != 0 {
		return nil, fmt.Errorf("git var GIT_EDITOR failed: %s", strings.TrimSpace(stderr))
	}
	argv := splitCommand(strings.TrimSpace(stdout))
	if len(argv) == 0 {
		return nil, errors.New("no editor configured (set GIT_EDITOR or core.editor)")
	}
	c := exec.CommandContext(ctx, argv[0], append(argv[1:], path)...)
	c.Dir = dir
	return c, nil
}

// splitCommand splits a command line on blanks, honoring single and double quotes.
func splitCommand(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	started := false
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, started = r, true
		case r == ' ' || r == '\t':
			if started {
				out = append(out, cur.String())
				cur.Reset()
				started = false
			}
		default:
			cur.WriteRune(r)
			started = true
		}
	}
	if started {
		out = append(out, cur.String())
	}
	return out
}
