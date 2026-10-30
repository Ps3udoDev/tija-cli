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

func TestApplyAdjacentRemovalsLeaveOneBlankLine(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			"banner between blank lines",
			"const a = 1;\n\n// ------------\n// Section\n// ------------\n\nconst b = 2;\n",
			"const a = 1;\n\nconst b = 2;\n",
		},
		{
			"banner between blank lines CRLF",
			"const a = 1;\r\n\r\n// ------------\r\n// Section\r\n// ------------\r\n\r\nconst b = 2;\r\n",
			"const a = 1;\r\n\r\nconst b = 2;\r\n",
		},
		{
			"jsx rows between blank lines",
			"const C = () => (\n\t<div>\n\t\t<A />\n\n\t\t{/* ---- Fila 1 ---- */}\n\t\t{/* ---- Fila 2 ---- */}\n\n\t\t<B />\n\t</div>\n);\n",
			"const C = () => (\n\t<div>\n\t\t<A />\n\n\t\t<B />\n\t</div>\n);\n",
		},
		{
			"banner at start of file",
			"// ------------\n// Section\n// ------------\n\nconst b = 2;\n",
			"const b = 2;\n",
		},
		{
			"banner at end of file",
			"const a = 1;\n\n// ------------\n// end\n",
			"const a = 1;\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := []byte(tc.src)
			lang, _ := comments.LangFor("x.jsx")
			cs, err := comments.Extract("x.jsx", src, lang)
			if err != nil {
				t.Fatal(err)
			}
			var edits []comments.Edit
			for _, c := range cs {
				edits = append(edits, comments.PlanEdit(src, c, "remove", ""))
			}
			got, err := comments.Apply(src, edits)
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

func TestApplyRemovalAtBlockEdgeLeavesNoBlankLine(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			"first jsx child before a blank line",
			"const C = () => (\n\t<div>\n\t\t{/* ---- Fila 1 ---- */}\n\n\t\t<A />\n\t</div>\n);\n",
			"const C = () => (\n\t<div>\n\t\t<A />\n\t</div>\n);\n",
		},
		{
			"multi-line opening tag",
			"const C = () => (\n\t<div\n\t\tclassName=\"x\"\n\t>\n\t\t{/* ---- Fila 1 ---- */}\n\n\t\t<A />\n\t</div>\n);\n",
			"const C = () => (\n\t<div\n\t\tclassName=\"x\"\n\t>\n\t\t<A />\n\t</div>\n);\n",
		},
		{
			"last jsx child after a blank line",
			"const C = () => (\n\t<div>\n\t\t<A />\n\n\t\t{/* ---- fin ---- */}\n\t</div>\n);\n",
			"const C = () => (\n\t<div>\n\t\t<A />\n\t</div>\n);\n",
		},
		{
			"first statement in a function body",
			"function f() {\n\t// ------------\n\n\treturn 1;\n}\n",
			"function f() {\n\treturn 1;\n}\n",
		},
		{
			"first statement in a function body CRLF",
			"function f() {\r\n\t// ------------\r\n\r\n\treturn 1;\r\n}\r\n",
			"function f() {\r\n\treturn 1;\r\n}\r\n",
		},
		{
			"sibling self-closing tag keeps its blank line",
			"const C = () => (\n\t<div>\n\t\t<A />\n\t\t{/* ---- Fila 2 ---- */}\n\n\t\t<B />\n\t</div>\n);\n",
			"const C = () => (\n\t<div>\n\t\t<A />\n\n\t\t<B />\n\t</div>\n);\n",
		},
		{
			"arrow function line keeps its blank line",
			"const f = () =>\n\t// ------------\n\n\t1;\n",
			"const f = () =>\n\n\t1;\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := []byte(tc.src)
			lang, _ := comments.LangFor("x.jsx")
			cs, err := comments.Extract("x.jsx", src, lang)
			if err != nil {
				t.Fatal(err)
			}
			var edits []comments.Edit
			for _, c := range cs {
				edits = append(edits, comments.PlanEdit(src, c, "remove", ""))
			}
			got, err := comments.Apply(src, edits)
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}
