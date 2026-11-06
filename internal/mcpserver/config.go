package mcpserver

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

var Clients = []string{"claude-code", "codex", "claude-desktop", "cursor", "antigravity"}

// ClientConfig prints configuration only; it never edits the client's files.
func ClientConfig(client, exe, dir string) (string, error) {
	args := []string{"mcp", "serve"}
	if dir != "" {
		args = append(args, "--dir", dir)
	}
	switch client {
	case "claude-code":
		parts := []string{"claude mcp add tija --", commandQuote(exe)}
		for _, arg := range args {
			parts = append(parts, commandQuote(arg))
		}
		return strings.Join(parts, " ") + "\n", nil
	case "codex":
		quoted := make([]string, len(args))
		for i, arg := range args {
			quoted[i] = tomlQuote(arg)
		}
		return fmt.Sprintf("[mcp_servers.tija]\ncommand = %s\nargs = [%s]\ntool_timeout_sec = 900\nstartup_timeout_sec = 20\n", tomlQuote(exe), strings.Join(quoted, ", ")), nil
	case "claude-desktop", "cursor", "antigravity":
		config := map[string]any{"mcpServers": map[string]any{"tija": map[string]any{"command": exe, "args": args}}}
		b, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return "", err
		}
		prefix := ""
		if client == "antigravity" {
			prefix = "# experimental: Antigravity MCP stdio support is not validated yet\n"
		}
		return prefix + string(b) + "\n", nil
	default:
		return "", fmt.Errorf("unknown MCP client %q (use %s)", client, strings.Join(Clients, ", "))
	}
}

func tomlQuote(s string) string {
	literal := true
	for _, r := range s {
		if r == '\'' || r < 0x20 || r == 0x7f {
			literal = false
			break
		}
	}
	if literal {
		return "'" + s + "'"
	}
	b, _ := json.Marshal(s)
	return strings.ReplaceAll(string(b), "\x7f", "\\u007f")
}

func commandQuote(s string) string {
	if runtime.GOOS == "windows" {
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
