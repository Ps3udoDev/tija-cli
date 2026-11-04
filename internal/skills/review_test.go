package skills_test

import (
	"github.com/Ps3udoDev/tija-cli/internal/assets"
	"github.com/Ps3udoDev/tija-cli/internal/skills"
	"reflect"
	"strings"
	"testing"
)

func TestParseSkillRules(t *testing.T) {
	p, err := skills.Parse([]byte("---\nname: x\nrules: [a, b]\n---\nbody"))
	if err != nil || !reflect.DeepEqual(p.Rules, []string{"a", "b"}) {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestThermoPacksDeclareRules(t *testing.T) {
	for name, want := range map[string][]string{
		"thermo-review":  {"breaking-functionality", "devex-regression", "feature-leak", "security-vulnerability"},
		"thermo-quality": {"code-judo", "1000-lines-ceiling", "spaghetti-condition", "clean-design", "type-boundary", "canonical-layer", "clean-orchestration"},
	} {
		b, _ := assets.Skill(name)
		p, err := skills.Parse(b)
		if err != nil || !reflect.DeepEqual(p.Rules, want) {
			t.Fatalf("%s: %+v %v", name, p.Rules, err)
		}
		if p.Adapted == nil || !strings.Contains(strings.Join(p.Adapted.Changes, " "), "rules") {
			t.Fatalf("%s lacks adaptation entry", name)
		}
	}
}
