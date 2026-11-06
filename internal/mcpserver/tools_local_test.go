package mcpserver

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

func TestLocalToolsListedWithAnnotations(t *testing.T) {
	c := connectTools(t, New(t.TempDir(), Deps{Env: sys.Default()}), nil)
	r, err := c.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, tool := range r.Tools {
		if !slices.Contains([]string{"quality_check", "review_run", "pr_prepare"}, tool.Name) {
			continue
		}
		found = append(found, tool.Name)
		a := tool.Annotations
		if a == nil || a.ReadOnlyHint || a.DestructiveHint == nil || *a.DestructiveHint || a.OpenWorldHint == nil || *a.OpenWorldHint {
			t.Fatalf("wrong local annotations for %s: %#v", tool.Name, a)
		}
		if tool.OutputSchema == nil {
			t.Fatalf("missing output schema for %s", tool.Name)
		}
		b, _ := json.Marshal(tool.InputSchema)
		var schema map[string]any
		if err := json.Unmarshal(b, &schema); err != nil {
			t.Fatal(err)
		}
		if schema["additionalProperties"] != false {
			t.Fatalf("open input schema: %s", b)
		}
		if _, ok := schema["properties"].(map[string]any)["dir"]; ok {
			t.Fatalf("%s accepts dir", tool.Name)
		}
	}
	if len(found) != 3 {
		t.Fatalf("missing local tools: %v", found)
	}
}
