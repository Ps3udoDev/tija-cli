package branch

import (
	"strings"
	"testing"
)

func testRules() Rules {
	return Rules{Prefixes: DefaultPrefixes, Bases: DefaultBases, Target: "dev"}
}

func TestClassify(t *testing.T) {
	const notFollow = "does not follow <prefix>/<short-description>"
	tests := []struct {
		name      string
		state     State
		suggested string
		reason    string
		issue     int
	}{
		{"feat/shared-imputation-form", "conforming", "", "", 0},
		{"bugfix/133-password-leak", "conforming", "", "", 133},
		{"release/1.2.3", "conforming", "", "", 0},
		{"release/v2.0.0-rc.1", "conforming", "", "", 0},
		{"main", "base", "", "", 0},
		{"dev", "base", "", "", 0},
		{"changes", "nonconforming", "feat/short-description", "generic branch name", 0},
		{"101-fixclaims-fix-integration-issues-and-ui-bugs-in-claims-module", "nonconforming", "fix/101-claims-integration-issues", notFollow, 101},
		{"Feat/Upper-Case", "nonconforming", "feat/upper-case", notFollow, 0},
		{"accidents-refactor", "nonconforming", "refactor/accidents", notFollow, 0},
		{"release/some-words", "nonconforming", "feat/release-some-words", notFollow, 0},
		{"bug-login-crash", "nonconforming", "fix/login-crash", notFollow, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Classify(tt.name, false, testRules())
			if v.State != tt.state {
				t.Fatalf("state = %q, want %q (%+v)", v.State, tt.state, v)
			}
			if v.Suggested != tt.suggested {
				t.Errorf("suggested = %q, want %q", v.Suggested, tt.suggested)
			}
			if v.Reason != tt.reason {
				t.Errorf("reason = %q, want %q", v.Reason, tt.reason)
			}
			if v.IssueNumber != tt.issue {
				t.Errorf("issue = %d, want %d", v.IssueNumber, tt.issue)
			}
			if v.Name != tt.name {
				t.Errorf("name = %q", v.Name)
			}
			if tt.suggested != "" && !strings.HasSuffix(tt.suggested, "/short-description") {
				if s := Classify(tt.suggested, false, testRules()); s.State != "conforming" {
					t.Errorf("suggested %q is not conforming: %+v", tt.suggested, s)
				}
			}
		})
	}
}

func TestClassifyDetached(t *testing.T) {
	v := Classify("HEAD", true, testRules())
	if v.State != "detached" {
		t.Fatalf("state = %q", v.State)
	}
}

func TestClassifyHuskyPrefixes(t *testing.T) {
	r := Rules{Prefixes: []string{"feat", "fix"}, Bases: DefaultBases, Target: "dev"}
	v := Classify("ci/pipeline", false, r)
	if v.State != "nonconforming" {
		t.Fatalf("state = %q", v.State)
	}
}

func TestDefaultBaseBranches(t *testing.T) {
	wantBases := []string{"main", "master", "dev", "develop", "desarrollo"}
	for _, b := range wantBases {
		v := Classify(b, false, Rules{Prefixes: DefaultPrefixes, Bases: DefaultBases})
		if v.State != Base {
			t.Errorf("Classify(%q) = %q, want Base", b, v.State)
		}
	}
}
