package gitx_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func commitFile(t *testing.T, dir, name string, data []byte, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, dir, "add", "-A")
	testutil.Git(t, dir, "commit", "-m", msg)
}

func TestResolveBasePrefersRemote(t *testing.T) {
	dir := testutil.NewRepo(t, "", testutil.WithRemote("", false))
	testutil.Git(t, dir, "checkout", "-b", "dev")
	testutil.Git(t, dir, "push", "origin", "dev")
	commitFile(t, dir, "extra.txt", []byte("x\n"), "local only")
	testutil.Git(t, dir, "checkout", "main")
	ref, ok := newGitx(dir).ResolveBase(context.Background(), "dev")
	if !ok || ref != "refs/remotes/origin/dev" {
		t.Fatalf("got %q %v", ref, ok)
	}
}

func TestResolveBaseLocalOnly(t *testing.T) {
	dir := testutil.NewRepo(t, "")
	testutil.Git(t, dir, "branch", "dev")
	ref, ok := newGitx(dir).ResolveBase(context.Background(), "dev")
	if !ok || ref != "refs/heads/dev" {
		t.Fatalf("got %q %v", ref, ok)
	}
}

func TestResolveBaseMissing(t *testing.T) {
	dir := testutil.NewRepo(t, "")
	if ref, ok := newGitx(dir).ResolveBase(context.Background(), "nope"); ok {
		t.Fatalf("got %q, want not ok", ref)
	}
}

func TestCommitsAndDiffStat(t *testing.T) {
	dir := testutil.NewRepo(t, "", testutil.WithBranch("feat/x"))
	commitFile(t, dir, "a.txt", []byte("1\n2\n3\n"), "feat: add a\n\nbody of a")
	commitFile(t, dir, "img.png", []byte{0x89, 'P', 'N', 'G', 0, 0, 1, 2}, "feat: add image")
	r := newGitx(dir)
	ctx := context.Background()

	commits := r.Commits(ctx, "refs/heads/main")
	if len(commits) != 2 || commits[0].Subject != "feat: add a" || commits[1].Subject != "feat: add image" {
		t.Fatalf("commits = %+v", commits)
	}
	if commits[0].Body != "body of a" || commits[0].SHA == "" {
		t.Fatalf("commit[0] = %+v", commits[0])
	}

	stat := r.DiffStat(ctx, "refs/heads/main")
	want := []gitx.FileStat{{Path: "a.txt", Added: 3}, {Path: "img.png", Binary: true}}
	if !reflect.DeepEqual(stat, want) {
		t.Fatalf("stat = %+v, want %+v", stat, want)
	}

	if d := r.DiffFile(ctx, "refs/heads/main", "a.txt"); d == "" {
		t.Fatal("empty diff")
	}
}
