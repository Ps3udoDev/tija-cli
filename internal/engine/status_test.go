package engine

import (
	"context"
	"errors"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"slices"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/branch"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func status(t *testing.T, dir string) StatusReport {
	t.Helper()
	r, err := Status(context.Background(), dir, fakeEnv())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestStatusConforming(t *testing.T) {
	repo := testutil.NewRepo(t, "", testutil.WithBranch("feat/shared-imputation-form"))
	testutil.Git(t, repo, "commit", "--allow-empty", "-m", "work")
	r := status(t, repo)
	if r.Branch.State != branch.Conforming {
		t.Errorf("state = %s", r.Branch.State)
	}
	if r.Target != "main" || !r.TargetFound || r.Ahead != 1 || r.Behind != 0 || !r.HasCommits {
		t.Errorf("report = %+v", r)
	}
	if r.ExitCode(true) != 0 || r.ExitCode(false) != 0 {
		t.Error("conforming must exit 0")
	}
}

func TestStatusIssueBranch(t *testing.T) {
	repo := testutil.NewRepo(t, "", testutil.WithBranch("101-fixclaims-fix-integration-issues"))
	r := status(t, repo)
	if r.Branch.State != branch.Nonconforming || r.Branch.Suggested != "fix/101-claims-integration-issues" {
		t.Errorf("verdict = %+v", r.Branch)
	}
	if r.ExitCode(false) != 0 || r.ExitCode(true) != 1 {
		t.Errorf("exit = %d/%d", r.ExitCode(false), r.ExitCode(true))
	}
}

func TestStatusHuskyWarning(t *testing.T) {
	repo := initRepo(t, "react-vite")
	testutil.Git(t, repo, "checkout", "-b", "changes")
	r := status(t, repo)
	if r.Branch.State != branch.Nonconforming || !r.HuskyWarning {
		t.Errorf("report = %+v", r)
	}
}

func TestStatusNoRemoteNoCommits(t *testing.T) {
	repo := testutil.NewRepo(t, "", testutil.WithoutCommit())
	r := status(t, repo)
	if r.HasCommits || r.HasRemote || r.TargetFound || r.Published || r.Ahead != 0 || r.Behind != 0 {
		t.Errorf("report = %+v", r)
	}
}

func TestStatusBaseBranch(t *testing.T) {
	repo := testutil.NewRepo(t, "")
	r := status(t, repo)
	if r.Branch.State != branch.Base || r.Branch.Name != "main" {
		t.Errorf("verdict = %+v", r.Branch)
	}
}

func TestStatusDetachedAndWorkTree(t *testing.T) {
	repo := testutil.NewRepo(t, "", testutil.WithRemote("", true))
	putFile(t, repo, "new.txt", []byte("x"))
	testutil.Git(t, repo, "checkout", "--detach")
	r := status(t, repo)
	if r.Branch.State != branch.Detached || r.Published || !r.HasRemote || r.Untracked != 1 {
		t.Errorf("report = %+v", r)
	}
	if r.ExitCode(true) != 1 {
		t.Error("detached must fail in strict mode")
	}
}

func TestStatusOutsideRepo(t *testing.T) {
	_, err := Status(context.Background(), t.TempDir(), fakeEnv())
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("err = %v", err)
	}
}

// manifestRepo returns an initialized react-vite repo whose manifest was
// edited by edit.
func manifestRepo(t *testing.T, edit func(m *manifest.Manifest)) string {
	t.Helper()
	repo := initRepo(t, "react-vite")
	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	edit(&m)
	if err := manifest.Save(repo, m); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestStatusUsesManifest(t *testing.T) {
	repo := manifestRepo(t, func(m *manifest.Manifest) {
		m.Git.BranchPrefixes = []string{"spike"}
		m.PullRequest.Target = "dev"
	})
	testutil.Git(t, repo, "branch", "dev")
	testutil.Git(t, repo, "checkout", "-b", "spike/try-x")
	r := status(t, repo)
	if r.Branch.State != branch.Conforming || r.Target != "dev" || !r.TargetFound {
		t.Errorf("report = %+v", r)
	}
	if r.HuskyWarning {
		t.Error("husky warning on conforming branch")
	}
}

func TestStatusInvalidManifestFallsBack(t *testing.T) {
	cases := map[string]func(t *testing.T) string{
		"fails validation": func(t *testing.T) string {
			return manifestRepo(t, func(m *manifest.Manifest) {
				m.Git.BranchPrefixes = []string{"spike"}
				m.Project.Type = "bogus"
			})
		},
		"does not parse": func(t *testing.T) string {
			repo := initRepo(t, "react-vite")
			putFile(t, repo, ".tija/manifest.yaml", []byte(":::"))
			return repo
		},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			repo := mk(t)
			testutil.Git(t, repo, "checkout", "-b", "spike/try-x")
			r := status(t, repo)
			if r.Branch.State != branch.Nonconforming {
				t.Errorf("state = %s, want nonconforming", r.Branch.State)
			}
			if want := defaultTarget(context.Background(), gitxRepo(repo)); r.Target != want || want == "" {
				t.Errorf("target = %q, want %q", r.Target, want)
			}
		})
	}
}

func TestStatusPublishedAndWorkTree(t *testing.T) {
	repo := testutil.NewRepo(t, "", testutil.WithRemote("", true), testutil.WithBranch("feat/x"))
	if r := status(t, repo); r.Published {
		t.Error("published before push")
	}
	testutil.Git(t, repo, "push", "origin", "feat/x")
	putFile(t, repo, "README.md", []byte("changed\n"))
	putFile(t, repo, "staged.txt", []byte("s"))
	testutil.Git(t, repo, "add", "staged.txt")
	r := status(t, repo)
	if !r.Published || r.Staged != 1 || r.Unstaged != 1 {
		t.Errorf("report = %+v", r)
	}
}

func gitxRepo(dir string) gitx.Repo { return gitx.Repo{Dir: dir, Run: fakeEnv().Run} }


func TestBaseBranchesConfigurable(t *testing.T) {
	repo := testutil.NewRepo(t, "", testutil.WithBranch("trunk"))
	data := []byte("version: 1\nproject:\n  name: x\n  type: unknown\ngit:\n  baseBranches: [\"trunk\"]\npullRequest:\n  target: main\n")
	putFile(t, repo, ".tija/manifest.yaml", data)

	r := status(t, repo)
	if r.Branch.State != branch.Base {
		t.Errorf("r.Branch.State = %q, want Base", r.Branch.State)
	}
}

func TestRemoteDefaultIsBase(t *testing.T) {
	repo := testutil.NewRepo(t, "", testutil.WithBranch("production"))
	env := fakeEnv()
	realRun := sys.Default().Run
	env.Run = func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
		if name == "git" && len(args) > 0 && args[0] == "symbolic-ref" && slices.Contains(args, "refs/remotes/origin/HEAD") {
			return "origin/production\n", "", 0, nil
		}
		return realRun(ctx, dir, name, args...)
	}
	r, err := Status(context.Background(), repo, env)
	if err != nil {
		t.Fatal(err)
	}
	if r.Branch.State != branch.Base {
		t.Errorf("r.Branch.State = %q, want Base", r.Branch.State)
	}
}

func TestStatusNoDeleteHintIfPublishedBase(t *testing.T) {
	repo := testutil.NewRepo(t, "", testutil.WithBranch("desarrollo"))
	testutil.Git(t, repo, "commit", "--allow-empty", "-m", "init")
	r := status(t, repo)
	if r.Branch.State != branch.Base {
		t.Errorf("r.Branch.State = %q, want Base", r.Branch.State)
	}
	if r.Branch.Suggested != "" {
		t.Errorf("r.Branch.Suggested = %q, want empty", r.Branch.Suggested)
	}
}
