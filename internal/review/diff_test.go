package review

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseDiffBasic(t *testing.T) {
	text := "diff --git a/a.ts b/a.ts\n--- a/a.ts\n+++ b/a.ts\n@@ -1,2 +1,3 @@\n keep\n-old\n+new\n+extra\n@@ -10 +11 @@\n-before\n+after\ndiff --git a/b.ts b/b.ts\n--- a/b.ts\n+++ b/b.ts\n@@ -0,0 +1 @@\n+hello\n"
	files := ParseDiff(text)
	if len(files) != 2 || files[0].Path != "a.ts" || files[1].Path != "b.ts" {
		t.Fatalf("files: %#v", files)
	}
	if !reflect.DeepEqual(files[0].Hunks, []Hunk{{NewStart: 1, NewLines: 3, Added: []int{2, 3}}, {NewStart: 11, NewLines: 1, Added: []int{11}}}) || files[0].ChangedLines() != 5 {
		t.Fatalf("hunks: %#v", files[0])
	}
	if !reflect.DeepEqual(ParseDiff(strings.ReplaceAll(text, "\n", "\r\n")), files) {
		t.Fatal("CRLF changed parsing")
	}
}

func TestParseDiffQuotedAndRenamed(t *testing.T) {
	text := "diff --git \"a/src/x y.ts\" \"b/src/x y.ts\"\n--- \"a/src/x y.ts\"\n+++ \"b/src/x y.ts\"\n@@ -1 +1 @@\n-a\n+b\ndiff --git a/src/old.ts b/src/new.ts\nsimilarity index 90%\nrename from src/old.ts\nrename to src/new.ts\n--- a/src/old.ts\n+++ b/src/new.ts\n@@ -1 +1 @@\n-a\n+b\n"
	f := ParseDiff(text)
	if len(f) != 2 || f[0].Path != "src/x y.ts" || f[1].Path != "src/new.ts" {
		t.Fatalf("paths: %#v", f)
	}
	escaped := ParseDiff("diff --git \"a/caf\\303\\251.ts\" \"b/caf\\303\\251.ts\"\n")
	if len(escaped) != 1 || escaped[0].Path != "café.ts" {
		t.Fatalf("quoted UTF8: %#v", escaped)
	}
}

func TestParseDiffDeletedAndBinary(t *testing.T) {
	f := ParseDiff("diff --git a/a b/a\n--- a/a\n+++ /dev/null\n@@ -1 +0,0 @@\n-gone\ndiff --git a/b b/b\nBinary files a/b and b/b differ\n")
	if len(f) != 2 || !f[0].Deleted || len(f[0].Hunks) != 0 || !f[1].Binary {
		t.Fatalf("files: %#v", f)
	}
}

func TestTouches(t *testing.T) {
	f := FileDiff{Hunks: []Hunk{{NewStart: 10, NewLines: 5}}}
	for _, v := range []struct {
		a, b int
		want bool
	}{{14, 14, true}, {15, 20, false}, {1, 10, true}, {0, 1, false}, {14, 12, false}} {
		if f.Touches(v.a, v.b) != v.want {
			t.Errorf("Touches(%d,%d)", v.a, v.b)
		}
	}
}

func TestSyntheticAdded(t *testing.T) {
	f := SyntheticAdded("src\\a.ts", "a\r\nb\r\nc\r\n")
	if f.Path != "src/a.ts" || !reflect.DeepEqual(f.AddedLines(), []int{1, 2, 3}) || !strings.HasPrefix(f.Text, "--- /dev/null\n") {
		t.Fatalf("file: %#v", f)
	}
	if len(SyntheticAdded("empty", "").AddedLines()) != 0 {
		t.Fatal("empty file has lines")
	}
}
