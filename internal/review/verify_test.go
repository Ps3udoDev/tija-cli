package review

import (
	"reflect"
	"strings"
	"testing"
)

func verifyFixture() (VerifyInput, AgentFinding) {
	in := VerifyInput{Pass: "review", Files: map[string]FileDiff{"src/a.ts": {Path: "src/a.ts", Hunks: []Hunk{{NewStart: 10, NewLines: 4}}}}, Rules: map[string]bool{"bug": true}, ReadLines: func(string) ([]string, error) {
		return strings.Split("1\n2\n3\n4\n5\n6\n7\n8\n9\ncode here\n11\n12\n13\n14\n15\n16\n17\n18", "\n"), nil
	}}
	return in, AgentFinding{File: "src\\a.ts", Line: 10, Severity: "high", Rule: "bug", Title: "Bug", Evidence: "code here"}
}
func TestVerifyReasons(t *testing.T) {
	cases := []struct {
		name, want string
		change     func(*VerifyInput, *AgentFinding)
	}{
		{"valid", "", func(*VerifyInput, *AgentFinding) {}},
		{"file", ReasonFileNotInDiff, func(_ *VerifyInput, f *AgentFinding) { f.File = "other"; f.Severity = "urgent" }},
		{"line", ReasonLineOutsideDiff, func(_ *VerifyInput, f *AgentFinding) { f.Line = 20 }},
		{"evidence", ReasonEvidenceNotFound, func(_ *VerifyInput, f *AgentFinding) { f.Evidence = "invented" }},
		{"severity", ReasonInvalidSeverity, func(_ *VerifyInput, f *AgentFinding) { f.Severity = "urgent" }},
		{"rule", ReasonUnknownRule, func(_ *VerifyInput, f *AgentFinding) { f.Rule = "invented" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in, f := verifyFixture()
			c.change(&in, &f)
			got, d := Verify(in, []AgentFinding{f})
			if c.want == "" {
				if len(got) != 1 || len(d) != 0 || got[0].Source != "agent" || got[0].EndLine != 10 || got[0].Severity != High || !reflect.DeepEqual(got[0].Passes, []string{"review"}) {
					t.Fatalf("%#v %#v", got, d)
				}
			} else if len(got) != 0 || len(d) != 1 || d[0].Reason != c.want || d[0].Pass != "review" {
				t.Fatalf("%#v %#v", got, d)
			}
		})
	}
}
func TestVerifyEvidenceWindow(t *testing.T) {
	for _, c := range []struct {
		line     int
		evidence string
		valid    bool
	}{{13, "code here", true}, {14, "code here", false}, {10, " code\t  here ", true}, {10, "", false}, {10, "code here\n11\n12\n13\n14\n15", false}} {
		in, f := verifyFixture()
		in.Files["src/a.ts"] = FileDiff{Hunks: []Hunk{{NewStart: 1, NewLines: 18}}}
		f.Line = c.line
		f.Evidence = c.evidence
		got, d := Verify(in, []AgentFinding{f})
		if (len(got) == 1) != c.valid {
			t.Fatalf("%#v => %#v %#v", c, got, d)
		}
	}
}
func TestVerifyCRLF(t *testing.T) {
	in, f := verifyFixture()
	a, _ := Verify(in, []AgentFinding{f})
	read := in.ReadLines
	in.ReadLines = func(p string) ([]string, error) {
		lines, e := read(p)
		for i := range lines {
			lines[i] += "\r"
		}
		return lines, e
	}
	b, _ := Verify(in, []AgentFinding{f})
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("%#v != %#v", a, b)
	}
}
func TestVerifyDeletedFile(t *testing.T) {
	in, f := verifyFixture()
	in.Files["src/a.ts"] = FileDiff{Deleted: true, Hunks: []Hunk{{NewStart: 10, NewLines: 4}}}
	got, d := Verify(in, []AgentFinding{f})
	if len(got) != 0 || len(d) != 1 || d[0].Reason != ReasonLineOutsideDiff {
		t.Fatalf("%#v %#v", got, d)
	}
}
