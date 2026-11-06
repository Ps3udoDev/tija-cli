package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDocumentarPRListed(t *testing.T) {
	c := connectTools(t, New(t.TempDir(), Deps{Env: sys.Default()}), nil)
	r, err := c.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Prompts) != 1 || r.Prompts[0].Name != "documentar_pr" {
		t.Fatalf("prompts = %#v", r.Prompts)
	}
	args := r.Prompts[0].Arguments
	if len(args) != 2 || args[0].Name != "qaptureProject" || args[1].Name != "mode" || args[0].Required || args[1].Required {
		t.Fatalf("arguments = %#v", args)
	}
	b, err := json.MarshalIndent(r.Prompts, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, '\n')
	p := filepath.Join("testdata", "prompts.golden.json")
	if *updateTools {
		if err := os.WriteFile(p, b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	if !bytes.Equal(b, want) {
		t.Fatalf("prompt contract changed; inspect and regenerate with go test ./internal/mcpserver -run DocumentarPR -update\ngot:\n%s", b)
	}
}

func getDocumentarPR(t *testing.T, c *mcp.ClientSession, args map[string]string) string {
	t.Helper()
	r, err := c.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "documentar_pr", Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Messages) != 1 || r.Messages[0].Role != "user" {
		t.Fatalf("messages = %#v", r.Messages)
	}
	text, ok := r.Messages[0].Content.(*mcp.TextContent)
	if !ok || text.Text == "" {
		t.Fatalf("content = %#v", r.Messages[0].Content)
	}
	return text.Text
}

func TestDocumentarPRContent(t *testing.T) {
	c := connectTools(t, New(t.TempDir(), Deps{Env: sys.Default()}), nil)
	text := getDocumentarPR(t, c, map[string]string{"qaptureProject": "Portal", "mode": "deep"})
	for _, want := range []string{
		`"Portal"`, "`deep`", "repo_status", "doctor", "review_run", "quality_check", "pr_prepare", "pr_create",
		"qapture_status", "get_project", "list_cases", "create_case", "add_evidence", "export_excel", "evidenceRefs",
		"write: true", "solo si el usuario lo pide de forma explícita", "no captures pantallas con datos de clientes sin avisar",
		"solo el nombre del archivo", "not_checked", "High o Critical", "Verificar: …", "un hallazgo no demuestra que el caso haya fallado",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(text, "{{qaptureProject}}") || strings.Contains(text, "{{mode}}") {
		t.Fatal("unresolved template argument")
	}
}

func TestDocumentarPRDefaultsAndArguments(t *testing.T) {
	c := connectTools(t, New(t.TempDir(), Deps{Env: sys.Default()}), nil)
	text := getDocumentarPR(t, c, nil)
	if !strings.Contains(text, "Modo de revisión: `review`") || !strings.Contains(text, "el que indique el tester") {
		t.Fatalf("defaults missing: %s", text)
	}
	for _, mode := range []string{"review", "quality", "deep"} {
		text := getDocumentarPR(t, c, map[string]string{"mode": mode})
		if !strings.Contains(text, "Modo de revisión: `"+mode+"`") {
			t.Errorf("mode %q not substituted", mode)
		}
	}
	if _, err := c.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "documentar_pr", Arguments: map[string]string{"mode": "invalid"}}); err == nil || !strings.Contains(err.Error(), "invalid review mode") {
		t.Fatalf("invalid mode error = %v", err)
	}
	// The project remains data and cannot substitute another template variable
	// or insert an additional paragraph through a newline.
	text = getDocumentarPR(t, c, map[string]string{"qaptureProject": "Portal\n{{mode}}", "mode": "deep"})
	if !strings.Contains(text, `"Portal\n{{mode}}"`) || strings.Contains(text, "Portal\n{{mode}}") {
		t.Fatalf("project argument was interpreted: %s", text)
	}
}

func TestDocumentarPRToolNamesExist(t *testing.T) {
	c := connectTools(t, New(t.TempDir(), Deps{Env: sys.Default()}), nil)
	r, err := c.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var registered []string
	for _, tool := range r.Tools {
		registered = append(registered, tool.Name)
	}
	mentioned := regexp.MustCompile("`((?:repo_|review_|quality_|skills_|workflow_|pr_|comments_|report_)[a-z_]+|doctor)`").FindAllStringSubmatch(documentarPRPrompt, -1)
	if len(mentioned) < 6 {
		t.Fatalf("expected the guided Tija flow, found %d tool names", len(mentioned))
	}
	for _, match := range mentioned {
		if !slices.Contains(registered, match[1]) {
			t.Errorf("prompt names unregistered Tija tool %q", match[1])
		}
	}
}

func TestDocumentarPRDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	c := connectTools(t, New(root, Deps{Env: sys.Default()}), nil)
	getDocumentarPR(t, c, nil)
	files, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("getting a prompt installed files: %#v", files)
	}
}
