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
	if len(files) != 12 {
		t.Errorf("Files() has %d entries, want 12", len(files))
	}
	if _, ok := files[PrePRWorkflowPath]; !ok {
		t.Errorf("Files() missing %s", PrePRWorkflowPath)
	}
	for _, skill := range []string{"antislop-code", "antislop-copywriting", "thermo-review", "thermo-quality"} {
		skillMD := ".tija/skills/" + skill + "/SKILL.md"
		license := ".tija/skills/" + skill + "/LICENSE"
		if _, ok := files[skillMD]; !ok {
			t.Errorf("Files() missing %s", skillMD)
		}
		if _, ok := files[license]; !ok {
			t.Errorf("Files() missing %s", license)
		}
	}
}

func TestFilesReturnsCopies(t *testing.T) {
	Files()[RulesPath][0] = 'X'
	if Files()[RulesPath][0] == 'X' {
		t.Error("Files() must return fresh copies")
	}
}

func TestSkillLookup(t *testing.T) {
	data, ok := Skill("antislop-code")
	if !ok || len(data) == 0 {
		t.Fatalf("Skill(antislop-code) not found or empty")
	}
	if !bytes.Contains(data, []byte("name: antislop-code")) {
		t.Errorf("Skill(antislop-code) does not contain name")
	}

	// Suffix / prefix tolerance
	if _, ok := Skill(".tija/skills/thermo-review/SKILL.md"); !ok {
		t.Errorf("Skill(.tija/skills/thermo-review/SKILL.md) failed")
	}

	if _, ok := Skill("nonexistent-skill"); ok {
		t.Errorf("Skill(nonexistent-skill) want false, got true")
	}
}
