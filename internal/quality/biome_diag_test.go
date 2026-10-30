package quality_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

func TestParseBiomeGithub(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("testdata", "biome-github.txt"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	counts, ok := quality.ParseBiomeGithub(string(content), "")
	if !ok {
		t.Fatalf("expected ok=true, got false")
	}

	formKey := "src/pages/(features)/claims/Form.jsx"
	btnKey := "src/components/Button.tsx"

	if len(counts) != 2 {
		t.Fatalf("expected 2 files in counts, got %d: %+v", len(counts), counts)
	}

	formCounts := counts[formKey]
	if formCounts == nil {
		t.Fatalf("missing entries for %s", formKey)
	}
	if got := formCounts["lint/correctness/noUnusedVariables"]; got != 2 {
		t.Errorf("expected 2 noUnusedVariables for Form.jsx, got %d", got)
	}
	if got := formCounts["lint/a11y/useButtonType"]; got != 1 {
		t.Errorf("expected 1 useButtonType for Form.jsx, got %d", got)
	}

	btnCounts := counts[btnKey]
	if btnCounts == nil {
		t.Fatalf("missing entries for %s", btnKey)
	}
	if got := btnCounts["lint/suspicious/noExplicitAny"]; got != 1 {
		t.Errorf("expected 1 noExplicitAny for Button.tsx, got %d", got)
	}
}

func TestParseBiomeGithubUnrecognized(t *testing.T) {
	t.Run("arbitrary text returns ok=false", func(t *testing.T) {
		counts, ok := quality.ParseBiomeGithub("error: unknown option --foo\nprocess exited with error", "")
		if ok {
			t.Errorf("expected ok=false for arbitrary text, got true")
		}
		if counts != nil {
			t.Errorf("expected nil counts, got %+v", counts)
		}
	})

	t.Run("empty text returns empty map ok=true", func(t *testing.T) {
		counts, ok := quality.ParseBiomeGithub("", "")
		if !ok {
			t.Errorf("expected ok=true for empty text, got false")
		}
		if counts == nil || len(counts) != 0 {
			t.Errorf("expected empty map, got %+v", counts)
		}

		countsWhitespace, ok := quality.ParseBiomeGithub("   \n\t  \n", "")
		if !ok {
			t.Errorf("expected ok=true for whitespace, got false")
		}
		if countsWhitespace == nil || len(countsWhitespace) != 0 {
			t.Errorf("expected empty map, got %+v", countsWhitespace)
		}
	})
}

func TestRegressions(t *testing.T) {
	tests := []struct {
		name   string
		before quality.DiagCounts
		after  quality.DiagCounts
		want   []quality.Regression
	}{
		{
			name: "no regressions when unchanged",
			before: quality.DiagCounts{
				"src/a.ts": {"lint/correctness/noUnusedVariables": 2},
			},
			after: quality.DiagCounts{
				"src/a.ts": {"lint/correctness/noUnusedVariables": 2},
			},
			want: nil,
		},
		{
			name: "no regressions when count decreased",
			before: quality.DiagCounts{
				"src/a.ts": {"lint/correctness/noUnusedVariables": 2},
			},
			after: quality.DiagCounts{
				"src/a.ts": {"lint/correctness/noUnusedVariables": 1},
			},
			want: nil,
		},
		{
			name: "no regressions when file resolved completely",
			before: quality.DiagCounts{
				"src/a.ts": {"lint/correctness/noUnusedVariables": 2},
			},
			after: quality.DiagCounts{},
			want:  nil,
		},
		{
			name: "regression when count increased",
			before: quality.DiagCounts{
				"src/a.ts": {"lint/correctness/noUnusedVariables": 2},
			},
			after: quality.DiagCounts{
				"src/a.ts": {"lint/correctness/noUnusedVariables": 3},
			},
			want: []quality.Regression{
				{
					Path:     "src/a.ts",
					Category: "lint/correctness/noUnusedVariables",
					Before:   2,
					After:    3,
				},
			},
		},
		{
			name: "regression when category appeared for first time",
			before: quality.DiagCounts{
				"src/a.ts": {"lint/correctness/noUnusedVariables": 2},
			},
			after: quality.DiagCounts{
				"src/a.ts": {
					"lint/correctness/noUnusedVariables":        2,
					"lint/correctness/useExhaustiveDependencies": 1,
				},
			},
			want: []quality.Regression{
				{
					Path:     "src/a.ts",
					Category: "lint/correctness/useExhaustiveDependencies",
					Before:   0,
					After:    1,
				},
			},
		},
		{
			name: "regression when new file appears in after",
			before: quality.DiagCounts{
				"src/a.ts": {"lint/correctness/noUnusedVariables": 2},
			},
			after: quality.DiagCounts{
				"src/a.ts": {"lint/correctness/noUnusedVariables": 2},
				"src/b.ts": {"syntax": 1},
			},
			want: []quality.Regression{
				{
					Path:     "src/b.ts",
					Category: "syntax",
					Before:   0,
					After:    1,
				},
			},
		},
		{
			name: "nil before handles all after as regressions",
			before: nil,
			after: quality.DiagCounts{
				"src/a.ts": {"syntax": 1},
			},
			want: []quality.Regression{
				{
					Path:     "src/a.ts",
					Category: "syntax",
					Before:   0,
					After:    1,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quality.Regressions(tt.before, tt.after)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Regressions() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBiomeDiagnosticsArgs(t *testing.T) {
	var capturedDir, capturedName string
	var capturedArgs []string

	mockEnv := sys.Env{
		LookPath: func(file string) (string, error) {
			if file == "biome" {
				return "/usr/local/bin/biome", nil
			}
			return "", errors.New("not found")
		},
		Run: func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
			capturedDir = dir
			capturedName = name
			capturedArgs = args
			// Biome exits 1 when diagnostics are found, which should not cause an error in BiomeDiagnostics.
			stdout := "::warning title=lint/correctness/noUnusedVariables,file=src/a.ts,line=10::unused\n"
			return stdout, "", 1, nil
		},
	}

	root := "/my/repo"
	files := []string{
		"src/a.ts",
		filepath.Join(root, "src/b.ts"),
	}

	counts, err := quality.BiomeDiagnostics(context.Background(), mockEnv, root, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedDir != root {
		t.Errorf("expected dir %s, got %s", root, capturedDir)
	}
	if capturedName != "/usr/local/bin/biome" {
		t.Errorf("expected name /usr/local/bin/biome, got %s", capturedName)
	}

	expectedArgs := []string{
		"check",
		"--reporter=github",
		"--max-diagnostics=none",
		"--colors=off",
		"src/a.ts",
		"src/b.ts",
	}
	if !reflect.DeepEqual(capturedArgs, expectedArgs) {
		t.Errorf("captured args = %v, want %v", capturedArgs, expectedArgs)
	}

	if counts == nil || counts["src/a.ts"]["lint/correctness/noUnusedVariables"] != 1 {
		t.Errorf("unexpected counts: %+v", counts)
	}
}

func TestBiomeDiagnosticsNotFound(t *testing.T) {
	mockEnv := sys.Env{
		LookPath: func(file string) (string, error) {
			return "", errors.New("not found")
		},
	}

	_, err := quality.BiomeDiagnostics(context.Background(), mockEnv, "/repo", []string{"src/a.ts"})
	if err == nil || err.Error() != "biome binary not found" {
		t.Fatalf("expected 'biome binary not found' error, got %v", err)
	}
}
