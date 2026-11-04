package review

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	root := t.TempDir()
	if _, ok, e := LoadSession(root); ok || e != nil {
		t.Fatalf("missing: %v %v", ok, e)
	}
	r := Result{Mode: "deep", Commit: "abc", FailOn: "high", Findings: []Finding{agent("review", 10, 12, High, "bug")}}
	r.Finalize()
	if e := SaveSession(root, r); e != nil {
		t.Fatal(e)
	}
	got, ok, e := LoadSession(root)
	if !ok || e != nil {
		t.Fatalf("%v %v", ok, e)
	}
	if _, e := time.Parse(time.RFC3339, got.SavedAt); e != nil {
		t.Fatal(e)
	}
	got.SavedAt = ""
	if !reflect.DeepEqual(r, got) {
		t.Fatalf("%#v != %#v", r, got)
	}
	r.Commit = "next"
	if e := SaveSession(root, r); e != nil {
		t.Fatal(e)
	}
	got, _, e = LoadSession(root)
	if e != nil || got.Commit != "next" {
		t.Fatalf("replace %#v %v", got, e)
	}
	if e := os.WriteFile(filepath.Join(root, filepath.FromSlash(SessionPath)), []byte("bad"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, _, e := LoadSession(root); e == nil {
		t.Fatal("accepted invalid JSON")
	}
}
