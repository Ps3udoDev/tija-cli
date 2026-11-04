package review

import (
	"reflect"
	"testing"
)

func TestAtLeast(t *testing.T) {
	for _, c := range []struct {
		s         Severity
		threshold string
		want      bool
	}{{High, "high", true}, {Medium, "high", false}, {Low, "low", true}, {Critical, "none", false}, {Critical, "urgent", false}} {
		if AtLeast(c.s, c.threshold) != c.want {
			t.Errorf("AtLeast(%s,%s)", c.s, c.threshold)
		}
	}
	if s, ok := ParseSeverity("hIgH"); !ok || s != High {
		t.Fatal("case-insensitive severity")
	}
}
func TestParseResponseFencedAndProse(t *testing.T) {
	raw := `{"findings":[{"file":"a.ts","line":10,"severity":"High","rule":"bug","evidence":"code"}]}`
	a, e := ParseResponse(raw)
	if e != nil {
		t.Fatal(e)
	}
	b, e := ParseResponse("Aquí va:\n```json\n" + raw + "\n```\nDone.")
	if e != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("%#v %v", b, e)
	}
	f, e := ParseResponse(`{"findings":[]}`)
	if e != nil || f == nil || len(f) != 0 {
		t.Fatalf("empty: %#v %v", f, e)
	}
}
func TestParseResponseInvalid(t *testing.T) {
	for _, s := range []string{"hello", "{}", `{"findings":{}}`, `{"findings":null}`, `{"findings":[null]}`, `{"findings":[{"line":"ten"}]}`} {
		if _, e := ParseResponse(s); e == nil {
			t.Errorf("accepted %s", s)
		}
	}
}
