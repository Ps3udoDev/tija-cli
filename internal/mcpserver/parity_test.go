package mcpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/cli"
	"github.com/Ps3udoDev/tija-cli/internal/mcpserver"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func paritySession(t *testing.T, root string, env sys.Env) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	s := mcpserver.New(root, mcpserver.Deps{Env: env})
	a, b := mcp.NewInMemoryTransports()
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, a) }()
	c, err := mcp.NewClient(&mcp.Implementation{Name: "parity", Version: "1"}, nil).Connect(ctx, b, nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = c.Close()
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("server did not stop")
		}
	})
	return c
}

func TestRepoStatusParity(t *testing.T) {
	root := testutil.NewRepo(t, "", testutil.WithBranch("feat/mcp-parity"))
	t.Chdir(root)
	env := sys.Default()
	c := paritySession(t, root, env)
	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "repo_status", Arguments: map[string]any{}})
	if err != nil || r.IsError {
		t.Fatalf("MCP status: %v %#v", err, r)
	}
	var stdout, stderr bytes.Buffer
	if code := cli.ExecuteWith(cli.Deps{Env: env}, []string{"status", "--json"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("CLI status exit %d: %s", code, stderr.String())
	}
	assertJSONParity(t, r.StructuredContent, stdout.Bytes())
	if r.Meta["tija/exitCode"] != float64(0) {
		t.Fatalf("exit metadata: %#v", r.Meta)
	}
}

func TestSkillsListParity(t *testing.T) {
	root := testutil.NewRepo(t, "")
	t.Chdir(root)
	env := sys.Default()
	c := paritySession(t, root, env)
	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "skills_list", Arguments: map[string]any{}})
	if err != nil || r.IsError {
		t.Fatalf("MCP skills: %v %#v", err, r)
	}
	var stdout, stderr bytes.Buffer
	if code := cli.ExecuteWith(cli.Deps{Env: env}, []string{"skills", "list", "--json"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("CLI skills exit %d: %s", code, stderr.String())
	}
	b, err := json.Marshal(r.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var wrapper map[string]json.RawMessage
	if err = json.Unmarshal(b, &wrapper); err != nil {
		t.Fatal(err)
	}
	assertJSONParity(t, json.RawMessage(wrapper["skills"]), stdout.Bytes())
}

func assertJSONParity(t *testing.T, structured any, cliJSON []byte) {
	t.Helper()
	b, err := json.Marshal(structured)
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err = json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(cliJSON, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MCP JSON differs from CLI\nMCP: %s\nCLI: %s", b, cliJSON)
	}
}
