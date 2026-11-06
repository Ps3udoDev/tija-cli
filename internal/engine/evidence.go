package engine

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type EvidenceCase struct {
	ID     string `json:"id,omitempty"`
	Code   string `json:"code,omitempty"`
	Title  string `json:"title,omitempty"`
	Status string `json:"status,omitempty"`
}

type EvidenceProject struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// EvidenceRef identifies external evidence. Tija validates its structure only.
// ExportPath is input-only; ExportName is computed and safe to persist.
type EvidenceRef struct {
	Source     string          `json:"source"`
	Project    EvidenceProject `json:"project"`
	Cases      []EvidenceCase  `json:"cases,omitempty"`
	ExportPath string          `json:"exportPath,omitempty"`
	ExportName string          `json:"exportName,omitempty"`
	Notes      string          `json:"notes,omitempty"`
}

type EvidenceValidation struct {
	Structural        string `json:"structural"`
	ExternalExistence string `json:"externalExistence"`
}

func evidenceError(field, reason string) error {
	return fmt.Errorf("invalid evidenceRefs: %s: %s", field, reason)
}

// NormalizeEvidence validates caller input, copies it, and discards export paths.
// Stored references have output-only ExportName and need separate revalidation
// when draft reuse is implemented; they must not be passed as fresh input.
func NormalizeEvidence(refs []EvidenceRef) ([]EvidenceRef, error) {
	if len(refs) > 10 {
		return nil, evidenceError("evidenceRefs", "at most 10 references are allowed")
	}
	if refs == nil {
		return nil, nil
	}
	out := make([]EvidenceRef, len(refs))
	for i, ref := range refs {
		prefix := fmt.Sprintf("[%d]", i)
		if ref.Source != "qapture" {
			return nil, evidenceError(prefix+".source", `only "qapture" is supported`)
		}
		if strings.TrimSpace(ref.Project.ID) == "" {
			return nil, evidenceError(prefix+".project.id", "is required")
		}
		if len(ref.Cases) > 50 {
			return nil, evidenceError(prefix+".cases", "at most 50 cases are allowed")
		}
		if ref.ExportPath == "" && ref.ExportName != "" {
			return nil, evidenceError(prefix+".exportName", "is output-only; provide exportPath instead")
		}
		if len(ref.Cases) == 0 && ref.ExportPath == "" && strings.TrimSpace(ref.Notes) == "" {
			return nil, evidenceError(prefix, "at least one of cases, exportPath or notes is required")
		}
		fields := []struct{ name, value string }{
			{"source", ref.Source}, {"project.id", ref.Project.ID}, {"project.name", ref.Project.Name},
			{"exportPath", ref.ExportPath}, {"notes", ref.Notes},
		}
		for j, c := range ref.Cases {
			cp := fmt.Sprintf("cases[%d]", j)
			if strings.TrimSpace(c.ID) == "" && strings.TrimSpace(c.Code) == "" {
				return nil, evidenceError(prefix+"."+cp, "id or code is required")
			}
			fields = append(fields, struct{ name, value string }{cp + ".id", c.ID}, struct{ name, value string }{cp + ".code", c.Code}, struct{ name, value string }{cp + ".title", c.Title}, struct{ name, value string }{cp + ".status", c.Status})
		}
		for _, field := range fields {
			if strings.ContainsAny(field.value, "\r\n\u0085\u2028\u2029") {
				return nil, evidenceError(prefix+"."+field.name, "must not contain line breaks")
			}
			if !utf8.ValidString(field.value) {
				return nil, evidenceError(prefix+"."+field.name, "must be valid UTF-8")
			}
			if utf8.RuneCountInString(field.value) > 200 {
				return nil, evidenceError(prefix+"."+field.name, "must be at most 200 characters")
			}
		}
		ref.ExportName = ""
		if ref.ExportPath != "" {
			ref.ExportName = baseName(ref.ExportPath)
			if ref.ExportName == "" || ref.ExportName == "." || ref.ExportName == ".." {
				return nil, evidenceError(prefix+".exportPath", "must end with a file name")
			}
		}
		ref.ExportPath = ""
		if ref.Cases != nil {
			ref.Cases = append([]EvidenceCase{}, ref.Cases...)
		}
		out[i] = ref
	}
	return out, nil
}

// baseName recognizes Windows and Unix separators on every host platform.
func baseName(p string) string { return p[strings.LastIndexAny(p, `/\`)+1:] }

func renderEvidence(refs []EvidenceRef) string {
	var lines []string
	for _, ref := range refs {
		name := ref.Project.Name
		if name == "" {
			name = ref.Project.ID
		}
		lines = append(lines, "- Qapture · proyecto «"+name+"»")
		for _, c := range ref.Cases {
			id := c.Code
			if id == "" {
				id = c.ID
			}
			line := "  - " + id
			if c.Title != "" {
				line += " " + c.Title
			}
			if c.Status != "" {
				line += " — " + c.Status
			}
			lines = append(lines, line)
		}
		if ref.ExportName != "" {
			lines = append(lines, "  - Matriz exportada: "+ref.ExportName)
		}
		if ref.Notes != "" {
			lines = append(lines, "  - Nota: "+ref.Notes)
		}
	}
	return strings.Join(lines, "\n")
}

func enforceEvidence(body string, refs []EvidenceRef) string {
	if len(refs) == 0 {
		return body
	}
	body = strings.ReplaceAll(body, "\r\n", "\n")
	head := regexp.MustCompile(`(?im)^##[ \t]+Evidencias[ \t]*$`)
	sections := head.FindAllStringIndex(body, -1)
	// Remove later sections backwards so their offsets remain valid. Keeping
	// just the first heading prevents a second agent-written block surviving.
	for i := len(sections) - 1; i > 0; i-- {
		loc := sections[i]
		end := len(body)
		if next := regexp.MustCompile(`(?m)^##[ \t]+`).FindStringIndex(body[loc[1]:]); next != nil {
			end = loc[1] + next[0]
		}
		body = body[:loc[0]] + body[end:]
	}
	if len(sections) == 0 {
		body = strings.TrimRight(body, "\r\n") + "\n\n## Evidencias\n"
	}
	return replaceSection(body, "Evidencias", func(string) string { return renderEvidence(refs) })
}
