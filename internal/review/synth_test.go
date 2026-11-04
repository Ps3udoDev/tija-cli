package review

import (
	"reflect"
	"testing"
)

func agent(pass string, line, end int, severity Severity, rule string) Finding {
	return Finding{Source: "agent", Passes: []string{pass}, File: "a.ts", Line: line, EndLine: end, Severity: severity, Rule: rule}
}
func TestMergeConfirmed(t *testing.T) {
	in := []Finding{agent("review", 10, 12, High, "bug"), agent("quality", 11, 11, High, "bug")}
	f := Merge(in)
	if len(f) != 1 || !reflect.DeepEqual(f[0].ConfirmedBy, []string{"quality", "review"}) || !reflect.DeepEqual(f[0].Passes, []string{"quality", "review"}) {
		t.Fatalf("%#v", f)
	}
	if len(in[0].ConfirmedBy) != 0 {
		t.Fatal("mutated input")
	}
	reversed := Merge([]Finding{in[1], in[0]})
	if !reflect.DeepEqual(f, reversed) {
		t.Fatalf("unstable merge %#v %#v", f, reversed)
	}
}
func TestMergeDisputed(t *testing.T) {
	f := Merge([]Finding{agent("review", 10, 12, Medium, "bug"), agent("quality", 11, 11, High, "bug")})
	if len(f) != 1 || f[0].Severity != High || !f[0].Disputed || !reflect.DeepEqual(f[0].Severities, []Severity{High, Medium}) {
		t.Fatalf("%#v", f)
	}
}
func TestMergeDifferentRulesKept(t *testing.T) {
	if f := Merge([]Finding{agent("review", 10, 12, High, "bug"), agent("quality", 11, 11, High, "style")}); len(f) != 2 {
		t.Fatalf("%#v", f)
	}
}
func TestMergeSkipsChecks(t *testing.T) {
	a := agent("review", 10, 12, High, "bug")
	b := a
	b.Source = "check"
	if f := Merge([]Finding{a, b}); len(f) != 2 {
		t.Fatalf("%#v", f)
	}
}
func TestMergeTransitiveOverlap(t *testing.T) {
	f := Merge([]Finding{agent("review", 10, 12, Medium, "bug"), agent("quality", 14, 16, High, "bug"), agent("quality", 12, 14, Low, "bug")})
	if len(f) != 1 || f[0].Line != 10 || f[0].EndLine != 16 {
		t.Fatalf("%#v", f)
	}
}
func TestSortAndIDs(t *testing.T) {
	f := []Finding{{Severity: Medium}, {Severity: High, File: "a", Line: 1}, {Severity: High, File: "z", ConfirmedBy: []string{"quality", "review"}}, {Severity: Critical}}
	Sort(f)
	if f[0].Severity != Critical || f[1].File != "z" || f[2].File != "a" || f[3].Severity != Medium || f[3].ID != "F4" {
		t.Fatalf("%#v", f)
	}
}
