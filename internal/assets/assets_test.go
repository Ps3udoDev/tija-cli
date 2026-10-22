package assets

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsMatchReference(t *testing.T) {
	ref := filepath.Join("..", "..", "docs", "reference", "pull-requests")
	files := Files()
	for path, refName := range map[string]string{
		RulesPath:    "PULL_REQUEST_TEMPLATE_GUIDE.md",
		TemplatePath: "PULL_REQUEST_TEMPLATE.md",
	} {
		want, err := os.ReadFile(filepath.Join(ref, refName))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(files[path], want) {
			t.Errorf("%s differs from %s", path, refName)
		}
	}
	if b, ok := files[WorkflowsKeep]; !ok || len(b) != 0 {
		t.Errorf("%s = %q, %v", WorkflowsKeep, b, ok)
	}
	if len(files) != 3 {
		t.Errorf("Files() has %d entries", len(files))
	}
}

func TestFilesReturnsCopies(t *testing.T) {
	Files()[RulesPath][0] = 'X'
	if Files()[RulesPath][0] == 'X' {
		t.Error("Files() must return fresh copies")
	}
}
