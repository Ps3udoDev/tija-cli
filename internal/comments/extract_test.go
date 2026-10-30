package comments_test

import (
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/comments"
)

func TestExtract(t *testing.T) {
	src := []byte(`// line 1
const x = 1; // trailing

/*
  block comment
*/

function App() {
  return (
    <div>
      {/* jsx comment */}
      <span />
    </div>
  );
}
`)

	cs, err := comments.Extract("src\\components\\App.jsx", src, comments.Lang{TS: false, JSX: true})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(cs) != 4 {
		t.Fatalf("Extract returned %d comments, want 4", len(cs))
	}

	// 1: // line 1
	c0 := cs[0]
	if c0.Kind != "line" {
		t.Errorf("c0.Kind = %q, want line", c0.Kind)
	}
	if c0.Path != "src/components/App.jsx" && c0.Path != "src\\components\\App.jsx" {
		// path should be slash-separated or match input
	}
	if c0.ID != "src/components/App.jsx#0" {
		t.Errorf("c0.ID = %q, want src/components/App.jsx#0", c0.ID)
	}
	if c0.Line != 1 || c0.Col != 1 {
		t.Errorf("c0.Line, Col = %d, %d, want 1, 1", c0.Line, c0.Col)
	}
	if !c0.FullLine {
		t.Errorf("c0.FullLine = false, want true")
	}
	if len(c0.Before) != 0 {
		t.Errorf("c0.Before = %v, want empty", c0.Before)
	}
	if len(c0.After) != 3 {
		t.Errorf("c0.After len = %d, want 3", len(c0.After))
	}

	// 2: // trailing
	c1 := cs[1]
	if c1.Kind != "line" {
		t.Errorf("c1.Kind = %q, want line", c1.Kind)
	}
	if c1.Line != 2 {
		t.Errorf("c1.Line = %d, want 2", c1.Line)
	}
	if c1.FullLine {
		t.Errorf("c1.FullLine = true, want false for trailing comment")
	}
	if len(c1.Before) != 1 || c1.Before[0] != "// line 1" {
		t.Errorf("c1.Before = %v, want [// line 1]", c1.Before)
	}

	// 3: block comment
	c2 := cs[2]
	if c2.Kind != "block" {
		t.Errorf("c2.Kind = %q, want block", c2.Kind)
	}
	if !c2.FullLine {
		t.Errorf("c2.FullLine = false, want true for block comment on its own lines")
	}

	// 4: jsx comment
	c3 := cs[3]
	if c3.Kind != "jsx" {
		t.Errorf("c3.Kind = %q, want jsx", c3.Kind)
	}
	if c3.Text != "{/* jsx comment */}" {
		t.Errorf("c3.Text = %q, want {/* jsx comment */}", c3.Text)
	}
	if !c3.FullLine {
		t.Errorf("c3.FullLine = false, want true for JSX comment on its own line")
	}
}

func TestSoleBlockComments(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string // texts expected to be marked
	}{
		{"empty catch", "try {\n\tf();\n} catch (_e) {\n\t// Fallback\n}\n", []string{"// Fallback"}},
		{"empty function", "function noop() { /* nothing */ }\n", []string{"/* nothing */"}},
		{"two comments in empty block", "if (x) {\n\t// a\n\t// b\n}\n", []string{"// a", "// b"}},
		{"block with code", "if (x) {\n\t// a\n\tf();\n}\n", nil},
		{"jsx comment between tags", "const C = () => (\n\t<div>\n\t\t{/* x */}\n\t</div>\n);\n", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := []byte(tc.src)
			lang, _ := comments.LangFor("x.jsx")
			cs, err := comments.Extract("x.jsx", src, lang)
			if err != nil {
				t.Fatal(err)
			}
			sole := comments.SoleBlockComments(cs, src)
			var got []string
			for _, c := range cs {
				if sole[c.ID] {
					got = append(got, c.Text)
				}
			}
			if len(got) != len(tc.want) {
				t.Fatalf("marked %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("marked %q, want %q", got, tc.want)
				}
			}
		})
	}
}
