package mcpserver_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func commentsRepo(t *testing.T) string {
	t.Helper()
	return testutil.NewRepo(t, filepath.Join("..", "engine", "testdata", "comments-repo"))
}

func trackedStatus(t *testing.T, root string) string {
	t.Helper()
	return testutil.Git(t, root, "status", "--porcelain", "--untracked-files=no")
}

func structured(t *testing.T, r *mcp.CallToolResult) map[string]any {
	t.Helper()
	b, err := json.Marshal(r.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestRepoToolsRequireWrite(t *testing.T) {
	root := commentsRepo(t)
	comp := filepath.Join(root, "src", "Component.jsx")
	before, err := os.ReadFile(comp)
	if err != nil {
		t.Fatal(err)
	}
	status := trackedStatus(t, root)
	c := paritySession(t, root, sys.Default())

	fix := structured(t, callLocal(t, c, "quality_fix", map[string]any{}))
	if fix["status"] != "needs-write" {
		t.Errorf("quality_fix without write: %v", fix)
	}
	if _, ok := fix["changedFiles"]; ok {
		t.Errorf("quality_fix without write ran the engine: %v", fix)
	}

	clean := structured(t, callLocal(t, c, "comments_clean", map[string]any{"agent": "none", "all": true}))
	if clean["written"] == true {
		t.Errorf("comments_clean without write wrote: %v", clean)
	}
	diff := false
	for _, f := range clean["files"].([]any) {
		if d, _ := f.(map[string]any)["diff"].(string); d != "" {
			diff = true
		}
	}
	if !diff {
		t.Errorf("comments_clean without write returned no diff: %v", clean)
	}

	after, err := os.ReadFile(comp)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) || trackedStatus(t, root) != status {
		t.Errorf("tools without write changed tracked files")
	}
}

func TestCommentsCleanWriteApplies(t *testing.T) {
	root := commentsRepo(t)
	c := paritySession(t, root, sys.Default())
	clean := structured(t, callLocal(t, c, "comments_clean", map[string]any{"agent": "none", "all": true, "write": true}))
	if clean["written"] != true {
		t.Fatalf("comments_clean with write did not write: %v", clean)
	}
	if !strings.Contains(trackedStatus(t, root), "src/Component.jsx") {
		t.Errorf("Component.jsx not modified; status:\n%s", trackedStatus(t, root))
	}
}

func TestUnsafeWithoutWriteIsUsageError(t *testing.T) {
	root := commentsRepo(t)
	status := trackedStatus(t, root)
	c := paritySession(t, root, sys.Default())
	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "comments_clean", Arguments: map[string]any{"unsafe": true}})
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsError || !strings.Contains(r.Content[0].(*mcp.TextContent).Text, "unsafe requires write") {
		t.Errorf("unsafe without write = %#v", r)
	}
	if trackedStatus(t, root) != status {
		t.Errorf("a rejected call changed files")
	}
}

func TestQualityFixRejectsUnknownKind(t *testing.T) {
	c := paritySession(t, commentsRepo(t), sys.Default())
	r, err := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "quality_fix", Arguments: map[string]any{"kinds": []string{"test"}, "write": true}})
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsError {
		t.Errorf("quality_fix accepted kind test: %#v", r)
	}
}
