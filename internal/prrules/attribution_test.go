package prrules

import (
	"strings"
	"testing"
)

func TestStripAttributionPositives(t *testing.T) {
	pats, err := AttributionPatterns(nil)
	if err != nil {
		t.Fatalf("AttributionPatterns failed: %v", err)
	}

	tests := []struct {
		name       string
		input      string
		wantSub    string // content that must remain
		notWantSub string // content that must be stripped
		wantCount  int
	}{
		{
			name: "claude code footer",
			input: strings.Join([]string{
				"## Summary",
				"Added user authentication.",
				"",
				"🤖 Generated with [Claude Code](https://claude.com/claude-code)",
				"",
			}, "\n"),
			wantSub:    "## Summary\nAdded user authentication.",
			notWantSub: "Generated with [Claude Code]",
			wantCount:  1,
		},
		{
			name: "generated with codex",
			input: strings.Join([]string{
				"## Summary",
				"Refactored payments.",
				"",
				"Generated with Codex",
			}, "\n"),
			wantSub:    "## Summary\nRefactored payments.",
			notWantSub: "Generated with Codex",
			wantCount:  1,
		},
		{
			name: "co-authored-by claude",
			input: strings.Join([]string{
				"feat(auth): add login form",
				"",
				"Detailed description.",
				"",
				"Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>",
			}, "\n"),
			wantSub:    "Detailed description.",
			notWantSub: "noreply@anthropic.com",
			wantCount:  1,
		},
		{
			name: "made with cursor",
			input: strings.Join([]string{
				"## Changes",
				"- Updated schema",
				"",
				"Made with Cursor",
			}, "\n"),
			wantSub:    "## Changes\n- Updated schema",
			notWantSub: "Made with Cursor",
			wantCount:  1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, count := StripAttribution(tc.input, pats)
			if count != tc.wantCount {
				t.Errorf("count = %d, want %d", count, tc.wantCount)
			}
			if !strings.Contains(got, tc.wantSub) {
				t.Errorf("got %q, want it to contain %q", got, tc.wantSub)
			}
			if strings.Contains(got, tc.notWantSub) {
				t.Errorf("got %q, must not contain %q", got, tc.notWantSub)
			}
			// Must have collapsed trailing empty lines
			if strings.HasSuffix(got, "\n") && strings.TrimSpace(got) == "" {
				t.Errorf("got has unexpected trailing blank space: %q", got)
			}
		})
	}
}

func TestStripAttributionNegatives(t *testing.T) {
	pats, err := AttributionPatterns(nil)
	if err != nil {
		t.Fatalf("AttributionPatterns failed: %v", err)
	}

	negatives := []string{
		"Integramos Codex de impuestos en el módulo",
		"Co-Authored-By: Ana <ana@empresa.com>",
		"Se generó el reporte con la librería claude-parser",
	}

	for _, text := range negatives {
		t.Run(text, func(t *testing.T) {
			got, count := StripAttribution(text, pats)
			if count != 0 {
				t.Errorf("StripAttribution(%q) removed %d lines, want 0", text, count)
			}
			if got != text {
				t.Errorf("StripAttribution(%q) = %q, want original", text, got)
			}
		})
	}
}

func TestCheckAttribution(t *testing.T) {
	pats, err := AttributionPatterns(nil)
	if err != nil {
		t.Fatalf("AttributionPatterns failed: %v", err)
	}

	bodyWithAttribution := strings.Join([]string{
		"## Resumen",
		"Implementación terminada.",
		"",
		"🤖 Generated with [Claude Code](https://claude.com/claude-code)",
	}, "\n")

	problems := CheckAttribution(bodyWithAttribution, pats)
	if len(problems) != 1 {
		t.Fatalf("expected 1 problem, got %d: %+v", len(problems), problems)
	}
	if problems[0].Field != "body" {
		t.Errorf("Field = %q, want %q", problems[0].Field, "body")
	}
	wantMsg := "agent attribution: 🤖 Generated with [Claude Code](https://claude.com/claude-code)"
	if problems[0].Message != wantMsg {
		t.Errorf("Message = %q, want %q", problems[0].Message, wantMsg)
	}

	cleanBody := strings.Join([]string{
		"## Resumen",
		"Implementación terminada.",
		"",
		"Co-Authored-By: Ana <ana@empresa.com>",
	}, "\n")
	if cleanProblems := CheckAttribution(cleanBody, pats); len(cleanProblems) != 0 {
		t.Errorf("expected 0 problems for clean body, got %+v", cleanProblems)
	}
}
