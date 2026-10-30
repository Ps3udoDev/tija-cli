// Package skills loads, validates, and manages skill packs.
package skills

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Ps3udoDev/tija-cli/internal/assets"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
)

// Source describes the origin repository and author of a third-party pack.
type Source struct {
	Project   string `yaml:"project" json:"project"`
	Author    string `yaml:"author" json:"author"`
	Repo      string `yaml:"repo" json:"repo"`
	Path      string `yaml:"path" json:"path"`
	Commit    string `yaml:"commit" json:"commit"`
	Version   string `yaml:"version,omitempty" json:"version,omitempty"`
	License   string `yaml:"license" json:"license"`
	Copyright string `yaml:"copyright" json:"copyright"`
}

// Adapted describes who modified the pack and what was changed.
type Adapted struct {
	By      string   `yaml:"by" json:"by,omitempty"`
	Date    string   `yaml:"date" json:"date,omitempty"`
	Changes []string `yaml:"changes" json:"changes,omitempty"`
}

// Pack represents a single skill pack.
type Pack struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Description string   `json:"description"`
	Origin      string   `json:"origin"` // "builtin" | "third-party" | "repo"
	Source      *Source  `json:"source,omitempty"`
	Adapted     *Adapted `json:"adapted,omitempty"`
	Edited      bool     `json:"edited"`
	Problems    []string `json:"problems,omitempty"`
	Body        string   `json:"body,omitempty"`
}

type skillFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Source      *Source  `yaml:"source"`
	Adapted     *Adapted `yaml:"adapted"`
}

// Parse parses a SKILL.md document, separating frontmatter from markdown body.
func Parse(data []byte) (Pack, error) {
	text := string(data)
	text = strings.TrimPrefix(text, "\ufeff")
	var pack Pack
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		pack.Body = text
		return pack, nil
	}

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
		pack.Body = text
		return pack, nil
	}

	fmContent := strings.Join(lines[1:endIdx], "\n")
	var fm skillFrontmatter
	if err := yaml.Unmarshal([]byte(fmContent), &fm); err != nil {
		return Pack{}, err
	}

	pack.Name = fm.Name
	pack.Description = fm.Description
	pack.Source = fm.Source
	pack.Adapted = fm.Adapted
	if pack.Source != nil {
		pack.Origin = "third-party"
	}

	body := strings.Join(lines[endIdx+1:], "\n")
	body = strings.TrimPrefix(body, "\r\n")
	body = strings.TrimPrefix(body, "\n")
	pack.Body = body

	return pack, nil
}

// Load loads a skill pack by name from root.
func Load(root string, m manifest.Manifest, name string) (Pack, error) {
	relPath := ""
	if ref, ok := m.Skills[name]; ok && ref.Path != "" {
		relPath = ref.Path
	} else {
		relPath = fmt.Sprintf(".tija/skills/%s/SKILL.md", name)
	}

	fullPath := filepath.Join(root, filepath.FromSlash(relPath))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return Pack{}, fmt.Errorf("skill %q: %w", name, err)
	}

	pack, err := Parse(data)
	if err != nil {
		return Pack{}, fmt.Errorf("skill %q: %w", name, err)
	}

	if pack.Name == "" {
		pack.Name = name
	}
	pack.Path = filepath.ToSlash(relPath)

	if pack.Source != nil {
		pack.Origin = "third-party"
	} else if _, ok := assets.Files()[pack.Path]; ok {
		pack.Origin = "builtin"
	} else {
		pack.Origin = "repo"
	}

	if pack.Origin != "third-party" {
		pack.Source = nil
	}

	if pack.Origin == "third-party" {
		licPath := filepath.Join(filepath.Dir(fullPath), "LICENSE")
		if _, err := os.Stat(licPath); err != nil {
			pack.Problems = append(pack.Problems, "missing LICENSE")
		}
		if pack.Source == nil || pack.Source.Commit == "" {
			pack.Problems = append(pack.Problems, "missing source.commit")
		}
		if pack.Source == nil || pack.Source.Repo == "" {
			pack.Problems = append(pack.Problems, "missing source.repo")
		}
		if pack.Source == nil || pack.Source.Author == "" {
			pack.Problems = append(pack.Problems, "missing source.author")
		}
	}

	if m.Tija.Installed != nil {
		if recorded, ok := m.Tija.Installed[pack.Path]; ok {
			if hash(data) != recorded {
				pack.Edited = true
			}
		}
	}

	return pack, nil
}

// List discovers and returns all skill packs in m.Skills and .tija/skills/*,
// sorted by Name.
func List(root string, m manifest.Manifest) ([]Pack, error) {
	names := make(map[string]bool)
	for name := range m.Skills {
		names[name] = true
	}

	skillsDir := filepath.Join(root, ".tija", "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				candidate := filepath.Join(skillsDir, e.Name(), "SKILL.md")
				if _, err := os.Stat(candidate); err == nil {
					names[e.Name()] = true
				}
			}
		}
	}

	sortedNames := make([]string, 0, len(names))
	for name := range names {
		sortedNames = append(sortedNames, name)
	}
	slices.Sort(sortedNames)

	packs := make([]Pack, 0, len(sortedNames))
	for _, name := range sortedNames {
		p, err := Load(root, m, name)
		if err != nil {
			return nil, err
		}
		p.Body = ""
		packs = append(packs, p)
	}

	return packs, nil
}

func hash(b []byte) string {
	s := sha256.Sum256(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")))
	return hex.EncodeToString(s[:])
}
