package comments_test

import (
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/comments"
)

func TestVerifyDetectsCodeChange(t *testing.T) {
	before := []byte(`const a = 1;
// some comment
const b = 2;
`)
	// after is missing semicolon on line 3
	after := []byte(`const a = 1;
const b = 2
`)

	err := comments.VerifyTokens(before, after, comments.Lang{TS: false, JSX: false})
	if err == nil {
		t.Fatalf("VerifyTokens should fail when code tokens change, got nil")
	}

	errMsg := err.Error()
	// Should mention line number in after
	if !strings.Contains(errMsg, "line") {
		t.Errorf("VerifyTokens error %q should mention line number", errMsg)
	}
}

func TestVerifyJSXWhitespaceNormalized(t *testing.T) {
	before := []byte(`function Component() {
  return (
    <div>
      <a />
      {/* comment in its own line */}
      <b />
    </div>
  );
}
`)

	after := []byte(`function Component() {
  return (
    <div>
      <a />
      <b />
    </div>
  );
}
`)

	err := comments.VerifyTokens(before, after, comments.Lang{TS: false, JSX: true})
	if err != nil {
		t.Errorf("VerifyTokens failed on JSX comment removal: %v", err)
	}
}
