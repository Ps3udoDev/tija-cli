package manifest

import (
	"reflect"
	"strings"
	"testing"
)

func TestReviewDefaults(t *testing.T) {
	d := DefaultReview()
	if d.FailOn != "high" || d.MaxLines != 3000 || !reflect.DeepEqual(d.Rules, []string{".tija/rules/architecture/*.md"}) || len(d.Exclude) != 10 {
		t.Fatalf("defaults: %+v", d)
	}
	r := (Review{FailOn: "low"}).WithDefaults()
	d.FailOn = "low"
	if !reflect.DeepEqual(r, d) {
		t.Fatalf("inherited: %+v", r)
	}
	empty := (Review{Rules: []string{}, Exclude: []string{}}).WithDefaults()
	if len(empty.Rules) != 0 || len(empty.Exclude) != 0 {
		t.Fatal("explicit empty lists must be preserved")
	}
}

func TestValidateReviewFailOn(t *testing.T) {
	m := FromDetection(frontendDetection(t), nil, "main")
	for _, tc := range []struct {
		value string
		max   int
		want  string
	}{{"urgent", 0, "review.failOn"}, {"none", 0, ""}, {"high", -1, "review.maxLines"}} {
		m.Review = Review{FailOn: tc.value, MaxLines: tc.max}
		errs := m.Validate()
		if tc.want == "" && len(errs) > 0 {
			t.Fatal(errs)
		}
		if tc.want != "" && (len(errs) == 0 || !strings.Contains(errs[0].Error(), tc.want)) {
			t.Fatalf("%+v: %v", tc, errs)
		}
	}
}

func TestFromDetectionHasReview(t *testing.T) {
	if !reflect.DeepEqual(FromDetection(frontendDetection(t), nil, "main").Review, DefaultReview()) {
		t.Fatal("missing default review")
	}
}
