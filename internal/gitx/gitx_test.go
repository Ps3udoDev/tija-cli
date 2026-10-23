package gitx_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

// fakeRun answers every command with the given exit code and stderr.
func fakeRun(code int, stderr string) sys.Runner {
	return func(context.Context, string, string, ...string) (string, string, int, error) {
		return "", stderr, code, nil
	}
}

func TestFindRootDubiousOwnership(t *testing.T) {
	msg := "fatal: detected dubious ownership in repository at 'C:/x'"
	_, ok, err := gitx.FindRoot(context.Background(), fakeRun(128, msg+"\n"), "C:/x")
	if err == nil || ok {
		t.Fatalf("ok=%v err=%v, want an error", ok, err)
	}
	if !strings.Contains(err.Error(), "detected dubious ownership") {
		t.Errorf("err = %q, want git's message", err)
	}
}

func TestFindRootNotARepoMessage(t *testing.T) {
	_, ok, err := gitx.FindRoot(context.Background(),
		fakeRun(128, "fatal: not a git repository (or any of the parent directories): .git\n"), "C:/x")
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v, want false,nil", ok, err)
	}
}

func TestCurrentBranchFailureIsAnError(t *testing.T) {
	r := gitx.Repo{Dir: "C:/x", Run: fakeRun(128, "fatal: detected dubious ownership in repository at 'C:/x'")}
	_, detached, err := r.CurrentBranch(context.Background())
	if err == nil || detached {
		t.Fatalf("detached=%v err=%v, want an error", detached, err)
	}
	r.Run = fakeRun(1, "")
	if _, detached, err = r.CurrentBranch(context.Background()); err != nil || !detached {
		t.Fatalf("exit 1: detached=%v err=%v, want detached", detached, err)
	}
}

func TestHeadAndStatusPaths(t *testing.T) {
	// Without commits
	dirEmpty := testutil.NewRepo(t, "", testutil.WithoutCommit())
	rEmpty := newGitx(dirEmpty)
	sha, ok := rEmpty.Head(context.Background())
	if ok || sha != "" {
		t.Fatalf("empty repo Head: got ok=%v sha=%q, want false, \"\"", ok, sha)
	}

	// With commits
	dir := testutil.NewRepo(t, "")
	r := newGitx(dir)
	sha, ok = r.Head(context.Background())
	if !ok || len(sha) != 40 {
		t.Fatalf("Head got ok=%v sha=%q (want 40 hex chars)", ok, sha)
	}

	// Create a new file with spaces and unicode: "a b/ñ.txt"
	sub := filepath.Join(dir, "a b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	newFile := filepath.Join(sub, "ñ.txt")
	if err := os.WriteFile(newFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Modify an existing file: README.md
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths := r.StatusPaths(context.Background())
	// Should be sorted and use forward slashes
	want := []string{"README.md", "a b/ñ.txt"}
	if len(paths) != len(want) {
		t.Fatalf("StatusPaths = %v, want %v", paths, want)
	}
	for i, p := range want {
		if paths[i] != p {
			t.Fatalf("StatusPaths[%d] = %q, want %q", i, paths[i], p)
		}
	}

	// Test rename: git mv README.md DOCS.md
	testutil.Git(t, dir, "mv", "README.md", "DOCS.md")
	paths = r.StatusPaths(context.Background())
	want = []string{"DOCS.md", "a b/ñ.txt"}
	if len(paths) != len(want) {
		t.Fatalf("StatusPaths after rename = %v, want %v", paths, want)
	}
	for i, p := range want {
		if paths[i] != p {
			t.Fatalf("StatusPaths after rename [%d] = %q, want %q", i, paths[i], p)
		}
	}
}
