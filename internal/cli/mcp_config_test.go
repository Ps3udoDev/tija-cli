package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestMCPConfigCmd(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Execute([]string{"mcp", "config", "codex", "--dir", `C:\repo x`}, strings.NewReader(""), &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "tool_timeout_sec = 900") || !strings.Contains(out.String(), `C:\repo x`) {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	code = Execute([]string{"mcp", "config", "claude-desktop"}, strings.NewReader(""), &out, &errOut)
	var config struct {
		Servers map[string]struct{ Command string } `json:"mcpServers"`
	}
	exe, _ := os.Executable()
	if err := json.Unmarshal(out.Bytes(), &config); code != 0 || err != nil || config.Servers["tija"].Command != exe {
		t.Fatalf("code=%d out=%s err=%v", code, out.String(), err)
	}
	out.Reset()
	errOut.Reset()
	if code := Execute([]string{"mcp", "config", "unknown"}, strings.NewReader(""), &out, &errOut); code != 2 || out.Len() != 0 || !strings.Contains(errOut.String(), "claude-code") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
}
