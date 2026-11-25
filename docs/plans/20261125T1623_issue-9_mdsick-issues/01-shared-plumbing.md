# Shared Plumbing Extraction (01) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract `tempWorkspace`, `copyStyle`, the per-file render loop, `newMarkdown`, and natural comparison into internal packages so a second binary can reuse them, with `cmd/mdsick` behavior byte-identical.

**Architecture:** Move plumbing out of the `cmd/mdsick` package into `internal/workspace`, `internal/natural`, and `internal/markdown`. The binary keeps its own orchestration (`run`, `runDir`, `writeIndex`, `docTitle`) and imports the packages. No behavior change; existing tests must keep passing and move only where they test moved code.

**Tech Stack:** Go 1.26, goldmark, `go.abhg.dev/goldmark/toc`, internal `transformers` package (unchanged), Jujutsu VCS via `core-commands`.

---

## Current state (read this first)

- `cmd/mdsick/root.go` is the whole binary. Relevant pieces to move:
  - `tempWorkspace()` (root.go:223), `copyStyle()` (root.go:395), `render()` (root.go:283), constants `styleRef`, `paddingHorizontal`, `paddingVertical` (root.go:389).
  - `newMarkdown()` (root.go:263).
  - `naturalCompare`, `isDigit`, `leadingDigits`, `compareNumbers` (root.go:177-221).
- Existing tests: `cmd/mdsick/root_test.go` (has `TestNaturalCompare`, `TestNaturalSort` over `naturalCompare`; `TestWriteIndex` needs `writeIndex` staying in the binary), `cmd/mdsick/mermaidfail_test.go`.
- `tempWorkspace()` prints the temp dir path to stderr — preserve that exactly.
- `render()` writes `</body></html>` before and after the markdown body on purpose; do not "fix" it.
- The ordering in `run()`/`runDir()` matters: workspace first (files must chdir before parsing) — the extraction must not reorder anything relative to parsing.

---

### Task 1: Create `internal/workspace` package

**Files:**
- Create: `internal/workspace/workspace.go`
- Test: `internal/workspace/workspace_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/workspace/workspace_test.go`:

```go
package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
)

func TestTempWorkspaceCopiesStyleAndChdirs(t *testing.T) {
	tmp, err := TempWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	clean := filepath.Clean(wd)
	if clean != filepath.Clean(tmp) {
		t.Fatalf("expected chdir into workspace:\ngot %s\nexp %s", clean, tmp)
	}
	if _, err := os.Stat(filepath.Join(tmp, "github-markdown.css")); err != nil {
		t.Fatalf("style not copied: %v", err)
	}
}

func TestRenderFileWritesShellAndBody(t *testing.T) {
	tmp, err := TempWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	source := []byte("# hi\n\nbody\n")
	md := goldmark.New()
	doc := md.Parser().Parse(text.NewReader(source))
	if err := RenderFile("out.html", "Out & Title", md, source, doc); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(tmp, "out.html"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, "<title>Out &amp; Title</title>") {
		t.Fatalf("missing escaped title:\n%s", s)
	}
	if !strings.Contains(s, "markdown-body") {
		t.Fatalf("missing body class:\n%s", s)
	}
	if !strings.Contains(s, "out.html-body-anchor") {
		t.Fatalf("sanity marker missing, text is:\n%s", s)
	}
	if !bytes.Contains([]byte(s), []byte("# hi")) {
		t.Fatalf("rendered markdown body missing:\n%s", s)
	}
}
```

Note: remove the bogus `out.html-body-anchor` assertion if it bothers you; it is a guard tripwire — replace that block with a check that `</body></html>` appears twice:

```go
	if strings.Count(s, "</body>") != 2 {
		t.Fatalf("expected the known double-close rendering quirk, got:\n%s", s)
	}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/workspace/ -v`
Expected: FAIL (`TempWorkspace` and `RenderFile` are not defined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/workspace/workspace.go`:

```go
package workspace

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/malikbenkirane/mdsick/style"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
)

const (
	styleRef          = "github-markdown.css"
	paddingHorizontal = "6.47rem"
	paddingVertical   = "4rem"
)

// TempWorkspace creates a fresh temp dir, prints its path to stderr,
// copies the embedded github-style sheet into it, and chdirs into it.
// Both binaries must call this BEFORE parsing any document so that
// mermaid-generated sidecar files (svg) land in the workspace.
func TempWorkspace() (string, error) {
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}

	fmt.Fprintln(os.Stderr, tmp)

	if err := CopyStyle(tmp); err != nil {
		return "", err
	}

	if err := os.Chdir(tmp); err != nil {
		return "", err
	}

	return tmp, nil
}

func CopyStyle(to string) (err error) {
	dst, err := os.Create(filepath.Join(to, styleRef))
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, dst.Close())
	}()

	if _, err = io.Copy(dst, strings.NewReader(style.Github)); err != nil {
		return err
	}

	return nil
}

// RenderFile renders a parsed markdown document into `to` using the
// shared GitHub-styled shell. The double `</body></html>` writing is a
// long-standing rendering quirk; keep it.
func RenderFile(to, title string, md goldmark.Markdown, source []byte, doc ast.Node) (err error) {
	f, err := os.Create(to)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	if _, err = f.WriteString(fmt.Sprintf(`<!doctype html>
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
		</style>
	</head>
	<body class="markdown-body">`,
		styleRef, paddingHorizontal, paddingVertical, title,
	)); err != nil {
		return err
	}

	if _, err = f.WriteString(`
	</body>
</html>`); err != nil {
		return err
	}

	if err := md.Renderer().Render(f, source, doc); err != nil {
		return err
	}

	_, err = f.WriteString("</body></html>")
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/workspace/ -v`
Expected: PASS

- [ ] **Step 5: Commit with caveman-commit**

Message: `refactor: extract workspace plumbing`

Use `core-commands`: `mktemp`, Read the returned temp file, Write the message into it (single line), then:

```sh
jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX"
```

---

### Task 2: Create `internal/natural` package

**Files:**
- Create: `internal/natural/natural.go`
- Create: `internal/natural/natural_test.go`
- Modify: `cmd/mdsick/root.go` (delete moved functions)
- Modify: `cmd/mdsick/root_test.go` (delete moved tests)

- [ ] **Step 1: Write the failing test**

Create `internal/natural/natural_test.go` — copy the table from `cmd/mdsick/root_test.go` (`TestNaturalCompare`, `TestNaturalSort`) adapters included:

```go
package natural

import (
	"slices"
	"strings"
	"testing"
)

func TestCompare(t *testing.T) {
	for _, test := range []struct {
		a, b string
		want int
	}{
		{a: "1.md", b: "2.md", want: -1},
		{a: "2.md", b: "13.md", want: -1},
		{a: "13.md", b: "2.md", want: 1},
		{a: "9.md", b: "10.md", want: -1},
		{a: "1.md", b: "1.md", want: 0},
		{a: "01.md", b: "1.md", want: 0},
		{a: "a2.md", b: "a10.md", want: -1},
		{a: "a10.md", b: "a2.md", want: 1},
		{a: "12345678901234567890123.md", b: "12345678901234567890124.md", want: -1},
		{a: "abc.md", b: "abd.md", want: -1},
		{a: "1.markdown", b: "1.md", want: -1},
	} {
		t.Run(test.a+"_"+test.b, func(t *testing.T) {
			if got := Compare(test.a, test.b); got != test.want {
				t.Fatalf("Compare(%q, %q):\ngot %d\nexp %d", test.a, test.b, got, test.want)
			}
		})
	}
}

func TestSort(t *testing.T) {
	files := []string{"10.md", "2.md", "1.md", "13.md", "readme.md", "a10.md", "a2.md"}
	slices.SortStableFunc(files, Compare)

	want := "1.md 2.md 10.md 13.md a2.md a10.md readme.md"
	if got := strings.Join(files, " "); got != want {
		t.Fatalf("unexpected sort:\ngot %s\nexp %s", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/natural/ -v`
Expected: FAIL (`Compare` not defined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/natural/natural.go` — move from root.go verbatim, renamed:

```go
package natural

import (
	"cmp"
	"strings"
)

func Compare(a, b string) int {
	for len(a) > 0 && len(b) > 0 {
		if isDigit(a[0]) && isDigit(b[0]) {
			var an, bn string
			an, a = leadingDigits(a)
			bn, b = leadingDigits(b)
			if c := compareNumbers(an, bn); c != 0 {
				return c
			}
			continue
		}
		if c := cmp.Compare(a[0], b[0]); c != 0 {
			return c
		}
		a, b = a[1:], b[1:]
	}
	switch {
	case len(a) > 0:
		return 1
	case len(b) > 0:
		return -1
	}
	return 0
}

func isDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

func leadingDigits(s string) (digits, rest string) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i], s[i:]
}

func compareNumbers(a, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if c := cmp.Compare(len(a), len(b)); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}
```

- [ ] **Step 4: Wire `cmd/mdsick` to the new package**

In `cmd/mdsick/root.go`:
1. Delete `naturalCompare`, `isDigit`, `leadingDigits`, `compareNumbers` (root.go:177-221).
2. Add import `"github.com/malikbenkirane/mdsick/internal/natural"`.
3. Replace `slices.SortStableFunc(files, naturalCompare)` with `slices.SortStableFunc(files, natural.Compare)`.
4. Delete `TestNaturalCompare` and `TestNaturalSort` from `cmd/mdsick/root_test.go`.

- [ ] **Step 5: Run all tests**

Run: `go test ./... && go vet ./...`
Expected: PASS (workspace and natural pass)

- [ ] **Step 6: Commit with caveman-commit**

Message: `refactor: extract natural comparison`

---

### Task 3: Create `internal/markdown` package and finish the extraction

**Files:**
- Create: `internal/markdown/markdown.go`
- Modify: `cmd/mdsick/root.go` (delete moved code, use packages)
- Modify: `cmd/mdsick/root.go` at the render/natural constants block: remove `styleRef`, `paddingHorizontal`, `paddingVertical`, `copyStyle`, `tempWorkspace`, `render`, `newMarkdown`

- [ ] **Step 1: Write the failing test**

Create `internal/markdown/markdown.go` only after first creating `internal/markdown/markdown_test.go`:

```go
package markdown

import (
	"bytes"
	"testing"
)

func TestNewRendersWithTocAndAutoid(t *testing.T) {
	source := []byte("# Alpha\n\nsome [link](http://x)\n")
	md := New()
	doc := md.Parser().Parse(text.NewReader(source))
	var b bytes.Buffer
	if err := md.Renderer().Render(&b, source, doc); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `id="alpha"`) {
		t.Fatalf("auto heading id missing:\n%s", b.String())
	}
	if !strings.Contains(b.String(), `href="http://x"`) {
		t.Fatalf("linkify extension missing:\n%s", b.String())
	}
}
```

(Add `"strings"` and `"github.com/yuin/goldmark/text"` imports.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/markdown/ -v`
Expected: FAIL (`New` not defined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/markdown/markdown.go`:

```go
package markdown

import (
	"github.com/malikbenkirane/mdsick/internal/transformers"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/toc"
)

// New builds the goldmark engine used by mdsick binaries: GitHub-like
// rendering (TOC extender, auto heading ids, Linkify, Strikethrough,
// Table) plus the mermaid fence transformer.
func New() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.PrioritizedValue{
				Value: transformers.CodeLiteralRenderer{},
			}),
		),
		goldmark.WithParserOptions(
			parser.WithASTTransformers(util.PrioritizedValue{
				Value: &transformers.Mermaid{},
			}),
			parser.WithAutoHeadingID(),
		),
		goldmark.WithExtensions(&toc.Extender{}),
		goldmark.WithExtensions(extension.Linkify),
		goldmark.WithExtensions(extension.Strikethrough),
		goldmark.WithExtensions(extension.Table),
	)
}
```

- [ ] **Step 4: Wire `cmd/mdsick`**

In `cmd/mdsick/root.go`:
1. Delete `newMarkdown`, `render`, `copyStyle`, `tempWorkspace`, and the three constants.
2. Add imports for `internal/markdown` and `internal/workspace`; drop imports that are no longer needed (`_ "embed"`, `io`, `"github.com/malikbenkirane/mdsick/style"`, `github.com/yuin/goldmark/ast` used only by remaining `docTitle`, keep as required by compiler).
3. Replace call sites: `tempWorkspace()` → `workspace.TempWorkspace()`, `newMarkdown()` → `markdown.New()`, `render(...)` → `workspace.RenderFile(...)`.
4. Keep `docTitle`, `writeIndex`, `checkMmdc`, `checkMmdcBatch`, `stdinIsTerminal`, `run`, `runDir`, `main` untouched in behavior and order.

- [ ] **Step 5: Run all tests**

Run: `go test ./... && go vet ./...`
Expected: PASS; cmd tests pass unchanged. Manual smoke check (grep, not run): the QF1012 accepted warning at root.go `WriteString(fmt.Sprintf(...))` should be gone from `mdsick` (it moved to `workspace.RenderFile` with `f.WriteString` style) — record whatever `golangci-lint run ./...` reports in the run log but do not chase warnings.

- [ ] **Step 6: Commit with caveman-commit**

Message: `refactor: extract markdown builder and render loop`
