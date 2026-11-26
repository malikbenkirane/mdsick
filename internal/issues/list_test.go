package issues

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func seedIssues(t *testing.T, open, closed []string) string {
	root := chdirTo(t, makeTree(t, "docs/issues", "docs/issues/closed"))
	write := func(dir string, names []string) {
		for _, n := range names {
			if err := os.WriteFile(filepath.Join(root, dir, n), []byte("# x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	write(filepath.Join("docs", "issues"), open)
	write(filepath.Join("docs", "issues", "closed"), closed)
	return root
}

func TestList(t *testing.T) {
	t.Run("reverse natural order per side", func(t *testing.T) {
		seedIssues(t, []string{"3.md", "21.md", "2.md"}, []string{"10.md", "9.md"})
		open, closed, err := List(".")
		if err != nil {
			t.Fatal(err)
		}
		assertOrder(t, open, "21 3 2")
		assertOrder(t, closed, "10 9")
	})

	t.Run("empty closed dir is fine", func(t *testing.T) {
		seedIssues(t, []string{"7.md"}, nil)
		open, closed, err := List(".")
		if err != nil {
			t.Fatal(err)
		}
		assertOrder(t, open, "7")
		if len(closed) != 0 {
			t.Fatalf("expected empty closed, got %v", closed)
		}
	})

	t.Run("non numeric filenames are skipped", func(t *testing.T) {
		seedIssues(t, []string{"7.md", "28-original-grill-me.md"}, []string{"9.md", "notes.md", "9-original.md"})
		open, closed, err := List(".")
		if err != nil {
			t.Fatal(err)
		}
		assertOrder(t, open, "7")
		assertOrder(t, closed, "9")
	})

	t.Run("duplicate number in both dirs fails hard", func(t *testing.T) {
		seedIssues(t, []string{"7.md", "9.md"}, []string{"9.md"})
		_, _, err := List(".")
		if err == nil || !strings.Contains(err.Error(), "duplicate issue number 9") {
			t.Fatalf("expected duplicate error, got %v", err)
		}
	})

	t.Run("missing directories fail hard", func(t *testing.T) {
		chdirTo(t, t.TempDir())
		_, _, err := List(".")
		if err == nil || !strings.Contains(err.Error(), "docs/issues") {
			t.Fatalf("expected dirs error, got %v", err)
		}
	})
}

func assertOrder(t *testing.T, rows []Row, want string) {
	t.Helper()
	var got []string
	for _, r := range rows {
		got = append(got, strconv.Itoa(r.Number))
	}
	if strings.Join(got, " ") != want {
		t.Fatalf("order:\ngot %v\nexp %s", got, want)
	}
}
