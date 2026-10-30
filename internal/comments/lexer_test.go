package comments_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/Ps3udoDev/tija-cli/internal/comments"
)

func TestLangFor(t *testing.T) {
	tests := []struct {
		path   string
		want   comments.Lang
		wantOk bool
	}{
		{"app.js", comments.Lang{TS: false, JSX: true}, true},
		{"Component.jsx", comments.Lang{TS: false, JSX: true}, true},
		{"index.mjs", comments.Lang{TS: false, JSX: false}, true},
		{"config.cjs", comments.Lang{TS: false, JSX: false}, true},
		{"types.ts", comments.Lang{TS: true, JSX: false}, true},
		{"utils.mts", comments.Lang{TS: true, JSX: false}, true},
		{"helpers.cts", comments.Lang{TS: true, JSX: false}, true},
		{"Button.tsx", comments.Lang{TS: true, JSX: true}, true},
		{"UPPER.TSX", comments.Lang{TS: true, JSX: true}, true},
		{"UPPER.JS", comments.Lang{TS: false, JSX: true}, true},
		{"package.json", comments.Lang{}, false},
		{"main.go", comments.Lang{}, false},
		{"styles.css", comments.Lang{}, false},
		{"README.md", comments.Lang{}, false},
		{"script", comments.Lang{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, ok := comments.LangFor(tt.path)
			if ok != tt.wantOk {
				t.Fatalf("LangFor(%q) ok = %v, want %v", tt.path, ok, tt.wantOk)
			}
			if got != tt.want {
				t.Fatalf("LangFor(%q) = %+v, want %+v", tt.path, got, tt.want)
			}
		})
	}
}

func TestLexTrickyFixtures(t *testing.T) {
	// Table of inline tricky fixtures as specified
	inlineTests := []struct {
		name          string
		src           string
		lang          comments.Lang
		wantComments  []string
		checkJSXChild bool
	}{
		{
			name:         "string url",
			src:          `"http://x"`,
			lang:         comments.Lang{TS: false, JSX: true},
			wantComments: nil,
		},
		{
			name:         "string block comment",
			src:          `'/* no */'`,
			lang:         comments.Lang{TS: false, JSX: true},
			wantComments: nil,
		},
		{
			name:         "template with comment inside interpolation",
			src:          "`a // b ${c /* d */} e`",
			lang:         comments.Lang{TS: false, JSX: true},
			wantComments: []string{"/* d */"},
		},
		{
			name:         "regex character class with comment-like chars",
			src:          `const r = /[/*]/g; // real`,
			lang:         comments.Lang{TS: false, JSX: true},
			wantComments: []string{"// real"},
		},
		{
			name:         "division followed by comment",
			src:          `a = b / c; // real`,
			lang:         comments.Lang{TS: false, JSX: true},
			wantComments: []string{"// real"},
		},
		{
			name:         "multiple divisions",
			src:          `x = y / z / w;`,
			lang:         comments.Lang{TS: false, JSX: true},
			wantComments: nil,
		},
		{
			name:          "jsx text and comment",
			src:           `<p>// no {/* si */}</p>`,
			lang:          comments.Lang{TS: false, JSX: true},
			wantComments:  []string{"/* si */"},
			checkJSXChild: true,
		},
		{
			name:         "ts generic and comment",
			src:          `foo<T>(x); // c`,
			lang:         comments.Lang{TS: true, JSX: false},
			wantComments: []string{"// c"},
		},
		{
			name:         "multi-line contiguous line comments",
			src:          "// line 1\n// line 2\n// line 3",
			lang:         comments.Lang{TS: false, JSX: true},
			wantComments: []string{"// line 1", "// line 2", "// line 3"},
		},
		{
			name:         "crlf line endings",
			src:          "const a = 1;\r\n// crlf comment\r\nconst b = 2;\r\n",
			lang:         comments.Lang{TS: false, JSX: true},
			wantComments: []string{"// crlf comment"},
		},
	}

	for _, tt := range inlineTests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := comments.Lex([]byte(tt.src), tt.lang)
			if err != nil {
				t.Fatalf("Lex error: %v", err)
			}

			var gotComments []string
			var foundJSXChild *comments.Token
			for i := range tokens {
				tok := &tokens[i]
				if tok.Kind == comments.TokenLineComment || tok.Kind == comments.TokenBlockComment {
					gotComments = append(gotComments, tok.Text)
					if tok.InJSXChild {
						foundJSXChild = tok
					}
				}
			}

			if len(gotComments) != len(tt.wantComments) {
				t.Fatalf("got %d comments (%q), want %d (%q)", len(gotComments), gotComments, len(tt.wantComments), tt.wantComments)
			}
			for i := range gotComments {
				if gotComments[i] != tt.wantComments[i] {
					t.Errorf("comment[%d] = %q, want %q", i, gotComments[i], tt.wantComments[i])
				}
			}

			if tt.checkJSXChild {
				if foundJSXChild == nil {
					t.Fatalf("expected InJSXChild comment, got none")
				}
				if !foundJSXChild.InJSXChild {
					t.Errorf("expected InJSXChild == true")
				}
				if foundJSXChild.OuterStart >= foundJSXChild.Start || foundJSXChild.OuterEnd <= foundJSXChild.End {
					t.Errorf("invalid OuterStart/OuterEnd (%d, %d) for comment (%d, %d)",
						foundJSXChild.OuterStart, foundJSXChild.OuterEnd, foundJSXChild.Start, foundJSXChild.End)
				}
				outer := tt.src[foundJSXChild.OuterStart:foundJSXChild.OuterEnd]
				if !strings.HasPrefix(outer, "{") || !strings.HasSuffix(outer, "}") {
					t.Errorf("outer span %q does not start with { and end with }", outer)
				}
			}
		})
	}

	// Also verify the testdata files and their .want files
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".want") {
			continue
		}
		lang, ok := comments.LangFor(name)
		if !ok {
			continue
		}
		t.Run("fixture/"+name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}

			wantData, err := os.ReadFile(filepath.Join("testdata", name+".want"))
			if err != nil {
				t.Fatalf("read %s.want: %v", name, err)
			}

			tokens, err := comments.Lex(src, lang)
			if err != nil {
				t.Fatalf("Lex error: %v", err)
			}

			var gotComments []string
			for _, tok := range tokens {
				if tok.Kind == comments.TokenLineComment || tok.Kind == comments.TokenBlockComment {
					gotComments = append(gotComments, tok.Text)
				}
			}

			wantStr := strings.TrimSpace(string(wantData))
			var wantComments []string
			if wantStr != "" {
				// Normalize CRLF in .want file lines
				normalized := strings.ReplaceAll(wantStr, "\r\n", "\n")
				wantComments = strings.Split(normalized, "\n")
			}

			if len(gotComments) != len(wantComments) {
				t.Fatalf("fixture %s: got %d comments (%q), want %d (%q)",
					name, len(gotComments), gotComments, len(wantComments), wantComments)
			}
			for i := range gotComments {
				if gotComments[i] != wantComments[i] {
					t.Errorf("fixture %s: comment[%d] = %q, want %q", name, i, gotComments[i], wantComments[i])
				}
			}
		})
	}
}

func TestLexUncertainSkips(t *testing.T) {
	tests := []struct {
		name string
		src  string
		lang comments.Lang
	}{
		{"unclosed string single quote", "'unclosed string", comments.Lang{TS: false, JSX: false}},
		{"unclosed string double quote", `"unclosed string`, comments.Lang{TS: false, JSX: false}},
		{"unclosed block comment", "/* unclosed comment", comments.Lang{TS: false, JSX: false}},
		{"unclosed template literal", "`hello ${x}", comments.Lang{TS: false, JSX: false}},
		{"unclosed div in jsx", "<div><span></span>", comments.Lang{TS: false, JSX: true}},
		{"mismatched jsx tags", "<div></span>", comments.Lang{TS: false, JSX: true}},
		{"unclosed regex literal", "const r = /unclosed", comments.Lang{TS: false, JSX: false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := comments.Lex([]byte(tt.src), tt.lang)
			if err == nil {
				t.Fatalf("expected ErrUncertain, got nil")
			}
			if !errors.Is(err, comments.ErrUncertain) {
				t.Fatalf("got error %v, want errors.Is(ErrUncertain)", err)
			}
		})
	}
}

func TestLexConcatenationRoundTrip(t *testing.T) {
	snippets := []struct {
		name string
		src  string
		lang comments.Lang
	}{
		{"string", `"http://x"`, comments.Lang{TS: false, JSX: true}},
		{"template", "`a // b ${c /* d */} e`", comments.Lang{TS: false, JSX: true}},
		{"regex", `const r = /[/*]/g; // real`, comments.Lang{TS: false, JSX: true}},
		{"division", `a = b / c; // real`, comments.Lang{TS: false, JSX: true}},
		{"divisions", `x = y / z / w;`, comments.Lang{TS: false, JSX: true}},
		{"jsx with comment", `<p>// no {/* si */}</p>`, comments.Lang{TS: false, JSX: true}},
		{"jsx self closing", `<img src="foo.png" alt="bar" />`, comments.Lang{TS: false, JSX: true}},
		{"jsx nested", `<div><span>hello</span><br /></div>`, comments.Lang{TS: false, JSX: true}},
		{"jsx attribute expr", `<button onClick={() => alert(1)}>Click</button>`, comments.Lang{TS: false, JSX: true}},
		{"jsx fragment", `<><div>1</div><div>2</div></>`, comments.Lang{TS: false, JSX: true}},
		{"ts generic", `foo<T>(x); // c`, comments.Lang{TS: true, JSX: false}},
		{"crlf", "const a = 1;\r\n// crlf comment\r\nconst b = 2;\r\n", comments.Lang{TS: false, JSX: true}},
		{"shebang and bom", "\xef\xbb\xbf#!/usr/bin/env node\nconst x = 1;\n", comments.Lang{TS: false, JSX: false}},
		{"arrow and regex", `const fn = () => /abc/.test("a");`, comments.Lang{TS: false, JSX: false}},
		{"ternary regex", `cond ? /foo/ : /bar/`, comments.Lang{TS: false, JSX: false}},
		{"multi-line comments", "// line 1\n// line 2\n// line 3\n", comments.Lang{TS: false, JSX: false}},
		{"complex expression", `return foo ? bar(1, 2) : baz;`, comments.Lang{TS: false, JSX: false}},
	}

	for _, tt := range snippets {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte(tt.src)
			tokens, err := comments.Lex(src, tt.lang)
			if err != nil {
				t.Fatalf("Lex error: %v", err)
			}

			var reconstructed bytes.Buffer
			lastEnd := 0

			for i, tok := range tokens {
				if tok.Start < lastEnd {
					t.Fatalf("token %d (%q) overlaps: start %d < lastEnd %d", i, tok.Text, tok.Start, lastEnd)
				}
				if tok.End > len(src) {
					t.Fatalf("token %d (%q) end %d > len(src) %d", i, tok.Text, tok.End, len(src))
				}
				if tok.Start > tok.End {
					t.Fatalf("token %d (%q) start %d > end %d", i, tok.Text, tok.Start, tok.End)
				}

				// Gap between last token end and current token start must be whitespace
				gap := src[lastEnd:tok.Start]
				for _, b := range gap {
					if !unicode.IsSpace(rune(b)) {
						t.Fatalf("non-whitespace byte %q in gap between offset %d and %d in %q",
							string(b), lastEnd, tok.Start, tt.src)
					}
				}
				reconstructed.Write(gap)

				// Token text must match src[Start:End]
				expectedText := string(src[tok.Start:tok.End])
				if tok.Text != expectedText {
					t.Fatalf("token %d text %q != src[%d:%d] %q", i, tok.Text, tok.Start, tok.End, expectedText)
				}
				reconstructed.WriteString(tok.Text)
				lastEnd = tok.End
			}

			// Trailing gap must be whitespace
			trailing := src[lastEnd:]
			for _, b := range trailing {
				if !unicode.IsSpace(rune(b)) {
					t.Fatalf("non-whitespace byte %q in trailing gap in %q", string(b), tt.src)
				}
			}
			reconstructed.Write(trailing)

			if !bytes.Equal(reconstructed.Bytes(), src) {
				t.Fatalf("reconstructed != src:\ngot:  %q\nwant: %q", reconstructed.String(), string(src))
			}
		})
	}
}

func TestLexCorpusIfAvailable(t *testing.T) {
	corpusPath := os.Getenv("TIJA_LEX_CORPUS")
	if corpusPath == "" {
		t.Skip("TIJA_LEX_CORPUS not set; skipping corpus test")
	}

	var checked int
	var uncertainFiles []string

	err := filepath.Walk(corpusPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		lang, ok := comments.LangFor(path)
		if !ok {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		checked++
		tokens, err := comments.Lex(src, lang)
		if err != nil {
			if errors.Is(err, comments.ErrUncertain) {
				uncertainFiles = append(uncertainFiles, path)
				return nil
			}
			t.Errorf("file %s: unexpected error: %v", path, err)
			return nil
		}

		// Concatenation check
		var lastEnd int
		for i, tok := range tokens {
			if tok.Start < lastEnd || tok.End > len(src) {
				t.Errorf("file %s: token %d out of bounds [%d:%d]", path, i, tok.Start, tok.End)
				break
			}
			lastEnd = tok.End
		}
		return nil
	})

	if err != nil {
		t.Fatalf("corpus walk error: %v", err)
	}

	t.Logf("Checked %d files in corpus. Uncertain files: %d", checked, len(uncertainFiles))
	for _, f := range uncertainFiles {
		t.Logf("uncertain: %s", f)
	}
	if len(uncertainFiles) > 0 {
		t.Errorf("%d files had ErrUncertain in corpus", len(uncertainFiles))
	}
}
