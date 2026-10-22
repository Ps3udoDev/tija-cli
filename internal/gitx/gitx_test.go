package gitx_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func newGitx(dir string) gitx.Repo {
	return gitx.Repo{Dir: dir, Run: sys.Default().Run}
}

func realPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", p, err)
	}
	return filepath.Clean(r)
}

func TestRepoInPathWithSpaces(t *testing.T) {
	root := testutil.NewRepo(t, "")
	if filepath.Base(filepath.Dir(root)) != "azure 2" {
		t.Fatalf("repo not under a folder with spaces: %s", root)
	}
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	got, ok, err := gitx.FindRoot(context.Background(), sys.Default().Run, src)
	if err != nil || !ok {
		t.Fatalf("FindRoot ok=%v err=%v", ok, err)
	}
	if realPath(t, got) != realPath(t, root) {
		t.Fatalf("FindRoot = %q, want %q", got, root)
	}
}

func TestFindRootOutsideRepo(t *testing.T) {
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(t.TempDir()))
	testutil.NewRepo(t, "") // applies hermetic git environment
	_, ok, err := gitx.FindRoot(context.Background(), sys.Default().Run, t.TempDir())
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v, want false,nil", ok, err)
	}
}

func TestCurrentBranchAndDetached(t *testing.T) {
	dir := testutil.NewRepo(t, "", testutil.WithBranch("feat/x"))
	r := newGitx(dir)
	name, detached, err := r.CurrentBranch(context.Background())
	if err != nil || name != "feat/x" || detached {
		t.Fatalf("got %q,%v,%v", name, detached, err)
	}
	testutil.Git(t, dir, "checkout", "--detach")
	_, detached, err = r.CurrentBranch(context.Background())
	if err != nil || !detached {
		t.Fatalf("detached=%v err=%v", detached, err)
	}
}

func TestNoCommits(t *testing.T) {
	dir := testutil.NewRepo(t, "", testutil.WithoutCommit())
	r := newGitx(dir)
	if r.HasCommits(context.Background()) {
		t.Fatal("HasCommits = true, want false")
	}
	name, detached, err := r.CurrentBranch(context.Background())
	if err != nil || name != "main" || detached {
		t.Fatalf("got %q,%v,%v", name, detached, err)
	}
}

func TestAheadBehind(t *testing.T) {
	dir := testutil.NewRepo(t, "", testutil.WithBranch("feat/x"))
	for _, f := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
		testutil.Git(t, dir, "add", f)
		testutil.Git(t, dir, "commit", "-m", "add "+f)
	}
	r := newGitx(dir)
	ahead, behind, found := r.AheadBehind(context.Background(), "main")
	if ahead != 2 || behind != 0 || !found {
		t.Fatalf("got %d,%d,%v", ahead, behind, found)
	}
	if _, _, found := r.AheadBehind(context.Background(), "dev"); found {
		t.Fatal("dev should not be found")
	}
}

func TestWorkTreeCounts(t *testing.T) {
	dir := testutil.NewRepo(t, "")
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a-tracked.txt", "v1")
	testutil.Git(t, dir, "add", "a-tracked.txt")
	testutil.Git(t, dir, "commit", "-m", "tracked")
	write("a-tracked.txt", "v2") // unstaged modification
	write("staged.txt", "s")
	testutil.Git(t, dir, "add", "staged.txt")
	write("u1.txt", "1")
	write("u2.txt", "2")
	s, u, n := newGitx(dir).WorkTree(context.Background())
	if s != 1 || u != 1 || n != 2 {
		t.Fatalf("got %d,%d,%d want 1,1,2", s, u, n)
	}
}

func TestDefaultRemoteBranch(t *testing.T) {
	ctx := context.Background()
	dir := testutil.NewRepo(t, "", testutil.WithRemote("", true))
	r := newGitx(dir)
	if got := r.DefaultRemoteBranch(ctx); got != "main" {
		t.Fatalf("DefaultRemoteBranch = %q, want main", got)
	}
	if r.RemoteURL(ctx) == "" {
		t.Fatal("RemoteURL empty")
	}
	if !r.IsPublished(ctx, "main") || r.IsPublished(ctx, "nope") {
		t.Fatal("IsPublished mismatch")
	}
	if got := newGitx(testutil.NewRepo(t, "")).DefaultRemoteBranch(ctx); got != "" {
		t.Fatalf("no remote: got %q, want empty", got)
	}
}

func TestLocalBranches(t *testing.T) {
	dir := testutil.NewRepo(t, "", testutil.WithBranch("feat/x"))
	got := newGitx(dir).LocalBranches(context.Background())
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}
