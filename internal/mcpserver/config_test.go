package mcpserver

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestClientConfigFormats(t *testing.T) {
	for _, dir := range []string{`C:\repo x`, `C:\O'Brien\repo x`, "/tmp/repo\u007f\u0001"} {
		exe := `C:\Program Files\Tija\tija.exe`
		text, err := ClientConfig("codex", exe, dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, expected := range []string{"[mcp_servers.tija]", "tool_timeout_sec = 900", "startup_timeout_sec = 20"} {
			if !strings.Contains(text, expected) {
				t.Fatalf("missing %s: %s", expected, text)
			}
		}
		if strings.ContainsAny(text, "\x7f\x01") {
			t.Fatal("raw control characters in TOML")
		}
		var command string
		var args []string
		for _, line := range strings.Split(text, "\n") {
			if value, ok := strings.CutPrefix(line, "command = "); ok {
				command = decodeConfigString(t, value)
			}
			if value, ok := strings.CutPrefix(line, "args = "); ok {
				for _, token := range regexp.MustCompile(`'[^']*'|"(?:\\.|[^"\\])*"`).FindAllString(value, -1) {
					args = append(args, decodeConfigString(t, token))
				}
			}
		}
		if command != exe || len(args) != 4 || args[0] != "mcp" || args[1] != "serve" || args[2] != "--dir" || args[3] != dir {
			t.Fatalf("roundtrip: command=%q args=%q", command, args)
		}
		for _, client := range []string{"claude-desktop", "cursor", "antigravity"} {
			text, err := ClientConfig(client, exe, dir)
			if err != nil {
				t.Fatal(err)
			}
			if client == "antigravity" {
				if !strings.HasPrefix(text, "# experimental") {
					t.Fatal(text)
				}
				_, text, _ = strings.Cut(text, "\n")
			}
			var config struct {
				Servers map[string]struct {
					Command string
					Args    []string
				} `json:"mcpServers"`
			}
			if err := json.Unmarshal([]byte(text), &config); err != nil {
				t.Fatal(err)
			}
			if got := config.Servers["tija"]; got.Command != exe || len(got.Args) != 4 || got.Args[3] != dir {
				t.Fatalf("%s lost paths: %#v", client, got)
			}
		}
		commandText, err := ClientConfig("claude-code", exe, dir)
		if err != nil || !strings.HasPrefix(commandText, "claude mcp add tija -- ") || !strings.Contains(commandText, "--dir") {
			t.Fatalf("command=%q err=%v", commandText, err)
		}
	}
	text, _ := ClientConfig("claude-desktop", "/bin/tija", "")
	if strings.Contains(text, "--dir") {
		t.Fatal("unexpected directory")
	}
}

func decodeConfigString(t *testing.T, value string) string {
	t.Helper()
	if strings.HasPrefix(value, "'") {
		return strings.TrimSuffix(strings.TrimPrefix(value, "'"), "'")
	}
	s, err := strconv.Unquote(value)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestClientConfigUnknown(t *testing.T) {
	_, err := ClientConfig("vscode", "/bin/tija", "")
	if err == nil {
		t.Fatal("accepted unknown client")
	}
	for _, client := range Clients {
		if !strings.Contains(err.Error(), client) {
			t.Fatal(err)
		}
	}
}
