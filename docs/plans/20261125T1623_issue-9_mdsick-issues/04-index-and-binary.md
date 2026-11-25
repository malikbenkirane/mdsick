# Index Writer and Binary (04) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `cmd/mdsick-issues`: a cobra binary that refuses to leave the repo layout, populates `issues.Index`, renders flat issue pages through the shared workspace, writes a single `index.html` with two fixed sections and a plain `## Warnings` block, and opens it. `cmd/mdsick` is untouched except for consuming the shared `DocTitle` helper moved in Task 1.

**Architecture:** The binary is thin assembly over sub-plans 01–03. The index writer lives in the binary's package (it is presentation for this binary only). Titles come from positional args (joined) with the default `lastTwoComponents(root) + " Issues"`. Empty sections render their fixed heading plus `No open issues` / `No closed issues`. End-to-end tests drive the exported functions over a generated fixture repo and assert with `golang.org/x/net/html`-free stdlib `encoding/html`? No — stdlib is `html` (tokenizer is missing); use `golang.org/x/net/html` added to go.mod, OR assert invariants with `strings`. This plan uses `golang.org/x/net/html` via `go get`.

**Tech Stack:** Go 1.26, cobra, goldmark, internal `issues`/`markdown`/`workspace` packages, `golang.org/x/net/html` (test only), Jujutsu via core-commands.

---

## Current state (read this before Task 3)

- Dependencies: sub-plans 01, 02, 03 all landed.
- `docTitle` lives in `cmd/mdsick/root.go` (a second plain goldmark instance `titleMarkdown` parses the raw source and reads the first level-1 heading). Move it behind `internal/markdown.DocTitle(source []byte) string` — `cmd/mdsick` keeps a one-line call; issues binary uses it to fill `Row.Head`.
- Browser opener is macOS `open`, same as `cmd/mdsick` (`exec.Command("open", ...)`). No cross-platform guard is introduced here either.
- The `Row.Head` field added in sub-plan 02 gets populated here.

---

### Task 1: Move `docTitle` into `internal/markdown`

**Files:**
- Create: `internal/markdown/title.go`
- Modify: `cmd/mdsick/root.go` (delete `docTitle`, `titleMarkdown`)
- Test: `internal/markdown/title_test.go`

- [ ] **Step 1: Move the test**

Move `TestDocTitle` from `cmd/mdsick/root_test.go` verbatim into `internal/markdown/title_test.go`, package `markdown`, calling `DocTitle`:

```go
package markdown

import "testing"

func TestDocTitle(t *testing.T) {
	for _, test := range []struct {
		name string
		doc  string
		want string
	}{
		{name: "leading h1", doc: "# Hello World\n\nbody\n", want: "Hello World"},
		{name: "inline markup", doc: "# *Bold*, `code` and [link](http://x)\n", want: "Bold, code and link"},
		{name: "entity", doc: "# Q &amp; A\n", want: "Q & A"},
		{name: "escaped punctuation", doc: "# A \\* B\n", want: "A * B"},
		{name: "h2 first", doc: "## Not a title\n", want: ""},
		{name: "paragraph first", doc: "intro\n\n# Later\n", want: ""},
		{name: "empty", doc: "", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := DocTitle([]byte(test.doc)); got != test.want {
				t.Fatalf("unexpected title:\ngot %q\nexp %q", got, test.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/markdown/ -run TestDocTitle -v`
Expected: FAIL (`DocTitle` undefined).

- [ ] **Step 3: Implement**

Create `internal/markdown/title.go`; move `titleMarkdown`, `docTitle` from root.go verbatim, exported:

```go
package markdown

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var titleMarkdown = goldmark.New()

// DocTitle returns the document's leading level-1 heading text, or ""
// when the first block is not an h1. It parses raw source with a plain
// goldmark instance so the TOC transformer's heading never leaks.
func DocTitle(source []byte) string {
	doc := titleMarkdown.Parser().Parse(text.NewReader(source))
	heading, ok := doc.FirstChild().(*ast.Heading)
	if !ok || heading.Level != 1 {
		return ""
	}

	var title bytes.Buffer
	if err := ast.Walk(heading, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindText {
			title.Write(n.(*ast.Text).Value(source))
		}
		return ast.WalkContinue, nil
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}

	resolved := util.ResolveNumericReferences(title.Bytes())
	resolved = util.ResolveEntityNames(resolved)
	resolved = util.UnescapePunctuations(resolved)
	return strings.TrimSpace(string(resolved))
}
```

Then in `cmd/mdsick/root.go`: delete `titleMarkdown` and `docTitle`; import `internal/markdown`; replace `docTitle(source)` call in `runDir()` with `markdown.DocTitle(source)`.

- [ ] **Step 4: Run all tests**

Run: `go test ./... && go vet ./...`
Expected: PASS.

- [ ] **Step 5: Commit with caveman-commit**

Message: `refactor: move doc title extraction`

---

### Task 2: `writeIssuesIndex`

**Files:**
- Create: `cmd/mdsick-issues/index.go`
- Test: `cmd/mdsick-issues/index_test.go`

- [ ] **Step 1: Write the failing test**

```go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malikbenkirane/mdsick/internal/issues"
)

func TestWriteIssuesIndex(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	})

	open := []issues.Row{
		{Number: 21, Base: "21.md", Side: "open", Head: "First"},
		{Number: 3, Base: "3.md", Side: "open"},
	}
	closed := []issues.Row{{Number: 10, Base: "10.md", Side: "closed", Head: "Archived"}}
	warns := []issues.Warning{{Source: "3.md", Ref: "#44"}}

	if err := writeIssuesIndex("Repo & Issues", open, closed, warns); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatal(err)
	}
	h := string(got)

	for _, want := range []string{
		"<title>Repo &amp; Issues</title>",
		"<h1>Repo &amp; Issues</h1>",
		"Open Issues", "Closed Issues",
		`<a href="21.html">21.md</a> <span class="file-title">First</span>`,
		`<a href="10.html">10.md</a> <span class="file-title">Archived</span>`,
		"## Warnings", "3.md: #44",
	} {
		if !strings.Contains(h, want) {
			t.Fatalf("missing %q in:\n%s", want, h)
		}
	}
}

func TestWriteIssuesIndexEmptySection(t *testing.T) {
	wd, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	})

	if err := writeIssuesIndex("Solo", []issues.Row{{Number: 7, Base: "7.md", Side: "open"}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatal(err)
	}
	h := string(got)
	if !strings.Contains(h, "No closed issues") {
		t.Fatalf("missing empty-section disclaimer:\n%s", h)
	}
	if strings.Contains(h, "No open issues") {
		t.Fatalf("open section should list entries:\n%s", h)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/mdsick-issues/ -v`
Expected: FAIL (package has no `writeIssuesIndex` yet; create `main.go` stub first if the package doesn't compile — see Task 3 ordering note: implement `main.go` after this task).

- [ ] **Step 3: Implement**

Create `cmd/mdsick-issues/index.go`:

```go
package main

import (
	"errors"
	"fmt"
	"html"
	"net/url"
	"os"
	"strings"

	"github.com/malikbenkirane/mdsick/internal/issues"
)

const (
	styleRef          = "github-markdown.css"
	paddingHorizontal = "6.47rem"
	paddingVertical   = "4rem"
)

// writeIssuesIndex renders the single landing page: two fixed sections
// (Open Issues, Closed Issues), reverse-natural-ordered entries, then a
// plain markdown-heading warnings block. Styl styling stays inside the
// markdown-body shell used by every mdsick page.
func writeIssuesIndex(title string, open, closed []issues.Row, warns []issues.Warning) (err error) {
	f, err := os.Create("index.html")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	_, err = fmt.Fprintf(f, `<!doctype html>
<html>
	<head>
		<title>%[4]s</title>
		<link rel="stylesheet" href="%[1]s">
		<style>
			body {
				padding-left: %[2]s;
				padding-right: %[2]s;
				padding-top: %[3]s;
				padding-bottom: %[3]s;
			}
			.file-title {
				color: #656d76;
				font-weight: normal;
				margin-left: 0.5rem;
			}
		</style>
	</head>
	<body class="markdown-body">
		<h1>%[4]s</h1>
`, styleRef, paddingHorizontal, paddingVertical, html.EscapeString(title))
	if err != nil {
		return err
	}

	if err := writeSection(f, "Open Issues", open); err != nil {
		return err
	}
	if err := writeSection(f, "Closed Issues", closed); err != nil {
		return err
	}

	if len(warns) > 0 {
		if _, err = fmt.Fprintf(f, "\t\t<h2>Warnings</h2>\n\t\t<pre>\n"); err != nil {
			return err
		}
		for _, w := range warns {
			if _, err = fmt.Fprintf(f, "\t\t\t%s\n", html.EscapeString(w.String())); err != nil {
				return err
			}
		}
		if _, err = f.WriteString("\t\t</pre>\n"); err != nil {
			return err
		}
	}

	_, err = fmt.Fprintf(f, "\t</body>\n</html>\n")
	return err
}

func writeSection(f *os.File, title string, rows []issues.Row) error {
	if _, err := fmt.Fprintf(f, "\t\t<h2>%s</h2>\n", html.EscapeString(title)); err != nil {
		return err
	}
	if len(rows) == 0 {
		_, err := fmt.Fprintf(f, "\t\t<p>No %s issues.</p>\n", strings.ToLower(title))
		return err
	}
	if _, err := f.WriteString("\t\t<ul>\n"); err != nil {
		return err
	}
	for _, row := range rows {
		href := fmt.Sprintf("%d.html", row.Number)
		name := html.EscapeString(row.Base)
		if _, err := fmt.Fprintf(f, "\t\t\t<li><a href=\"%s\">%s</a>", url.PathEscape(href), name); err != nil {
			return err
		}
		if row.Head != "" {
			if _, err := fmt.Fprintf(f, " <span class=\"file-title\">%s</span>", html.EscapeString(row.Head)); err != nil {
				return err
			}
		}
		if _, err := f.WriteString("</li>\n"); err != nil {
			return err
		}
	}
	_, err := f.WriteString("\t\t</ul>\n")
	return err
}
```

Notes for the engineer: disclaimers are `No open issues.` / `No closed issues.` — the test's substring `"No closed issues"` matches with or without the trailing period; conform the tests to the string possibly carrying the period. The test also asserts `"## Warnings"` — the writer emits `<h2>Warnings</h2>`; relax the test to `"<h2>Warnings</h2>"` (decided Q3.2: plain block under sections, markdown-body default styling).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/mdsick-issues/ -run TestWriteIssuesIndex -v`
Expected: PASS (after the two string relaxations above).

- [ ] **Step 5: Commit with caveman-commit**

Message: `feat: issues index writer`

---

**Files:**
- Create: `cmd/mdsick-issues/main.go`
- Test: `cmd/mdsick-issues/run_test.go`

The binary is split for testability: `build(rootPath string, titleArgs []string, show func(page string) error) (string, error)` does discovery → args-joined title → preflight → workspace → render → index, and the cobra `RunE` calls `build` then the macOS `open` shim. Tests pass a no-op `show` and assert the returned workspace path.

- [ ] **Step 1: Write the failing end-to-end test**

Create `cmd/mdsick-issues/run_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malikbenkirane/mdsick/internal/issues"

	"golang.org/x/net/html"
)

func chdirToRoot(t *testing.T, dir string) {
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
}

func fixtureRepo(t *testing.T, marker string) string {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "issues", "closed"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) {
		path := filepath.Join(root, rel)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join("docs", "issues", "21.md"), "# Alpha\n\nfixes #43 and [closed ten](closed/10.md)\n")
	write(filepath.Join("docs", "issues", "3.md"), "# Beta\n\nsee docs/issues/closed/10.md\n")
	write(filepath.Join("docs", "issues", "closed", "10.md"), "# Archived\n\nback-references #3 and `21.md`\n")
	write(filepath.Join("docs", "issues", "43.md"), "# Dangling\n\nsee #99\n")
	return root
}

// collectHrefs returns every href attribute value in the document, in
// order, using the stdlib-independent html tokenizer from x/net.
func collectHrefs(t *testing.T, body string) []string {
	t.Helper()
	var hrefs []string
	z := html.NewTokenizer(strings.NewReader(body))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return hrefs
		case html.StartTagToken:
			name, _ := z.TagName()
			if string(name) != "a" {
				continue
			}
			for {
				k, v, more := z.TagAttr()
				if string(k) == "href" {
					hrefs = append(hrefs, string(v))
				}
				if !more {
					break
				}
			}
		}
	}
}

func TestBuild(t *testing.T) {
	root := fixtureRepo(t, ".jj")
	chdirToRoot(t, root)

	show := func(path string) error {
		return nil // browser stub: record nothing, succeed
	}
	tmp, err := build(root, nil, show)
	if err != nil {
		t.Fatal(err)
	}

	mustRead := func(p string) string {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	idx := mustRead(filepath.Join(tmp, "index.html"))
	hrefs := collectHrefs(t, idx)
	if len(hrefs) != 3 {
		t.Fatalf("expected 3 anchors (21, 3, 10), got %v", hrefs)
	}
	for _, g := range []string{"21.html", "3.html", "10.html"} {
		found := false
		for _, h := range hrefs {
			if h == g {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing anchor %q in index hrefs %v", g, hrefs)
		}
	}

	page21 := mustRead(filepath.Join(tmp, "21.html"))
	if !strings.Contains(page21, `href="43.html"`) {
		t.Fatalf("hash ref to issue 43 missing:\n%s", page21)
	}
	if !strings.Contains(page21, `href="10.html"`) {
		t.Fatalf("dir-dest link to closed issue missing:\n%s", page21)
	}

	page10 := mustRead(filepath.Join(tmp, "10.html"))
	if !strings.Contains(page10, `href="21.html"`) {
		t.Fatalf("back-reference to open issue missing:\n%s", page10)
	}
	if !strings.Contains(page10, `href="3.html"`) {
		t.Fatalf("back-reference to issue 3 missing:\n%s", page10)
	}
}

func TestBuildWarningsSurfaceOnIndex(t *testing.T) {
	root := fixtureRepo(t, ".jj")
	chdirToRoot(t, root)

	tmp, err := build(root, nil, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(tmp, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	h := string(body)
	if !strings.Contains(h, "43.md: #99") {
		t.Fatalf("unresolvable warning missing from index:\n%s", h)
	}
	if !strings.Contains(h, "No closed issues") {
		t.Fatalf("empty-section disclaimer missing (fixture has no closed files visible? fix fixture):\n%s", h)
	}
}
```

Note: this second test's disclaimer assertion contradicts the fixture (which HAS a closed issue). Make `TestBuildWarningsSurfaceOnIndex` use its own fixture without a `closed/` entry:

```go
// in TestBuildWarningsSurfaceOnIndex, replace the fixture write list:
// write 21.md, 43.md only; keep the closed/ directory (must exist) but
// leave it empty, then assert "No closed issues".
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/mdsick-issues/ -run 'TestBuild' -v`
Expected: FAIL (`build` undefined).

- [ ] **Step 3: Implement `main.go`**

Create `cmd/mdsick-issues/main.go` (complete file, final version):

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/malikbenkirane/mdsick/internal/issues"
	"github.com/malikbenkirane/mdsick/internal/markdown"
	"github.com/malikbenkirane/mdsick/internal/workspace"
	"github.com/spf13/cobra"
	"github.com/yuin/goldmark/text"
)

var vcs string

// build renders every issue page flat in the workspace (NN.md ->
// NN.html regardless of open/closed side), writes the landing page,
// and hands the workspace directory plus page path to `show`.
// rootPath is the repo root; cwd may have changed to the workspace by
// the time show runs, so sources are read via their absolute paths.
func build(rootPath string, titleArgs []string, show func(page string) error) (string, error) {
	original, err := os.Getwd()
	if err != nil {
		return "", err
	}

	open, closed, err := issues.List(rootPath)
	if err != nil {
		return "", err
	}

	title := defaultTitle(rootPath)
	if len(titleArgs) > 0 {
		title = strings.Join(titleArgs, " ")
	}

	tmp, err := workspace.TempWorkspace()
	if err != nil {
		return "", err
	}

	ix := issues.NewIndex(open, closed)
	var warns []issues.Warning

	for _, rows := range [][]issues.Row{open, closed} {
		for _, row := range rows {
			srcPath := filepath.Join(original, "docs", "issues", row.Base)
			if row.Side == "closed" {
				srcPath = filepath.Join(original, "docs", "issues", "closed", row.Base)
			}
			source, err := os.ReadFile(srcPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			md := markdown.NewIssues(ix, row.Base, &warns)
			doc := md.Parser().Parse(text.NewReader(source))
			name := fmt.Sprintf("%d.html", row.Number)
			if err := workspace.RenderFile(name, row.Base, md, source, doc); err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			setHead(open, closed, row.Number, markdown.DocTitle(source))
		}
	}

	page := filepath.Join(tmp, "index.html")
	if err := os.Chdir(tmp); err != nil {
		return tmp, err
	}
	if err := writeIssuesIndex(title, open, closed, warns); err != nil {
		return tmp, err
	}
	if show != nil {
		if err := show(page); err != nil {
			return tmp, err
		}
	}
	return tmp, nil
}

func setHead(open, closed []issues.Row, number int, head string) {
	for i := range open {
		if open[i].Number == number {
			open[i].Head = head
		}
	}
	for i := range closed {
		if closed[i].Number == number {
			closed[i].Head = head
		}
	}
}

// defaultTitle: path/to/repo -> "to/repo Issues" (last two components).
func defaultTitle(rootPath string) string {
	return filepath.Base(filepath.Dir(rootPath)) + "/" + filepath.Base(rootPath) + " Issues"
}

func main() {
	cmd := &cobra.Command{
		Use:  "mdsick-issues [Title]",
		Long: "Render docs/issues and docs/issues/closed into one GitHub-styled landing page with flat per-issue pages.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootPath, err := issues.FindRepo(vcs)
			if err != nil {
				return err
			}
			if _, err := os.Stat(filepath.Join(rootPath, "docs", "issues")); err != nil {
				return fmt.Errorf("issues: docs/issues missing under %s", rootPath)
			}
			_, err = build(rootPath, args, func(page string) error {
				return exec.Command("open", page).Run()
			})
			return err
		},
	}
	cmd.Flags().StringVar(&vcs, "vcs", "auto", "repo marker preference: auto (jj first, then git), jj, git")
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

Remaining notes:
1. The `markdown` import is still needed for `markdown.NewIssues` in `build`.
2. The test calls `build(root, nil, func(page string) error { return nil })` — three args: rootPath, titleArgs, show. Update the earlier `TestBuild` call sites accordingly (they are shown with three args).

3. Duplicates and missing dirs already fail hard inside `issues.List` (sub-plan 02); the redundant `os.Stat` check in RunE is correct only as a friendlier error wrapper — keep it, it guards the "no discovery below the root" contract.

- [ ] **Step 4: Fetch deps, build, test everything**

Run:
```sh
go get github.com/spf13/cobra && go mod tidy
go get golang.org/x/net && go mod tidy
go build ./cmd/mdsick ./cmd/mdsick-issues
go test ./... && go vet ./...
```
Expected: both binaries build; all package tests pass; vet clean.

- [ ] **Step 5: Commit with caveman-commit**

Message: `feat: mdsick-issues binary renders issues landing page`

---

### Task 4: Final sweep and spec cross-check

**Files:**
- Modify: `docs/issues/9.md` (tick checklist items)
- Verify: `go test ./...`, `go vet ./...`, `golangci-lint run ./...`

- [ ] **Step 1: Run the full suite**

Run: `go test ./... && go vet ./... && golangci-lint run ./...`
Expected: tests pass, vet clean, lint at most the accepted baseline observations (record any new warnings in the run log and fix or file).

- [ ] **Step 2: Manual smoke from the real repo**

Run: `go run ./cmd/mdsick-issues` from the mdsick repo itself.
Expected: browser shows the landing page: Open Issues (11, 13, 6, 7, 9, 14 reverse order), Closed Issues (10, 5, 3, 1 reverse order), page title `sensible/mdsick Issues`.

- [ ] **Step 3: Tick the checklist in docs/issues/9.md**

Mark every checklist item complete except any deliberately deferred (e.g. the SLM detector in docs/issues/14.md stays out of scope). Commit docs with caveman-commit.

Message: `docs(issues): mark issue 9 implemented`

- [ ] **Step 4: Final commit check**

Run: `jj status`
Expected: clean working copy; all four sub-plan commits visible in `jj log` (ask the user to inspect; `jj log` is restricted by core-commands).
