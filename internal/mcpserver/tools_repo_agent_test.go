package mcpserver

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCommentsCleanHonorsExplicitAgent(t *testing.T) {
	root := testutil.NewRepo(t, "")
	env := sys.Default()
	env.LookPath = func(string) (string, error) { return "", errors.New("unavailable") }
	session := connectTools(t, New(root, Deps{Env: env}), nil)
	r, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "comments_clean", Arguments: map[string]any{"agent": "codex"}})
	if err != nil || !r.IsError {
		t.Fatalf("result=%#v err=%v", r, err)
	}
	text := ""
	for _, content := range r.Content {
		if c, ok := content.(*mcp.TextContent); ok {
			text += c.Text
		}
	}
	if !strings.Contains(text, "codex: agent CLI is not installed") {
		t.Fatal(text)
	}
}
