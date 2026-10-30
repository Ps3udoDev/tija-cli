package skills_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/skills"
)

func hashLF(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestParseFrontmatter(t *testing.T) {
	content := []byte(`---
name: antislop-code
description: Classify code comments as slop or valuable.
source:
  project: anti-slop
  author: Miqdad Badjuber
  repo: https://github.com/miqdadbadjuber/anti-slop
  path: skills/antislop-code/SKILL.md
  commit: 3005235d030b4793146ad9afe8d01045efd90e1d
  version: 3.2.24
  license: MIT
  copyright: "Copyright (c) 2026 Miqdad Badjuber (antislop)"
adapted:
  by: Tija
  date: 2026-10-29
  changes:
    - Removed allowed-tools.
---

> Adaptado de anti-slop de Miqdad Badjuber.

# antislop-code
Body text here.
`)

	pack, err := skills.Parse(content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if pack.Name != "antislop-code" {
		t.Errorf("Name = %q, want %q", pack.Name, "antislop-code")
	}
	if pack.Description != "Classify code comments as slop or valuable." {
		t.Errorf("Description = %q", pack.Description)
	}
	if pack.Source == nil {
		t.Fatal("Source is nil, want parsed Source")
	}
	if pack.Source.Project != "anti-slop" || pack.Source.Author != "Miqdad Badjuber" || pack.Source.License != "MIT" {
		t.Errorf("Source = %+v", pack.Source)
	}
	if pack.Source.Commit != "3005235d030b4793146ad9afe8d01045efd90e1d" {
		t.Errorf("Commit = %q", pack.Source.Commit)
	}
	if pack.Adapted == nil {
		t.Fatal("Adapted is nil, want parsed Adapted")
	}
	if pack.Adapted.By != "Tija" || len(pack.Adapted.Changes) != 1 {
		t.Errorf("Adapted = %+v", pack.Adapted)
	}

	wantBody := `> Adaptado de anti-slop de Miqdad Badjuber.

# antislop-code
Body text here.
`
	if pack.Body != wantBody {
		t.Errorf("Body = %q, want %q", pack.Body, wantBody)
	}
}

func TestListMarksEdited(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, ".tija", "skills", "test-pack")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	origContent := []byte("---\nname: test-pack\n---\nOriginal body\n")
	editedContent := []byte("---\nname: test-pack\n---\nEdited body\n")

	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, editedContent, 0o644); err != nil {
		t.Fatal(err)
	}

	relPath := ".tija/skills/test-pack/SKILL.md"
	m := manifest.Manifest{
		Skills: map[string]manifest.SkillRef{
			"test-pack": {Path: relPath},
		},
		Tija: manifest.Meta{
			Installed: map[string]string{
				relPath: hashLF(origContent),
			},
		},
	}

	packs, err := skills.List(tmp, m)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(packs) != 1 {
		t.Fatalf("len(packs) = %d, want 1", len(packs))
	}
	if !packs[0].Edited {
		t.Errorf("pack.Edited = false, want true")
	}
}

func TestListReportsMissingLicense(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, ".tija", "skills", "antislop-code")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := []byte(`---
name: antislop-code
source:
  project: anti-slop
  author: Miqdad Badjuber
  repo: https://github.com/miqdadbadjuber/anti-slop
  commit: 3005235d030b4793146ad9afe8d01045efd90e1d
  license: MIT
---
# antislop-code
`)
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	m := manifest.Manifest{
		Skills: map[string]manifest.SkillRef{
			"antislop-code": {Path: ".tija/skills/antislop-code/SKILL.md"},
		},
	}

	packs, err := skills.List(tmp, m)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(packs) != 1 {
		t.Fatalf("len(packs) = %d, want 1", len(packs))
	}

	pack := packs[0]
	if pack.Origin != "third-party" {
		t.Errorf("Origin = %q, want 'third-party'", pack.Origin)
	}

	hasMissingLicense := false
	for _, prob := range pack.Problems {
		if prob == "missing LICENSE" {
			hasMissingLicense = true
			break
		}
	}
	if !hasMissingLicense {
		t.Errorf("pack.Problems = %v, want to contain 'missing LICENSE'", pack.Problems)
	}
}
