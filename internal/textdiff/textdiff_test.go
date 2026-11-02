package textdiff

import "testing"

func TestLinesOnlyChanges(t *testing.T) {
	old := []byte("a\nb\nc\nd\n")
	new := []byte("a\nB\nc\nd\ne\n")
	if got, want := Lines(old, new), "-b\n+B\n+e"; got != want {
		t.Errorf("Lines = %q, want %q", got, want)
	}
	if got := Lines([]byte("x\r\ny\r\n"), []byte("x\ny\n")); got != "" {
		t.Errorf("CRLF-only change = %q, want empty", got)
	}
	if got := Lines(nil, []byte("x\n")); got != "+x" {
		t.Errorf("from empty = %q", got)
	}
}

func TestLinesBinary(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	if got := Lines(png, []byte("text\n")); got != BinaryDiff {
		t.Errorf("binary old = %q", got)
	}
	if got := Lines([]byte("text\n"), png); got != BinaryDiff {
		t.Errorf("binary new = %q", got)
	}
}
