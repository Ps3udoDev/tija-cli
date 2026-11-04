package review

import "testing"

func TestChunksByDirectory(t *testing.T) {
	files := []FileDiff{{Path: "b/z", Removed: 1500}, {Path: "a/b", Removed: 1000}, {Path: "a/a", Removed: 1000}, {Path: "deleted", Deleted: true}, {Path: "binary", Binary: true}}
	c := Chunks(files, 3000)
	if len(c) != 2 || len(c[0]) != 2 || c[0][0].Path != "a/a" || c[1][0].Path != "b/z" {
		t.Fatalf("chunks: %#v", c)
	}
	if len(Chunks(files, 10000)) != 1 {
		t.Fatal("expected one chunk")
	}
	c = Chunks([]FileDiff{{Path: "a/small", Removed: 1}, {Path: "a/huge", Removed: 5000}, {Path: "b/other", Removed: 2}}, 3000)
	if len(c) != 2 || len(c[0]) != 1 || c[0][0].Path != "a/huge" {
		t.Fatalf("oversized: %#v", c)
	}
	if files[0].Path != "b/z" {
		t.Fatal("Chunks mutated input")
	}
}

func TestChunksKeepNestedDirectoryGroupsComplete(t *testing.T) {
	files := []FileDiff{{Path: "a/a.ts", Removed: 5}, {Path: "a/sub/x.ts", Removed: 5}, {Path: "a/z.ts", Removed: 5}}
	c := Chunks(files, 10)
	if len(c) != 2 || len(c[0]) != 2 || c[0][0].Path != "a/a.ts" || c[0][1].Path != "a/z.ts" || len(c[1]) != 1 || c[1][0].Path != "a/sub/x.ts" {
		t.Fatalf("directory group split by a nested directory: %#v", c)
	}
}
