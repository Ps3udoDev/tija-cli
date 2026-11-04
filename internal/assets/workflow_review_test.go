package assets_test

import (
	"reflect"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/assets"
	"github.com/Ps3udoDev/tija-cli/internal/workflows"
	"gopkg.in/yaml.v3"
)

func TestPrePRHasReview(t *testing.T) {
	var wf workflows.Workflow
	if err := yaml.Unmarshal(assets.Files()[assets.PrePRWorkflowPath], &wf); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range wf.Steps {
		got = append(got, s.Run)
	}
	if !reflect.DeepEqual(got, []string{"clean comments --write", "check", "review", "pr prepare"}) {
		t.Fatal(got)
	}
	if errs := workflows.Validate(wf); len(errs) > 0 {
		t.Fatal(errs)
	}
}
