package comments

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/manifest"
)

type Classification struct {
	ID         string `json:"id"`
	Action     string `json:"action"`     // "keep" | "remove" | "rewrite"
	Class      string `json:"class"`      // classification category
	Confidence string `json:"confidence"` // "high" | "low"
	Reason     string `json:"reason"`
	Text       string `json:"text,omitempty"` // replacement text when action is "rewrite"
}

type Safety string

const (
	SafetySafe   Safety = "safe"
	SafetyUnsafe Safety = "unsafe"
	SafetyNone   Safety = "none"
)

// SafetyOf evaluates the safety of a classification:
// - "keep" -> SafetyNone ("none")
// - "rewrite" -> SafetyUnsafe ("unsafe")
// - "remove" with Confidence != "high" -> SafetyUnsafe ("unsafe")
// - "remove" with Confidence == "high" AND Class in:
//   {"restates-obvious", "workflow-narration", "empty-label", "labeled-separator",
//    "decorative-emoji", "end-marker", "line-by-line"} -> SafetySafe ("safe")
// - any other "remove" -> SafetyUnsafe ("unsafe")
func SafetyOf(c Classification) Safety {
	switch c.Action {
	case "keep":
		return SafetyNone
	case "rewrite":
		return SafetyUnsafe
	case "remove":
		if c.Confidence != "high" {
			return SafetyUnsafe
		}
		switch c.Class {
		case "restates-obvious",
			"workflow-narration",
			"empty-label",
			"labeled-separator",
			"decorative-emoji",
			"end-marker",
			"line-by-line":
			return SafetySafe
		default:
			return SafetyUnsafe
		}
	default:
		return SafetyUnsafe
	}
}

// BuildPrompt builds the LLM prompt with:
// 1. Pack markdown body
// 2. Language name (e.g. "Spanish" for "es", via manifest.LanguageName)
// 3. Comments list: for each comment in batch, its ID, kind, line, Col, Text, Before context lines, and After context lines
// 4. Strict instruction: "Answer with a JSON array only."
func BuildPrompt(pack, language string, batch []Comment) string {
	langName := manifest.LanguageName(language)
	var b strings.Builder

	b.WriteString(strings.TrimSpace(pack))
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "Classify each of the following comments. Write all reasons and rewritten texts in %s.\n\n", langName)
	b.WriteString("Comments to classify:\n\n")

	for _, c := range batch {
		fmt.Fprintf(&b, "### Comment %s\n", c.ID)
		fmt.Fprintf(&b, "- ID: %s\n", c.ID)
		fmt.Fprintf(&b, "- Kind: %s\n", c.Kind)
		fmt.Fprintf(&b, "- Location: line %d, col %d\n", c.Line, c.Col)
		if len(c.Before) > 0 {
			b.WriteString("- Context before:\n")
			for _, line := range c.Before {
				fmt.Fprintf(&b, "    %s\n", line)
			}
		}
		fmt.Fprintf(&b, "- Comment text:\n    %s\n", c.Text)
		if len(c.After) > 0 {
			b.WriteString("- Context after:\n")
			for _, line := range c.After {
				fmt.Fprintf(&b, "    %s\n", line)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("Answer with a JSON array only.\n")
	return b.String()
}

var codeFenceRx = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

func extractJSONArray(out string) ([]byte, error) {
	// 1. Look inside markdown code fences: ```json ... ``` or ``` ... ```
	matches := codeFenceRx.FindAllStringSubmatch(out, -1)
	for _, m := range matches {
		content := m[1]
		start := strings.IndexByte(content, '[')
		end := strings.LastIndexByte(content, ']')
		if start >= 0 && end > start {
			candidate := strings.TrimSpace(content[start : end+1])
			var items []Classification
			if err := json.Unmarshal([]byte(candidate), &items); err == nil {
				return []byte(candidate), nil
			}
		}
	}

	// 2. Look for outermost [ and ] in the entire string
	start := strings.IndexByte(out, '[')
	end := strings.LastIndexByte(out, ']')
	if start >= 0 && end > start {
		candidate := strings.TrimSpace(out[start : end+1])
		var items []Classification
		if err := json.Unmarshal([]byte(candidate), &items); err == nil {
			return []byte(candidate), nil
		}
	}

	// 3. Fallback to direct unmarshal on trimmed string
	trimmed := strings.TrimSpace(out)
	var items []Classification
	if err := json.Unmarshal([]byte(trimmed), &items); err == nil {
		return []byte(trimmed), nil
	}

	return nil, errors.New("no valid JSON array found in agent output")
}

// ParseClassifications parses the model output JSON array tolerant of:
// - markdown code fences (```json ... ``` or ``` ...) and prose before/after
// - returns map[string]Classification keyed by ID, slice of warning messages, and error if JSON is unparseable
// Tolerant rules:
// - Unknown IDs: discarded with a warning message
// - Missing IDs from batch: defaulted to action "keep" (Confidence "high", Class "keep-other", Reason "missing in response")
// - For action "rewrite":
//   - If comment Kind is "line" and Text has newline "\n": revert action to "keep" with a warning
//   - If comment Kind is "block" and Text contains "*/": revert action to "keep" with a warning
func ParseClassifications(out string, batch []Comment) (map[string]Classification, []string, error) {
	raw, err := extractJSONArray(out)
	if err != nil {
		return nil, nil, err
	}

	var items []Classification
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, nil, fmt.Errorf("unmarshal classifications: %w", err)
	}

	batchMap := make(map[string]Comment, len(batch))
	for _, c := range batch {
		batchMap[c.ID] = c
	}

	var warnings []string
	result := make(map[string]Classification, len(batch))

	for _, item := range items {
		orig, ok := batchMap[item.ID]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("unknown comment ID %q ignored", item.ID))
			continue
		}

		if item.Action == "rewrite" {
			if orig.Kind == "line" && strings.Contains(item.Text, "\n") {
				item.Action = "keep"
				warnings = append(warnings, fmt.Sprintf("line comment %s: rewrite contains newline; reverted to keep", item.ID))
			} else if (orig.Kind == "block" || orig.Kind == "jsx") && strings.Contains(item.Text, "*/") {
				item.Action = "keep"
				warnings = append(warnings, fmt.Sprintf("block comment %s: rewrite contains '*/'; reverted to keep", item.ID))
			}
		}

		result[item.ID] = item
	}

	// Missing IDs from batch: defaulted to action "keep"
	for _, c := range batch {
		if _, ok := result[c.ID]; !ok {
			result[c.ID] = Classification{
				ID:         c.ID,
				Action:     "keep",
				Class:      "keep-other",
				Confidence: "high",
				Reason:     "missing in response",
			}
		}
	}

	return result, warnings, nil
}

// Batches groups comments into batches of up to max comments.
// It groups by file (comments with the same Path stay together in the same batch).
// It does not split comments of a single file across batches unless that single file has more comments than max.
func Batches(cs []Comment, max int) [][]Comment {
	if len(cs) == 0 {
		return nil
	}
	if max <= 0 {
		max = 150
	}

	// Group comments by file in order of appearance
	type fileGroup struct {
		path     string
		comments []Comment
	}
	var files []fileGroup
	fileIndex := make(map[string]int)
	for _, c := range cs {
		if idx, ok := fileIndex[c.Path]; ok {
			files[idx].comments = append(files[idx].comments, c)
		} else {
			fileIndex[c.Path] = len(files)
			files = append(files, fileGroup{path: c.Path, comments: []Comment{c}})
		}
	}

	var batches [][]Comment
	var current []Comment

	for _, fg := range files {
		// If the file alone exceeds max, flush current batch first
		if len(fg.comments) > max {
			if len(current) > 0 {
				batches = append(batches, current)
				current = nil
			}
			// Split this file's comments in chunks of max
			for i := 0; i < len(fg.comments); i += max {
				end := i + max
				if end > len(fg.comments) {
					end = len(fg.comments)
				}
				chunk := fg.comments[i:end]
				if len(chunk) == max {
					batches = append(batches, chunk)
				} else {
					current = append([]Comment(nil), chunk...)
				}
			}
			continue
		}

		// File fits in max. Can it fit in the current batch?
		if len(current)+len(fg.comments) <= max {
			current = append(current, fg.comments...)
		} else {
			if len(current) > 0 {
				batches = append(batches, current)
			}
			current = append([]Comment(nil), fg.comments...)
		}
	}

	if len(current) > 0 {
		batches = append(batches, current)
	}

	return batches
}
