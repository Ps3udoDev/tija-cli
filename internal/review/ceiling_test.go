package review

import "testing"

func TestCeiling(t *testing.T) {
	f := Ceiling([]LineCount{{"a", 990, 1005}, {"b", 1005, 1100}, {"c", 900, 1000}})
	if len(f) != 1 || f[0].Source != "check" || f[0].Rule != "1000-lines-ceiling" || f[0].Severity != Medium || f[0].Line != 1001 || f[0].EndLine != 1001 || f[0].File != "a" {
		t.Fatalf("%#v", f)
	}
}
