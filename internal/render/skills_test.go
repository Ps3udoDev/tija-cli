package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/skills"
)

func sampleSkills() []skills.Pack {
	return []skills.Pack{
		{
			Name:        "antislop-code",
			Path:        ".tija/skills/antislop-code/SKILL.md",
			Description: "Classify code comments as slop or valuable. Never touches code.",
			Origin:      "third-party",
			Source: &skills.Source{
				Project:   "anti-slop",
				Author:    "Miqdad Badjuber",
				Repo:      "https://github.com/miqdadbadjuber/anti-slop",
				Path:      "skills/antislop-code/SKILL.md",
				Commit:    "3005235d030b4793146ad9afe8d01045efd90e1d",
				Version:   "3.2.24",
				License:   "MIT",
				Copyright: "Copyright (c) 2026 Miqdad Badjuber (antislop)",
			},
			Edited: false,
		},
		{
			Name:        "antislop-copywriting",
			Path:        ".tija/skills/antislop-copywriting/SKILL.md",
			Description: "Clean slop from technical PR descriptions.",
			Origin:      "third-party",
			Source: &skills.Source{
				Project:   "anti-slop",
				Author:    "Miqdad Badjuber",
				Repo:      "https://github.com/miqdadbadjuber/anti-slop",
				Path:      "skills/antislop-copywriting/SKILL.md",
				Commit:    "3005235d030b4793146ad9afe8d01045efd90e1d",
				Version:   "3.2.24",
				License:   "MIT",
				Copyright: "Copyright (c) 2026 Miqdad Badjuber (antislop)",
			},
			Edited: false,
		},
		{
			Name:        "custom-team-rules",
			Path:        ".tija/skills/custom-team-rules/SKILL.md",
			Description: "Internal team guidelines.",
			Origin:      "repo",
			Edited:      true,
		},
		{
			Name:        "thermo-quality",
			Path:        ".tija/skills/thermo-quality/SKILL.md",
			Description: "Review code maintainability.",
			Origin:      "third-party",
			Source: &skills.Source{
				Project:   "Thermos",
				Author:    "Cursor",
				Repo:      "https://github.com/cursor/plugins",
				Path:      "thermos/skills/thermo-nuclear-code-quality-review/SKILL.md",
				Commit:    "1ba91fc25c0977741dd0e267849e6234b037ed0a",
				License:   "MIT",
				Copyright: "Copyright (c) 2026 Cursor",
			},
			Edited: false,
		},
	}
}

func TestRenderSkillsPlain(t *testing.T) {
	got := renderPlain(func(p Printer) { Skills(p, sampleSkills()) })
	for _, want := range []string{
		"NAME", "ORIGIN", "AUTHOR", "LICENSE", "COMMIT", "EDITED",
		"antislop-code", "third-party", "Miqdad Badjuber", "MIT", "3005235", "no",
		"custom-team-rules", "repo", "-", "yes",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	golden(t, "skills_plain", got)
}

func TestRenderSkillsJSON(t *testing.T) {
	var buf bytes.Buffer
	p := Printer{W: &buf, Mode: JSON}
	Skills(p, sampleSkills())
	got := buf.String()
	for _, want := range []string{
		`"name": "antislop-code"`,
		`"origin": "third-party"`,
		`"author": "Miqdad Badjuber"`,
		`"edited": false`,
		`"name": "custom-team-rules"`,
		`"origin": "repo"`,
		`"edited": true`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	golden(t, "skills_json", got)
}

func TestRenderSkillShowPlain(t *testing.T) {
	pack := sampleSkills()[0]
	pack.Body = "> Adaptado de anti-slop.\n\n# antislop-code\nRule content here.\n"

	got := renderPlain(func(p Printer) { SkillShow(p, pack) })
	for _, want := range []string{
		"Skill:          antislop-code\n",
		"Origin:         third-party\n",
		"Author:         Miqdad Badjuber\n",
		"License:        MIT\n",
		"Commit:         3005235\n",
		"Path:           .tija/skills/antislop-code/SKILL.md\n",
		"Edited:         no\n",
		"> Adaptado de anti-slop.\n",
		"Rule content here.\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
}
