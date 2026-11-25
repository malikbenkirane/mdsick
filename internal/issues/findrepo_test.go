package issues

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeTree(t *testing.T, dirs ...string) string {
	root := t.TempDir()
	for _, d := range dirs {
		full := filepath.Join(root, filepath.FromSlash(d))
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func chdirTo(t *testing.T, dir string) string {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatal(err)
		}
	})
	return dir
}

func TestFindRepo(t *testing.T) {
	t.Run("jj preferred at same level", func(t *testing.T) {
		repo := makeTree(t, ".jj", ".git")
		got, err := findRepoFrom(chdirTo(t, repo))
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Clean(got) != filepath.Clean(repo) {
			t.Fatalf("got %s exp %s", got, repo)
		}
	})

	t.Run("git used when no jj above", func(t *testing.T) {
		repo := makeTree(t, ".git")
		got, err := findRepoFrom(chdirTo(t, repo))
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Clean(got) != filepath.Clean(repo) {
			t.Fatalf("got %s exp %s", got, repo)
		}
	})

	t.Run("walks up through nested dir", func(t *testing.T) {
		repo := makeTree(t, ".jj", "docs/issues")
		sub := filepath.Join(repo, "docs", "issues")
		got, err := findRepoFrom(chdirTo(t, sub))
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Clean(got) != filepath.Clean(repo) {
			t.Fatalf("got %s exp %s", got, repo)
		}
	})

	t.Run("not found is an explicit error", func(t *testing.T) {
		_, err := findRepoFrom(chdirTo(t, t.TempDir()))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "repo root") {
			t.Fatalf("error must mention repo root:\n%v", err)
		}
	})

	t.Run("override git skips jj markers", func(t *testing.T) {
		outer := makeTree(t, ".jj", "repo/.git", "repo/docs")
		repo := filepath.Join(outer, "repo")
		got, err := findRepoOverride(chdirTo(t, filepath.Join(repo, "docs")), "git")
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Clean(got) != filepath.Clean(repo) {
			t.Fatalf("got %s exp %s", got, repo)
		}
	})
}
