package comments_test

import (
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/comments"
)

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"src/**/*.{js,jsx}", "src/a/b/c.jsx", true},
		{"src/**/*.{js,jsx}", "src/a.ts", false},
		{"dist/**", "dist/x/y.js", true},
		{"dist/**", "dist/app.js", true},
		{"dist/**", "src/dist/x.js", false},
		{"**/*.min.js", "app.min.js", true},
		{"**/*.min.js", "dist/vendor/lib.min.js", true},
		{"**/*.min.js", "lib.js", false},
		{"src/**/*.{ts,tsx}", "src/components/Button.tsx", true},
		{"src/**/*.{ts,tsx}", "src/index.ts", true},
		{"{foo,bar}/*.js", "foo/index.js", true},
		{"{foo,bar}/*.js", "bar/index.js", true},
		{"{foo,bar}/*.js", "baz/index.js", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			got := comments.Match(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("Match(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}
