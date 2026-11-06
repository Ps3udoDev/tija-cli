package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func TestApplyCreatePRRejectsChangedSnapshot(t *testing.T) {
	for _, change := range []string{"head", "branch", "target"} {
		t.Run(change, func(t *testing.T) {
			root, bare := remotePRRepo(t)
			gh := &ghFake{create: "https://github.com/example/test/pull/42"}
			env := withGH(sys.Default(), gh)
			req := CreateRequest{Prepare: PRRequest{Agent: "none", NoCheck: true, EvidenceRefs: []EvidenceRef{{Source: "qapture", Project: EvidenceProject{ID: "synthetic"}, Notes: "Evidence for planned commit"}}}}
			p := planCreate(t, root, env, req)
			switch change {
			case "head":
				commitFile(t, root, "another.ts", "export const next = 1\n", "feat(test): add another commit")
			case "branch":
				testutil.Git(t, root, "checkout", "-b", "feat/other-branch")
			case "target":
				m, err := manifest.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				m.PullRequest.Target = "another-target"
				if err := manifest.Save(root, m); err != nil {
					t.Fatal(err)
				}
			}
			_, err := ApplyCreatePR(context.Background(), root, env, p, req)
			if err == nil || !strings.Contains(err.Error(), "repository changed") {
				t.Fatalf("err=%v", err)
			}
			if len(gh.called("create")) != 0 || len(gh.called("edit")) != 0 {
				t.Fatal("published a stale plan")
			}
			if branches := testutil.Git(t, bare, "branch", "--list"); strings.Contains(branches, "feat/") {
				t.Fatalf("pushed before rejecting stale plan: %s", branches)
			}
		})
	}
}
