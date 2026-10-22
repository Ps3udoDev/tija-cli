package engine

import (
	"context"
	"errors"
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
