package review

import "testing"

func TestAttachPRComments(t *testing.T) {
	findings := []Finding{{File: "src/a.ts", Line: 10, EndLine: 12}}
	f, u := AttachPRComments(findings, []PRComment{{Path: "src\\a.ts", Line: 8, URL: "match"}, {Path: "other", Line: 10}, {Path: "src/a.ts", Line: 16}, {Path: "src/a.ts", Line: 0}})
	if len(f[0].PRComments) != 1 || f[0].PRComments[0].URL != "match" || len(u) != 3 {
		t.Fatalf("%#v %#v", f, u)
	}
	if len(findings[0].PRComments) != 0 {
		t.Fatal("mutated input")
	}
}
