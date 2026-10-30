package comments

import (
	"strings"
	"testing"
)

func TestBuildPromptContents(t *testing.T) {
	pack := "# Anti-Slop Code Rules\nAvoid restating the obvious."
	lang := "es"
	batch := []Comment{
		{
			ID:     "src/app.js#42",
			Path:   "src/app.js",
			Kind:   "line",
			Line:   10,
			Col:    5,
			Text:   "// obvious comment",
			Before: []string{"const a = 1;", "const b = 2;"},
			After:  []string{"return a + b;"},
		},
		{
			ID:     "src/app.js#100",
			Path:   "src/app.js",
			Kind:   "block",
			Line:   25,
			Col:    1,
			Text:   "/* block comment */",
			Before: []string{"function test() {"},
			After:  []string{"}"},
		},
	}

	prompt := BuildPrompt(pack, lang, batch)

	if !strings.Contains(prompt, pack) {
		t.Errorf("prompt missing pack content; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Spanish") {
		t.Errorf("prompt missing language name 'Spanish'; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Answer with a JSON array only.") {
		t.Errorf("prompt missing literal 'Answer with a JSON array only.'; got:\n%s", prompt)
	}
	for _, c := range batch {
		if !strings.Contains(prompt, c.ID) {
			t.Errorf("prompt missing comment ID %q", c.ID)
		}
		if !strings.Contains(prompt, c.Text) {
			t.Errorf("prompt missing comment text %q", c.Text)
		}
		for _, before := range c.Before {
			if !strings.Contains(prompt, before) {
				t.Errorf("prompt missing before context %q", before)
			}
		}
		for _, after := range c.After {
			if !strings.Contains(prompt, after) {
				t.Errorf("prompt missing after context %q", after)
			}
		}
	}
}

func TestParseClassificationsTolerant(t *testing.T) {
	batch := []Comment{
		{
			ID:   "src/app.js#10",
			Path: "src/app.js",
			Kind: "line",
		},
		{
			ID:   "src/app.js#20",
			Path: "src/app.js",
			Kind: "line",
		},
		{
			ID:   "src/app.js#30",
			Path: "src/app.js",
			Kind: "line",
		},
		{
			ID:   "src/app.js#40",
			Path: "src/app.js",
			Kind: "block",
		},
	}

	// 1. JSON inside code fence with prose before/after
	// 2. Unknown ID included
	// 3. Line comment rewrite with \n
	// 4. Block comment rewrite with */
	// Note: batch has src/app.js#20 which is omitted in the response (missing ID)
	rawOutput := `Here are the classifications from the agent:

` + "```json" + `
[
  {
    "id": "src/app.js#10",
    "action": "remove",
    "class": "restates-obvious",
    "confidence": "high",
    "reason": "Restates code"
  },
  {
    "id": "src/unknown.js#99",
    "action": "remove",
    "class": "labeled-separator",
    "confidence": "high",
    "reason": "Unknown file"
  },
  {
    "id": "src/app.js#30",
    "action": "rewrite",
    "class": "over-explained",
    "confidence": "high",
    "reason": "Shortened",
    "text": "first line\nsecond line"
  },
  {
    "id": "src/app.js#40",
    "action": "rewrite",
    "class": "over-explained",
    "confidence": "high",
    "reason": "Shortened",
    "text": "contains */ illegal block close"
  }
]
` + "```" + `

Please let me know if you need anything else!
`

	parsed, warnings, err := ParseClassifications(rawOutput, batch)
	if err != nil {
		t.Fatalf("ParseClassifications unexpected error: %v", err)
	}

	// Unknown ID should be discarded with a warning
	if _, ok := parsed["src/unknown.js#99"]; ok {
		t.Errorf("expected unknown ID src/unknown.js#99 to be discarded")
	}
	foundUnknownWarn := false
	for _, w := range warnings {
		if strings.Contains(w, "src/unknown.js#99") || strings.Contains(w, "unknown") {
			foundUnknownWarn = true
			break
		}
	}
	if !foundUnknownWarn {
		t.Errorf("expected warning for unknown ID, got warnings: %v", warnings)
	}

	// src/app.js#10 should be parsed OK
	c10, ok := parsed["src/app.js#10"]
	if !ok {
		t.Errorf("missing parsed result for src/app.js#10")
	} else if c10.Action != "remove" || c10.Class != "restates-obvious" || c10.Confidence != "high" {
		t.Errorf("unexpected classification for src/app.js#10: %+v", c10)
	}

	// src/app.js#20 was missing from response -> defaulted to "keep"
	c20, ok := parsed["src/app.js#20"]
	if !ok {
		t.Errorf("missing result for missing ID src/app.js#20")
	} else {
		if c20.Action != "keep" {
			t.Errorf("expected action keep for missing ID, got %q", c20.Action)
		}
		if c20.Confidence != "high" {
			t.Errorf("expected confidence high for missing ID, got %q", c20.Confidence)
		}
		if c20.Class != "keep-other" {
			t.Errorf("expected class keep-other for missing ID, got %q", c20.Class)
		}
		if c20.Reason != "missing in response" {
			t.Errorf("expected reason 'missing in response' for missing ID, got %q", c20.Reason)
		}
	}

	// src/app.js#30: line rewrite with \n -> reverted to keep with warning
	c30, ok := parsed["src/app.js#30"]
	if !ok {
		t.Errorf("missing parsed result for src/app.js#30")
	} else if c30.Action != "keep" {
		t.Errorf("expected src/app.js#30 rewrite with newline to be reverted to keep, got %q", c30.Action)
	}
	foundNewlineWarn := false
	for _, w := range warnings {
		if strings.Contains(w, "src/app.js#30") && strings.Contains(w, "newline") {
			foundNewlineWarn = true
			break
		}
	}
	if !foundNewlineWarn {
		t.Errorf("expected warning for line rewrite with newline, got: %v", warnings)
	}

	// src/app.js#40: block rewrite with */ -> reverted to keep with warning
	c40, ok := parsed["src/app.js#40"]
	if !ok {
		t.Errorf("missing parsed result for src/app.js#40")
	} else if c40.Action != "keep" {
		t.Errorf("expected src/app.js#40 rewrite with */ to be reverted to keep, got %q", c40.Action)
	}
	foundBlockWarn := false
	for _, w := range warnings {
		if strings.Contains(w, "src/app.js#40") && strings.Contains(w, "*/") {
			foundBlockWarn = true
			break
		}
	}
	if !foundBlockWarn {
		t.Errorf("expected warning for block rewrite with */, got: %v", warnings)
	}

	// Invalid JSON -> returns error
	_, _, err = ParseClassifications("this is not json { [", batch)
	if err == nil {
		t.Errorf("expected error for invalid JSON, got nil")
	}
}

func TestSafetyOf(t *testing.T) {
	tests := []struct {
		name string
		c    Classification
		want Safety
	}{
		{
			name: "keep is none",
			c:    Classification{Action: "keep", Class: "business-rule", Confidence: "high"},
			want: SafetyNone,
		},
		{
			name: "keep with low confidence is none",
			c:    Classification{Action: "keep", Class: "restates-obvious", Confidence: "low"},
			want: SafetyNone,
		},
		{
			name: "rewrite is always unsafe",
			c:    Classification{Action: "rewrite", Class: "over-explained", Confidence: "high", Text: "short"},
			want: SafetyUnsafe,
		},
		{
			name: "rewrite with safe class is still unsafe",
			c:    Classification{Action: "rewrite", Class: "restates-obvious", Confidence: "high", Text: "short"},
			want: SafetyUnsafe,
		},
		{
			name: "remove with low confidence is unsafe even if class is safe",
			c:    Classification{Action: "remove", Class: "restates-obvious", Confidence: "low"},
			want: SafetyUnsafe,
		},
		{
			name: "remove with medium confidence is unsafe",
			c:    Classification{Action: "remove", Class: "restates-obvious", Confidence: "medium"},
			want: SafetyUnsafe,
		},
		{
			name: "remove high restates-obvious is safe",
			c:    Classification{Action: "remove", Class: "restates-obvious", Confidence: "high"},
			want: SafetySafe,
		},
		{
			name: "remove high workflow-narration is safe",
			c:    Classification{Action: "remove", Class: "workflow-narration", Confidence: "high"},
			want: SafetySafe,
		},
		{
			name: "remove high empty-label is safe",
			c:    Classification{Action: "remove", Class: "empty-label", Confidence: "high"},
			want: SafetySafe,
		},
		{
			name: "remove high labeled-separator is safe",
			c:    Classification{Action: "remove", Class: "labeled-separator", Confidence: "high"},
			want: SafetySafe,
		},
		{
			name: "remove high decorative-emoji is safe",
			c:    Classification{Action: "remove", Class: "decorative-emoji", Confidence: "high"},
			want: SafetySafe,
		},
		{
			name: "remove high end-marker is safe",
			c:    Classification{Action: "remove", Class: "end-marker", Confidence: "high"},
			want: SafetySafe,
		},
		{
			name: "remove high line-by-line is safe",
			c:    Classification{Action: "remove", Class: "line-by-line", Confidence: "high"},
			want: SafetySafe,
		},
		{
			name: "remove high vague-todo is unsafe",
			c:    Classification{Action: "remove", Class: "vague-todo", Confidence: "high"},
			want: SafetyUnsafe,
		},
		{
			name: "remove high signature-echo is unsafe",
			c:    Classification{Action: "remove", Class: "signature-echo", Confidence: "high"},
			want: SafetyUnsafe,
		},
		{
			name: "remove high commented-out-code is unsafe",
			c:    Classification{Action: "remove", Class: "commented-out-code", Confidence: "high"},
			want: SafetyUnsafe,
		},
		{
			name: "remove high over-explained is unsafe",
			c:    Classification{Action: "remove", Class: "over-explained", Confidence: "high"},
			want: SafetyUnsafe,
		},
		{
			name: "unknown action is unsafe",
			c:    Classification{Action: "delete", Class: "restates-obvious", Confidence: "high"},
			want: SafetyUnsafe,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafetyOf(tt.c)
			if got != tt.want {
				t.Errorf("SafetyOf(%+v) = %q, want %q", tt.c, got, tt.want)
			}
		})
	}
}

func TestBatches(t *testing.T) {
	t.Run("3 files with 60 comments each with max 150 -> 2 batches", func(t *testing.T) {
		var cs []Comment
		files := []string{"src/a.js", "src/b.js", "src/c.js"}
		for _, f := range files {
			for i := 0; i < 60; i++ {
				cs = append(cs, Comment{
					ID:   f + "#" + string(rune('0'+i)),
					Path: f,
				})
			}
		}

		batches := Batches(cs, 150)
		if len(batches) != 2 {
			t.Fatalf("expected 2 batches, got %d", len(batches))
		}
		if len(batches[0]) != 120 {
			t.Errorf("expected batch 0 to have 120 comments (60+60), got %d", len(batches[0]))
		}
		if len(batches[1]) != 60 {
			t.Errorf("expected batch 1 to have 60 comments, got %d", len(batches[1]))
		}
	})

	t.Run("1 file with 400 comments with max 150 -> 3 batches (150, 150, 100)", func(t *testing.T) {
		var cs []Comment
		for i := 0; i < 400; i++ {
			cs = append(cs, Comment{
				ID:   "src/big.js#comment",
				Path: "src/big.js",
			})
		}

		batches := Batches(cs, 150)
		if len(batches) != 3 {
			t.Fatalf("expected 3 batches, got %d", len(batches))
		}
		if len(batches[0]) != 150 {
			t.Errorf("expected batch 0 to have 150 comments, got %d", len(batches[0]))
		}
		if len(batches[1]) != 150 {
			t.Errorf("expected batch 1 to have 150 comments, got %d", len(batches[1]))
		}
		if len(batches[2]) != 100 {
			t.Errorf("expected batch 2 to have 100 comments, got %d", len(batches[2]))
		}
	})

	t.Run("empty input returns nil or empty", func(t *testing.T) {
		batches := Batches(nil, 150)
		if len(batches) != 0 {
			t.Errorf("expected 0 batches for nil input, got %d", len(batches))
		}
	})
}
