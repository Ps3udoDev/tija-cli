package comments_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/comments"
)

func TestApplyRemovesFullLine(t *testing.T) {
	// Source with two empty lines surrounding a comment line
	src := []byte("const x = 1;\n\n// separator\n\nconst y = 2;\n")
	c := comments.Comment{
		Kind:     "line",
		Text:     "// separator",
		Start:    14,
		End:      26,
		FullLine: true,
	}

	edit := comments.PlanEdit(src, c, "remove", "")
	got, err := comments.Apply(src, []comments.Edit{edit})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Should collapse the two consecutive empty lines into a single empty line
	want := "const x = 1;\n\nconst y = 2;\n"
	if string(got) != want {
		t.Errorf("Apply got:\n%q\nwant:\n%q", string(got), want)
	}
}

func TestApplyTrailingComment(t *testing.T) {
	src := []byte("x = 1; // hi\n")
	c := comments.Comment{
		Kind:     "line",
		Text:     "// hi",
		Start:    7,
		End:      12,
		FullLine: false,
	}

	edit := comments.PlanEdit(src, c, "remove", "")
	got, err := comments.Apply(src, []comments.Edit{edit})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	want := "x = 1;\n"
	if string(got) != want {
		t.Errorf("Apply got:\n%q\nwant:\n%q", string(got), want)
	}
}

func TestApplyJSXCommentLine(t *testing.T) {
	t.Run("full line jsx comment", func(t *testing.T) {
		src := []byte("<div>\n  {/* x */}\n  <span>text</span>\n</div>\n")
		// {/* x */} starts at 8, ends at 17
		c := comments.Comment{
			Kind:     "jsx",
			Text:     "{/* x */}",
			Start:    8,
			End:      17,
			FullLine: true,
		}

		edit := comments.PlanEdit(src, c, "remove", "")
		got, err := comments.Apply(src, []comments.Edit{edit})
		if err != nil {
			t.Fatalf("Apply failed: %v", err)
		}

		want := "<div>\n  <span>text</span>\n</div>\n"
		if string(got) != want {
			t.Errorf("Apply got:\n%q\nwant:\n%q", string(got), want)
		}
	})

	t.Run("inline jsx comment", func(t *testing.T) {
		src := []byte("<a/>{/* x */}<b/>\n")
		c := comments.Comment{
			Kind:     "jsx",
			Text:     "{/* x */}",
			Start:    4,
			End:      13,
			FullLine: false,
		}

		edit := comments.PlanEdit(src, c, "remove", "")
		got, err := comments.Apply(src, []comments.Edit{edit})
		if err != nil {
			t.Fatalf("Apply failed: %v", err)
		}

		want := "<a/><b/>\n"
		if string(got) != want {
			t.Errorf("Apply got:\n%q\nwant:\n%q", string(got), want)
		}
	})
}

func TestApplyRewriteKeepsIndentAndKind(t *testing.T) {
	t.Run("line comment rewrite", func(t *testing.T) {
		src := []byte("    // old comment text\n")
		c := comments.Comment{
			Kind:     "line",
			Text:     "// old comment text",
			Start:    4,
			End:      23,
			FullLine: true,
		}

		edit := comments.PlanEdit(src, c, "rewrite", "new concise comment")
		got, err := comments.Apply(src, []comments.Edit{edit})
		if err != nil {
			t.Fatalf("Apply failed: %v", err)
		}

		want := "    // new concise comment\n"
		if string(got) != want {
			t.Errorf("Apply got:\n%q\nwant:\n%q", string(got), want)
		}
	})

	t.Run("block comment rewrite", func(t *testing.T) {
		src := []byte("  /* old block */\n")
		c := comments.Comment{
			Kind:     "block",
			Text:     "/* old block */",
			Start:    2,
			End:      17,
			FullLine: true,
		}

		edit := comments.PlanEdit(src, c, "rewrite", "new block")
		got, err := comments.Apply(src, []comments.Edit{edit})
		if err != nil {
			t.Fatalf("Apply failed: %v", err)
		}

		want := "  /* new block */\n"
		if string(got) != want {
			t.Errorf("Apply got:\n%q\nwant:\n%q", string(got), want)
		}
	})
}

func TestApplyPreservesCRLFAndBOM(t *testing.T) {
	bom := []byte("\xef\xbb\xbf")
	content := []byte("const a = 1;\r\n// separator\r\nconst b = 2;\r\n")
	src := append(bom, content...)

	// In src, comment starts at 3 + 14 = 17
	start := 3 + strings.Index(string(content), "// separator")
	end := start + len("// separator")

	c := comments.Comment{
		Kind:     "line",
		Text:     "// separator",
		Start:    start,
		End:      end,
		FullLine: true,
	}

	edit := comments.PlanEdit(src, c, "remove", "")
	got, err := comments.Apply(src, []comments.Edit{edit})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if !bytes.HasPrefix(got, bom) {
		t.Errorf("Apply did not preserve BOM")
	}
	if !bytes.Contains(got, []byte("\r\n")) {
		t.Errorf("Apply did not preserve CRLF")
	}
	want := append(bom, []byte("const a = 1;\r\nconst b = 2;\r\n")...)
	if !bytes.Equal(got, want) {
		t.Errorf("Apply got:\n%q\nwant:\n%q", string(got), string(want))
	}
}

func TestApplyRejectsOverlap(t *testing.T) {
	src := []byte("const a = 1; // comment\n")
	e1 := comments.Edit{Start: 10, End: 20, Replacement: ""}
	e2 := comments.Edit{Start: 15, End: 24, Replacement: ""}

	_, err := comments.Apply(src, []comments.Edit{e1, e2})
	if err == nil {
		t.Errorf("Apply on overlapping edits should return error, got nil")
	}
}
