package gitx_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func TestMergeBaseAndWorkingDiff(t *testing.T) {
	root := testutil.NewRepo(t, "", testutil.WithBranch("feature"))
	ctx := context.Background()
	repo := newGitx(root)
	base, ok := repo.MergeBase(ctx, "main")
	if !ok || base == "" {
		t.Fatal(base, ok)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# test\ncommitted\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, root, "add", "README.md")
	testutil.Git(t, root, "commit", "-m", "change")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# test\ncommitted\nuncommitted\n"), 0644); err != nil {
		t.Fatal(err)
	}
	diff := repo.WorkingDiff(ctx, base)
	if !strings.Contains(diff, "+committed") || !strings.Contains(diff, "+uncommitted") || strings.Contains(diff, "\r\n") {
		t.Fatal(diff)
	}
	if _, ok := repo.MergeBase(ctx, "does-not-exist"); ok {
		t.Fatal("invalid ref")
	}
}
func TestUntracked(t *testing.T) {
	root := testutil.NewRepo(t, "")
	for name, text := range map[string]string{".gitignore": "ignored.txt\n", "ignored.txt": "x", "new file.txt": "x"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got := newGitx(root).Untracked(context.Background())
	if !reflect.DeepEqual(got, []string{".gitignore", "new file.txt"}) {
		t.Fatal(got)
	}
}
func TestLineCountAt(t *testing.T) {
	root := testutil.NewRepo(t, "")
	for name, text := range map[string]string{"three.txt": "a\nb\nc\n", "partial.txt": "a\nb\nc", "empty.txt": ""} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	testutil.Git(t, root, "add", ".")
	testutil.Git(t, root, "commit", "-m", "files")
	repo := newGitx(root)
	for _, tc := range []struct {
		name   string
		count  int
		exists bool
	}{{"three.txt", 3, true}, {"partial.txt", 3, true}, {"empty.txt", 0, true}, {"new.txt", 0, false}} {
		count, ok := repo.LineCountAt(context.Background(), "HEAD", tc.name)
		if count != tc.count || ok != tc.exists {
			t.Fatalf("%s: %d,%v", tc.name, count, ok)
		}
	}
}
