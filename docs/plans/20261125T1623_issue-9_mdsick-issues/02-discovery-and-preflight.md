# Issue Discovery and Preflight (02) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the `internal/issues` package: repo-root walk-up (`.jj` before `.git` per level, overridable), `docs/issues` + `docs/issues/closed` presence checks, numeric-filename listing in reverse natural order, and fail-hard duplicate detection — all without the CLI (the cobra flag is wired in sub-plan 04).

**Architecture:** One package, three exported entry points: `FindRepo`, `List`, and the `Row` type. Filename parsing is strict: a file in either directory must be `NN.md` (digits only) or the listing fails. Duplicates (`NN.md` on both sides) fail hard before anything renders. Ordering is reverse natural sort by base name via `internal/natural` so higher issue numbers come first.

**Tech Stack:** Go 1.26 stdlib (`os`, `path/filepath`, `regexp`, `slices`), `internal/natural.Compare` from sub-plan 01.

---

## Current state (read this first)

- Uses `internal/natural.Compare` — sub-plan 01 must be complete (concurrent-batch note: 02 can start in parallel, do NOT wire its import until 01 has landed the `internal/natural` package).
- There is no `.git` in this repo — tests must build fixture repos under `t.TempDir()` (directories containing a `.jj` dir and/or a `.git` dir as plain folders).
- Contract (from docs/issues/9.md):
  - Walk up from cwd until a directory contains `.jj` (preferred) or `.git`. None found → explicit error.
  - With an override, only the named marker counts.
  - Both `docs/issues` and `docs/issues/closed` must exist under the root; error otherwise.
  - Every `.md` in those two dirs must be all-digits `NN.md`; anything else is an explicit listing error.
  - The same `NN` in both dirs is fatal ("duplicate issue number").

---

### Task 1: `FindRepo`

**Files:**
- Create: `internal/issues/findrepo.go`
- Test: `internal/issues/findrepo_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
		full := filepath.Join(root, d)
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestFindRepo(t *testing.T) {
	t.Run("jj preferred at same level", func(t *testing.T) {
		repo := makeTree(t, ".jj", ".git")
		got, err := findRepoFrom(makeTreeWithRoot(t, repo))
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Clean(got) != filepath.Clean(repo) {
			t.Fatalf("got %s exp %s", got, repo)
		}
	})

	t.Run("git used when no jj above", func(t *testing.T) {
		repo := makeTree(t, ".git")
		got, err := findRepoFrom(makeTreeWithRoot(t, repo))
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Clean(got) != filepath.Clean(repo) {
			t.Fatalf("got %s exp %s", got, repo)
		}
	})

	t.Run("walks up through nested dir", func(t *testing.T) {
		repo := makeTree(t, ".jj", "docs", "issues")
		sub := filepath.Join(repo, "docs", "issues")
		got, err := findRepoFrom(treeWithCwd(t, sub))
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Clean(got) != filepath.Clean(repo) {
			t.Fatalf("got %s exp %s", got, repo)
		}
	})

	t.Run("not found is an explicit error", func(t *testing.T) {
		_, err := findRepoFrom(treeWithCwd(t, t.TempDir()))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "repo root") {
			t.Fatalf("error must mention repo root:\n%v", err)
		}
	})

	t.Run("override git skips jj markers", func(t *testing.T) {
		outer := makeTree(t, ".jj", "repo", ".git")
		repo := filepath.Join(outer, "repo")
		got, err := findRepoOverride(treeWithCwd(t, filepath.Join(repo, "docs")), "git")
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Clean(got) != filepath.Clean(repo) {
			t.Fatalf("got %s exp %s", got, repo)
		}
	})
}
```

Helpers `makeTreeWithRoot` and `treeWithCwd` both: chdir the test process into the given directory and restore with `t.Cleanup`.

```go
func treeWithCwd(t *testing.T, dir string) string {
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
```

`makeTreeWithRoot(t, repo)` is the same helper; keep one `chdirTo` and use it everywhere (rewrite the call sites above accordingly — the plan text uses both names, unify on `chdirTo`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/issues/ -v`
Expected: FAIL (package exists but `findRepoFrom` undefined).

- [ ] **Step 3: Write minimal implementation**

```go
package issues

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindRepo walks up from the process cwd until a directory contains a
// VCS marker. With "auto", .jj wins over .git at each level; an
// explicit "jj" or "git" restricts detection to that marker.
func FindRepo(vcs string) (string, error) {
	return findRepoOverride(".", vcs)
}

func findRepoOverride(start, vcs string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		marker := ""
		switch vcs {
		case "auto":
			switch {
			case exists(filepath.Join(dir, ".jj")):
				marker = ".jj"
			case exists(filepath.Join(dir, ".git")):
				marker = ".git"
			}
		case "jj":
			if exists(filepath.Join(dir, ".jj")) {
				marker = ".jj"
			}
		case "git":
			if exists(filepath.Join(dir, ".git")) {
				marker = ".git"
			}
		default:
			return "", fmt.Errorf("issues: unknown --vcs value %q", vcs)
		}
		if marker != "" {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("issues: no repo root (.jj or .git) found from %s", start)
		}
		dir = parent
	}
}

func findRepoFrom(start string) (string, error) {
	return findRepoOverride(start, "auto")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/issues/`
Expected: PASS

- [ ] **Step 5: Commit with caveman-commit**

Message: `feat: issue repo discovery`

---

### Task 2: `List` — globs, strict filenames, reverse order, duplicates

**Files:**
- Modify: `internal/issues/findrepo.go` — new file better: create `internal/issues/list.go`
- Test: `internal/issues/list_test.go`

- [ ] **Step 1: Write the failing test**

```go
package issues

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedIssues(t *testing.T, open, closed []string) string {
	root := chdirTo(t, makeTree(t, "docs", "issues", "closed"))
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

	t.Run("non numeric filename fails hard", func(t *testing.T) {
		seedIssues(t, []string{"7.md", "notes.md"}, nil)
		_, _, err := List(".")
		if err == nil || !strings.Contains(err.Error(), "notes.md") {
			t.Fatalf("expected numeric name error, got %v", err)
		}
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
		got = append(got, itoa(r.Number))
	}
	if strings.Join(got, " ") != want {
		t.Fatalf("order:\ngot %v\nexp %s", got, want)
	}
}
```

(Define `itoa` with `strconv.Itoa`; add that import.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/issues/ -run TestList -v`
Expected: FAIL (`List`, `Row` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/issues/list.go`:

```go
package issues

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/malikbenkirane/mdsick/internal/natural"
)

// Row is one issue file on one side of the tree.
type Row struct {
	Number int    // parsed NN
	Base   string // "NN.md"
	Side   string // "open" or "closed"
	Head   string // first level-1 heading text, "" when absent
}

func List(root string) (open, closed []Row, err error) {
	issuesDir := filepath.Join(root, "docs", "issues")
	closedDir := filepath.Join(issuesDir, "closed")
	if !exists(issuesDir) || !exists(closedDir) {
		return nil, nil, fmt.Errorf("issues: docs/issues and docs/issues/closed must exist under the repo root")
	}

	openNames, err := listNames(issuesDir, "open")
	if err != nil {
		return nil, nil, err
	}
	closedNames, err := listNames(closedDir, "closed")
	if err != nil {
		return nil, nil, err
	}

	seen := make(map[int]bool)
	for _, row := range append(slices.Clone(openNames), closedNames...) {
		if seen[row.Number] {
			return nil, nil, fmt.Errorf("issues: duplicate issue number %d (present on both sides of docs/issues): %s", row.Number, row.Base)
		}
		seen[row.Number] = true
	}

	return openNames, closedNames, nil
}

func listNames(dir, side string) ([]Row, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var rows []Row
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") || e.IsDir() {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".md")
		n, convErr := strconv.Atoi(base)
		if convErr != nil || !isDigitsOnly(base) {
			return nil, fmt.Errorf("issues: unexpected issue filename %q in %s (want NN.md)", e.Name(), side)
		}
		rows = append(rows, Row{Number: n, Base: e.Name(), Side: side})
	}
	slices.SortStableFunc(rows, func(a, b Row) int {
		return natural.Compare(b.Base, a.Base)
	})
	return rows, nil
}

func isDigitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/issues/ -v`
Expected: PASS (both tasks).

- [ ] **Step 5: Commit with caveman-commit**

Message: `feat: list and validate issue files`
