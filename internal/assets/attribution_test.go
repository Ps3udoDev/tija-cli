package assets

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Source      *struct {
		Project   string `yaml:"project"`
		Author    string `yaml:"author"`
		Repo      string `yaml:"repo"`
		Path      string `yaml:"path"`
		Commit    string `yaml:"commit"`
		Version   string `yaml:"version"`
		License   string `yaml:"license"`
		Copyright string `yaml:"copyright"`
	} `yaml:"source"`
	Adapted *struct {
		By      string   `yaml:"by"`
		Date    string   `yaml:"date"`
		Changes []string `yaml:"changes"`
	} `yaml:"adapted"`
}

var hex40Regex = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

func parseSkillMD(content []byte) (skillFrontmatter, string, error) {
	var fm skillFrontmatter
	text := string(content)
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return fm, text, nil
	}
	// Split by delimiter
	lines := strings.Split(text, "\n")
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if line == "---" {
			endIdx = i
			break
		}
	}
	if endIdx == -1 {
		return fm, text, nil
	}
	yamlPart := strings.Join(lines[1:endIdx], "\n")
	bodyPart := strings.Join(lines[endIdx+1:], "\n")
	if err := yaml.Unmarshal([]byte(yamlPart), &fm); err != nil {
		return fm, "", err
	}
	return fm, bodyPart, nil
}

func firstNonEmptyLine(body string) string {
	lines := strings.Split(body, "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func TestThirdPartyPacksAreAttributed(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("defaults", "skills", "*", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no skill packs found in defaults/skills/*/SKILL.md")
	}

	thirdPartyCount := 0
	for _, skillPath := range matches {
		data, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("failed reading %s: %v", skillPath, err)
		}
		fm, body, err := parseSkillMD(data)
		if err != nil {
			t.Fatalf("failed parsing frontmatter in %s: %v", skillPath, err)
		}
		if fm.Source == nil {
			continue
		}
		thirdPartyCount++
		packDir := filepath.Dir(skillPath)

		// 1. source.repo non-empty
		if strings.TrimSpace(fm.Source.Repo) == "" {
			t.Errorf("%s: source.repo is empty", skillPath)
		}
		// 2. source.commit 40 hex chars
		if !hex40Regex.MatchString(strings.TrimSpace(fm.Source.Commit)) {
			t.Errorf("%s: source.commit %q is not 40 hex chars", skillPath, fm.Source.Commit)
		}
		// 3. source.license == "MIT"
		if fm.Source.License != "MIT" {
			t.Errorf("%s: source.license = %q, want MIT", skillPath, fm.Source.License)
		}
		// 4. source.copyright non-empty
		if strings.TrimSpace(fm.Source.Copyright) == "" {
			t.Errorf("%s: source.copyright is empty", skillPath)
		}
		// 5. adapted.changes >= 1 item
		if fm.Adapted == nil || len(fm.Adapted.Changes) < 1 {
			t.Errorf("%s: adapted.changes must have at least 1 item", skillPath)
		}
		// 6. first non-empty line of body begins with `> Adaptado de`
		firstLine := firstNonEmptyLine(body)
		if !strings.HasPrefix(firstLine, "> Adaptado de") {
			t.Errorf("%s: first non-empty line of body must begin with '> Adaptado de', got %q", skillPath, firstLine)
		}
		// 7. LICENSE file exists next to SKILL.md containing source.copyright as substring
		licensePath := filepath.Join(packDir, "LICENSE")
		licenseData, err := os.ReadFile(licensePath)
		if err != nil {
			t.Errorf("%s: missing LICENSE file next to SKILL.md: %v", skillPath, err)
			continue
		}
		if !bytes.Contains(licenseData, []byte(fm.Source.Copyright)) {
			t.Errorf("%s: LICENSE does not contain copyright %q", skillPath, fm.Source.Copyright)
		}
	}

	if thirdPartyCount == 0 {
		t.Fatal("expected at least one third-party skill pack with source")
	}
}

func TestNoticesListEveryPack(t *testing.T) {
	noticesPath := filepath.Join("..", "..", "THIRD_PARTY_NOTICES.md")
	data, err := os.ReadFile(noticesPath)
	if err != nil {
		t.Fatalf("failed reading %s: %v", noticesPath, err)
	}
	notices := string(data)

	if !strings.Contains(notices, "MIT License") {
		t.Errorf("%s does not contain 'MIT License'", noticesPath)
	}

	matches, err := filepath.Glob(filepath.Join("defaults", "skills", "*", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no skill packs found in defaults/skills/*/SKILL.md")
	}

	for _, skillPath := range matches {
		skillData, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatal(err)
		}
		fm, _, err := parseSkillMD(skillData)
		if err != nil {
			t.Fatal(err)
		}
		if fm.Source != nil && fm.Source.Repo != "" {
			if !strings.Contains(notices, fm.Source.Repo) {
				t.Errorf("%s does not contain repo %q for %s", noticesPath, fm.Source.Repo, skillPath)
			}
		}
	}
}

func TestGoreleaserShipsNotices(t *testing.T) {
	goreleaserPath := filepath.Join("..", "..", ".goreleaser.yaml")
	data, err := os.ReadFile(goreleaserPath)
	if err != nil {
		t.Fatalf("failed reading %s: %v", goreleaserPath, err)
	}
	if !strings.Contains(string(data), "THIRD_PARTY_NOTICES.md") {
		t.Errorf("%s does not include THIRD_PARTY_NOTICES.md in archives.files", goreleaserPath)
	}
}
