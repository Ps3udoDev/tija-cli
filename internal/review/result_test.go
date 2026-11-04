package review

import (
	"encoding/json"
	"testing"
)

func TestFinalizeExit(t *testing.T) {
	for _, c := range []struct {
		threshold    string
		failed, want int
	}{{"high", 0, 1}, {"critical", 0, 0}, {"none", 0, 0}, {"none", 1, 2}} {
		r := Result{FailOn: c.threshold, Findings: []Finding{{Severity: High}}, Chunks: ChunkSummary{Failed: c.failed}}
		r.Finalize()
		if r.Exit != c.want || r.Summary[High] != 1 || len(r.Summary) != 4 {
			t.Fatalf("%#v", r)
		}
	}
	r := Result{}
	r.Finalize()
	b, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(b, &raw)
	for _, key := range []string{"findings", "discarded"} {
		if string(raw[key]) != "[]" {
			t.Errorf("%s=%s", key, raw[key])
		}
	}
}
