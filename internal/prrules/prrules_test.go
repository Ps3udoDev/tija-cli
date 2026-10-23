package prrules

import (
	"os"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
)

func rules() manifest.PullRequest {
	return manifest.FromDetection(detect.Detection{}, nil, "main").PullRequest
}

func TestValidateTitle(t *testing.T) {
	long := "feat(x): " + strings.Repeat("a", 92) // 101 chars
	cases := []struct {
		title string
		ok    bool
	}{
		{"feat(claims): add catastrophe workflow", true},
		{"fix(ui-kit): corregir modal", true},
		{"Feat(x): a", false},
		{"feat: sin scope", false},
		{"feature(x): a", false},
		{"fix(x): termina con punto.", false},
		{long, false},
		{"fix(x):sin espacio", false},
	}
	if len(long) != 101 {
		t.Fatalf("long title is %d chars", len(long))
	}
	for _, c := range cases {
		got := ValidateTitle(c.title, rules())
		if (len(got) == 0) != c.ok {
			t.Errorf("%q: ok=%v, problems=%v", c.title, c.ok, got)
		}
	}
}

func TestValidateBodyTemplateFails(t *testing.T) {
	b, err := os.ReadFile("../assets/defaults/templates/pull-request.md")
	if err != nil {
		t.Fatal(err)
	}
	ps := ValidateBody(string(b), rules())
	if len(ps) != 4 {
		t.Fatalf("want 4 problems, got %d: %v", len(ps), ps)
	}
	for i, name := range rules().RequiredSections {
		if ps[i].Field != "section:"+name {
			t.Errorf("problem %d field = %q", i, ps[i].Field)
		}
	}
}

const okBody = "## Resumen\n\nAgrega el flujo de catástrofe.\n\n## Objetivo\n\nPermitir registrar reclamos.\n\n## Cambios realizados\n\n- Nuevo servicio\n- Nuevas pruebas\n\n## Pruebas\n\n- [x] Se ejecutó dotnet test\n"

func TestValidateBodyInvalidValues(t *testing.T) {
	r := rules()
	for _, v := range []string{"- N/A", "- pendiente"} {
		body := strings.Replace(okBody, "- [x] Se ejecutó dotnet test", v, 1)
		ps := ValidateBody(body, r)
		if len(ps) != 1 || ps[0].Field != "section:Pruebas" {
			t.Errorf("%q: %v", v, ps)
		}
	}
	if ps := ValidateBody(okBody, r); len(ps) != 0 {
		t.Errorf("valid body: %v", ps)
	}
}

func TestValidateBodyCRLF(t *testing.T) {
	body := strings.ReplaceAll(okBody, "\n", "\r\n")
	if ps := ValidateBody(body, rules()); len(ps) != 0 {
		t.Errorf("%v", ps)
	}
}

func TestValidateBodyCaseInsensitiveHeading(t *testing.T) {
	body := strings.Replace(okBody, "## Resumen", "## resumen", 1)
	if ps := ValidateBody(body, rules()); len(ps) != 0 {
		t.Errorf("%v", ps)
	}
}

func TestValidateBodyIgnoresEvidencias(t *testing.T) {
	body := okBody + "\n## Evidencias\n\nN/A\n"
	if ps := ValidateBody(body, rules()); len(ps) != 0 {
		t.Errorf("%v", ps)
	}
}

func TestValidateBodyEmpty(t *testing.T) {
	ps := ValidateBody("<!-- nada -->", rules())
	if len(ps) == 0 || ps[0].Field != "body" {
		t.Errorf("%v", ps)
	}
}

func TestMatchTitle(t *testing.T) {
	for _, s := range []string{"feat(pr): x", "fix: y", "feat(a/b)!: z"} {
		if !MatchTitle(s) {
			t.Errorf("MatchTitle(%q) = false", s)
		}
	}
	for _, s := range []string{"", "add stuff", "feat(x) missing colon", "feat(x):no space"} {
		if MatchTitle(s) {
			t.Errorf("MatchTitle(%q) = true", s)
		}
	}
}
