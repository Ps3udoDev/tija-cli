package comments_test

import (
	"regexp"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/comments"
)

func TestProtectedBuiltins(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		ext       string
		extra     []*regexp.Regexp
		protected bool
	}{
		{"biome-ignore", "// biome-ignore lint/correctness/useExhaustiveDependencies: required", ".jsx", nil, true},
		{"eslint-disable-next-line", "// eslint-disable-next-line no-unused-vars", ".js", nil, true},
		{"eslint-enable", "/* eslint-enable */", ".ts", nil, true},
		{"prettier-ignore", "// prettier-ignore", ".tsx", nil, true},
		{"@ts-ignore", "// @ts-ignore", ".ts", nil, true},
		{"@ts-expect-error", "// @ts-expect-error type mismatch expected", ".ts", nil, true},
		{"@ts-nocheck", "// @ts-nocheck", ".ts", nil, true},
		{"@ts-check", "// @ts-check", ".js", nil, true},
		{"istanbul ignore", "/* istanbul ignore next */", ".js", nil, true},
		{"c8 ignore", "/* c8 ignore next 2 */", ".js", nil, true},
		{"v8 ignore", "/* v8 ignore next */", ".js", nil, true},
		{"@vite-ignore", "/* @vite-ignore */", ".ts", nil, true},
		{"webpackChunkName", "/* webpackChunkName: \"my-chunk\" */", ".js", nil, true},
		{"webpackMode", "/* webpackMode: \"lazy\" */", ".js", nil, true},
		{"#__PURE__", "/*#__PURE__*/", ".js", nil, true},
		{"@__PURE__", "/*@__PURE__*/", ".js", nil, true},
		{"@jsx", "/** @jsx jsx */", ".jsx", nil, true},
		{"@jsxImportSource", "/** @jsxImportSource @emotion/react */", ".tsx", nil, true},
		{"@flow", "/* @flow */", ".js", nil, true},
		{"license bang", "/*! MIT License (c) 2026 */", ".js", nil, true},
		{"@license tag", "/** @license Apache-2.0 */", ".js", nil, true},
		{"@preserve tag", "/** @preserve DO NOT REMOVE */", ".js", nil, true},
		{"@copyright tag", "/** @copyright 2026 Acme Corp */", ".js", nil, true},
		{"triple slash", "/// <reference path=\"./types.d.ts\" />", ".ts", nil, true},
		{"shebang", "#!/usr/bin/env node", ".js", nil, true},
		{"jsdoc type in js", "/** @type {string} */", ".js", nil, true},
		{"jsdoc typedef in jsx", "/** @typedef {Object} User */", ".jsx", nil, true},
		{"jsdoc param in js", "/** @param {number} x */", ".js", nil, true},
		{"jsdoc returns in js", "/** @returns {boolean} */", ".js", nil, true},
		{"jsdoc type in ts not protected", "/** @type {string} */", ".ts", nil, false},
		{"jsdoc param in ts not protected", "/** @param {number} x */", ".ts", nil, false},
		{"extra regex pattern", "// DO_NOT_TOUCH_THIS_SECTION", ".js", []*regexp.Regexp{regexp.MustCompile(`DO_NOT_TOUCH`)}, true},
		{"regular comment", "// normal explanation of code", ".js", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := comments.Comment{
				Text: tt.text,
			}
			got := comments.Protected(c, tt.ext, tt.extra)
			if got != tt.protected {
				t.Errorf("Protected(%q, %q) = %v, want %v", tt.text, tt.ext, got, tt.protected)
			}
		})
	}
}

func TestRuleDeterministic(t *testing.T) {
	t.Run("matches separator line", func(t *testing.T) {
		c := comments.Comment{
			Kind:     "line",
			Text:     "// ---------------------------------------------------------------------------",
			FullLine: true,
		}
		v, ok := comments.Rule(c, []byte("// ---------------------------------------------------------------------------\n"))
		if !ok || v.Class != "separator" || v.Action != "remove" {
			t.Errorf("Rule() = (%+v, %v), want separator/remove/true", v, ok)
		}
	})

	t.Run("matches separator jsx", func(t *testing.T) {
		c := comments.Comment{
			Kind:     "jsx",
			Text:     "{/* ---------- */}",
			FullLine: true,
		}
		v, ok := comments.Rule(c, []byte("{/* ---------- */}\n"))
		if !ok || v.Class != "separator" || v.Action != "remove" {
			t.Errorf("Rule() = (%+v, %v), want separator/remove/true", v, ok)
		}
	})

	t.Run("matches end-marker", func(t *testing.T) {
		src := []byte("} // end if\n")
		c := comments.Comment{
			Kind:     "line",
			Text:     "// end if",
			Start:    2,
			End:      11,
			FullLine: false,
		}
		v, ok := comments.Rule(c, src)
		if !ok || v.Class != "end-marker" || v.Action != "remove" {
			t.Errorf("Rule() = (%+v, %v), want end-marker/remove/true", v, ok)
		}
	})

	t.Run("matches empty-line when neighbors have no comments", func(t *testing.T) {
		src := []byte("const a = 1;\n//\nconst b = 2;\n")
		c := comments.Comment{
			Kind:     "line",
			Text:     "//",
			Start:    13,
			End:      15,
			FullLine: true,
			Before:   []string{"const a = 1;"},
			After:    []string{"const b = 2;"},
		}
		v, ok := comments.Rule(c, src)
		if !ok || v.Class != "empty-line" || v.Action != "remove" {
			t.Errorf("Rule() = (%+v, %v), want empty-line/remove/true", v, ok)
		}
	})

	t.Run("does not match jsx banner with letters", func(t *testing.T) {
		c := comments.Comment{
			Kind:     "jsx",
			Text:     "{/* ---------- Fila 1 ---------- */}",
			FullLine: true,
		}
		_, ok := comments.Rule(c, []byte("{/* ---------- Fila 1 ---------- */}\n"))
		if ok {
			t.Errorf("Rule() matched banner with letters, want false (should go to agent)")
		}
	})

	t.Run("does not match empty comment inside contiguous block", func(t *testing.T) {
		src := []byte("// line 1\n//\n// line 2\n")
		c := comments.Comment{
			Kind:     "line",
			Text:     "//",
			Start:    10,
			End:      12,
			FullLine: true,
			Before:   []string{"// line 1"},
			After:    []string{"// line 2"},
		}
		_, ok := comments.Rule(c, src)
		if ok {
			t.Errorf("Rule() matched // inside contiguous block, want false")
		}
	})

	t.Run("does not match line with letters and dashes", func(t *testing.T) {
		c := comments.Comment{
			Kind:     "line",
			Text:     "// -- ver issue 12 --",
			FullLine: true,
		}
		_, ok := comments.Rule(c, []byte("// -- ver issue 12 --\n"))
		if ok {
			t.Errorf("Rule() matched // -- ver issue 12 --, want false")
		}
	})
}
