package agents

import (
	"errors"
	"testing"
)

func TestParseDraftTolerant(t *testing.T) {
	const title = "feat(pr): generate drafts"
	const body = "## Resumen\nAlgo.\n\n## Cambios\n- uno"
	cases := map[string]string{
		"exact":     "<<<TITLE\n" + title + "\nTITLE>>>\n<<<BODY\n" + body + "\nBODY>>>\n",
		"preamble":  "Claro, aquí está:\n\n<<<TITLE\n" + title + "\nTITLE>>>\n\n<<<BODY\n" + body + "\nBODY>>>\nEspero que sirva.",
		"fenced":    "<<<TITLE\n" + title + "\nTITLE>>>\n<<<BODY\n```markdown\n" + body + "\n```\nBODY>>>",
		"fencedAll": "```\n<<<TITLE\n" + title + "\nTITLE>>>\n<<<BODY\n" + body + "\nBODY>>>\n```",
		"quoted":    "<<<TITLE\n\"" + title + "\"\nTITLE>>>\n<<<BODY\n" + body + "\nBODY>>>",
		"backtick":  "<<<TITLE\n`" + title + "`\nTITLE>>>\n<<<BODY\n" + body + "\nBODY>>>",
		"ansi":      "\x1b[1m<<<TITLE\n" + title + "\nTITLE>>>\x1b[0m\n<<<BODY\n" + body + "\nBODY>>>",
		"crlf":      "<<<TITLE\r\n" + title + "\r\nTITLE>>>\r\n<<<BODY\r\n## Resumen\r\nAlgo.\r\n\r\n## Cambios\r\n- uno\r\nBODY>>>",
		"nomarkers": "Here you go:\n\n" + title + "\n\n" + body + "\n",
	}
	for name, in := range cases {
		d, err := ParseDraft(in)
		if err != nil {
			t.Errorf("%s: err = %v", name, err)
			continue
		}
		if d.Title != title {
			t.Errorf("%s: title = %q", name, d.Title)
		}
		if d.Body != body {
			t.Errorf("%s: body = %q", name, d.Body)
		}
	}
}

func TestParseDraftKeepsTrailingCodeBlock(t *testing.T) {
	body := "## Resumen\nEjemplo:\n\n```sh\ntija pr\n```"
	d, err := ParseDraft("<<<TITLE\nfeat(pr): x\nTITLE>>>\n<<<BODY\n" + body + "\nBODY>>>")
	if err != nil || d.Body != body {
		t.Errorf("body = %q, err = %v", d.Body, err)
	}
}

func TestParseDraftGarbage(t *testing.T) {
	for _, in := range []string{"", "I cannot help with that.", "feat(x): y\nno heading here", "<<<TITLE\n\nTITLE>>>"} {
		if _, err := ParseDraft(in); !errors.Is(err, ErrUnparseable) {
			t.Errorf("ParseDraft(%q) err = %v, want ErrUnparseable", in, err)
		}
	}
}
