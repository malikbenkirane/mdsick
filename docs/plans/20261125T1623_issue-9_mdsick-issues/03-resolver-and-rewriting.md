# Resolver and Rewriting (03) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform a parsed issue document so every issue reference becomes a link to the flat `NN.html` page for that issue number, collecting unresolvable references as warnings instead of dropping them.

**Architecture:** `internal/issues` gains `Index` (number → resolvability), `Warning`, and an `ast.Transformer`. The transformer walks once, classifies nodes into rewrite kinds, defers AST mutation until after the walk (the same pattern `internal/transformers/mermaid.go` uses, because mutating mid-walk is unsafe), applies the queued changes, and then lets the caller render. Markdown *link destinations* and *prose text* rewrite by the number only: a directory part in a link target like `closed/8.md` is ignored. Fenced code blocks are skipped entirely; code spans and prose ARE rewritten.

**Tech Stack:** goldmark AST (`ast`, `parser`, `text`), `regexp`, `internal/markdown` engine, `internal/workspace.RenderFile`.

---

## Current state (read this first)

- Requires sub-plan 01 (`internal/markdown`, `internal/workspace`) and the `Row` type from sub-plan 02 (`internal/issues/list.go`). The transformer itself only needs `Index` + `Warning`, whose `Row`-free tests run independently.
- Number-only resolution contract (docs/issues/9.md):
  1. Prose text `#NN` → link to `NN.html` (word-boundary regex; false positives are out of scope, do not over-engineer).
  2. Link `[label](…NN.md)` → destination rewritten to `NN.html` regardless of label or directory part; unresolvable → warning with the raw ref.
  3. Prose `issue NN` (case-insensitive) → the matched phrase becomes a link.
  4. Prose or code-span path refs `NN.md`, `docs/issues/NN.md`, `docs/issues/closed/NN.md` → the matched path text becomes a link (label keeps the original text).
  5. Fenced code blocks skipped. Everything else rewritten.
- Sythesized replacement nodes use `ast.NewString` — the same mechanism goldmark's own Linkify extension uses; the default HTML renderer renders `ast.String`.

---

### Task 1: `Index` and `Warning`

**Files:**
- Create: `internal/issues/resolver.go`
- Test: `internal/issues/resolver_test.go`

- [ ] **Step 1: Write the failing test**

```go
package issues

import "testing"

func TestNewIndex(t *testing.T) {
	open := []Row{{Number: 21}, {Number: 3}}
	closed := []Row{{Number: 10}}
	ix := NewIndex(open, closed)

	if href, ok := ix.Resolve(21); !ok || href != "21.html" {
		t.Fatalf("Resolve(21) = %q,%v; want 21.html,true", href, ok)
	}
	if _, ok := ix.Resolve(10); !ok {
		t.Fatal("closed issue must resolve")
	}
	if href, ok := ix.Resolve(44); ok || href != "" {
		t.Fatalf("Resolve(44) = %q,%v; want \"\",false", href, ok)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/issues/ -run TestNewIndex -v`
Expected: FAIL (`NewIndex` undefined).

- [ ] **Step 3: Write minimal implementation**

```go
package issues

import "fmt"

// Index is the flat set of resolvable issue numbers, regardless of
// which side an issue currently lives on.
type Index struct {
	numbers map[int]bool
}

func NewIndex(open, closed []Row) *Index {
	ix := &Index{numbers: make(map[int]bool)}
	for _, rows := range [][]Row{open, closed} {
		for _, row := range rows {
			ix.numbers[row.Number] = true
		}
	}
	return ix
}

func (ix *Index) Resolve(n int) (href string, ok bool) {
	if !ix.numbers[n] {
		return "", false
	}
	return fmt.Sprintf("%d.html", n), true
}

// Warning is one unresolvable reference, raised on the index page.
type Warning struct {
	Source string // the issue file the reference came from, e.g. "43.md"
	Ref    string // the raw unresolvable reference, e.g. "#44"
}

func (w Warning) String() string {
	return fmt.Sprintf("%s: %s — unresolvable", w.Source, w.Ref)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/issues/`
Expected: PASS

- [ ] **Step 5: Commit with caveman-commit**

Message: `feat: issue number index`

---

### Task 2: The references transformer (all tiers together)

This is the core task; one transformer, four rewrite kinds, one deferred-apply pass.

**Files:**
- Create: `internal/issues/refs.go`
- Test: `internal/issues/refs_test.go`

- [ ] **Step 1: Write the failing test**

```go
package issues

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/malikbenkirane/mdsick/internal/markdown"
)

func renderIssues(t *testing.T, ix *Index, src string) (string, []Warning) {
	t.Helper()
	var warns []Warning
	md := markdown.NewIssues(ix, "9.md", &warns)
	doc := md.Parser().Parse(text.NewReader([]byte(src)))
	var b bytes.Buffer
	if err := md.Renderer().Render(&b, []byte(src), doc); err != nil {
		t.Fatal(err)
	}
	return b.String(), warns
}

func TestRewrites(t *testing.T) {
	ix := NewIndex([]Row{{Number: 21}, {Number: 8}, {Number: 43}}, nil)

	t.Run("hash ref in prose", func(t *testing.T) {
		out, _ := renderIssues(t, ix, "fixes #21 now\n")
		if !strings.Contains(out, `<a href="21.html">#21</a>`) {
			t.Fatalf("hash ref not linked:\n%s", out)
		}
	})

	t.Run("code fence skipped, code span rewritten", func(t *testing.T) {
		out, _ := renderIssues(t, ix, "```\nsee #21 raw\n```\n\nref `43.md` inline\n")
		if strings.Contains(out, `<a href="21.html">`) {
			t.Fatalf("fenced code was rewritten:\n%s", out)
		}
		if strings.Contains(out, "<code>21") {
			t.Fatalf("hash inside code span unexpectedly rewritten:\n%s", out)
		}
		if !strings.Contains(out, fmt.Sprintf("<code>%s", "43")) && !strings.Contains(out, `href="43.html"`) {
			t.Fatalf("code span path not linked:\n%s", out)
		}
	})

	t.Run("link destination with directory part", func(t *testing.T) {
		out, _ := renderIssues(t, ix, "[8](closed/8.md) and [epic](21.md)\n")
		if !strings.Contains(out, `<a href="8.html">8</a>`) {
			t.Fatalf("dir dest not rewritten:\n%s", out)
		}
		if !strings.Contains(out, `<a href="21.html">epic</a>`) {
			t.Fatalf("epic label link not rewritten:\n%s", out)
		}
	})

	t.Run("unresolvable flows to warnings", func(t *testing.T) {
		out, warns := renderIssues(t, ix, "see #44 and [9](9.md)\n")
		if !strings.Contains(out, "9.md") {
			t.Fatalf("unresolvable references must stay intact:\n%s", out)
		}
		if len(warns) != 2 {
			t.Fatalf("expected 2 warnings, got %v", warns)
		}
		for _, w := range warns {
			if w.Source != "9.md" || (w.Ref != "#44" && w.Ref != "9.md") {
				t.Fatalf("bad warning: %+v", w)
			}
		}
	})

	t.Run("prose issue phrase", func(t *testing.T) {
		out, _ := renderIssues(t, ix, "we address issue 8 here (not issue 44)\n")
		if !strings.Contains(out, `href="8.html"`) {
			t.Fatalf("issue phrase not linkified:\n%s", out)
		}
	})

	t.Run("full path in prose", func(t *testing.T) {
		out, _ := renderIssues(t, ix, "see docs/issues/closed/21.md fully\n")
		if !strings.Contains(out, `href="21.html"`) {
			t.Fatalf("path not linkified:\n%s", out)
		}
	})
}
```

(Include `github.com/yuin/goldmark/text` import in the helper.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/issues/ -run TestRewrites -v`
Expected: FAIL (`markdown.NewIssues` undefined).

- [ ] **Step 3: Implement `markdown.NewIssues`**

Create `internal/issues/refs.go` first, then extend `internal/markdown/markdown.go` (shown as Step 6). Transformer:

```go
package issues

import (
	"regexp"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// Transformer rewrites issue references in one document. One instance
// per file; instantiate fresh for every document (it holds per-file
// state: source filename for warnings and the collected mutations).
type Transformer struct {
	index    *Index
	source   string
	warnings *[]Warning
	edits    []edit
}

type edit struct {
	parent ast.Node
	old    ast.Node
	new    []ast.Node
}

var (
	hashRe   = regexp.MustCompile(`(?:^|[^\w])#(\d+)\b`)
	issuePhr = regexp.MustCompile(`(?i)\bissue[ \t]+(\d+)\b`)
	nnMd     = regexp.MustCompile(`\b(\d+)\.md\b`)
)

func (t *Transformer) warn(source, ref string) {
	*t.warnings = append(*t.warnings, Warning{Source: source, Ref: ref})
}

// splitAndLink builds the replacement node sequence for one text
// value using one regex over `val`. Group `numGroup` carries the issue
// number; the full match (loc[0]..loc[1]) becomes the link label.
// A nil result means "no matches" or "nothing resolvable"; warnings
// are still collected for the unresolvable ones. Fragment nodes are
// ast.String (same mechanism goldmark's Linkify extension uses) and
// ast.Link with one ast.String child.
func (t *Transformer) splitAndLink(val []byte, re *regexp.Regexp, numGroup int) []ast.Node {

```go
package issues

import (
	"fmt"
	"regexp"

	"github.com/malikbenkirane/mdsick/internal/markdown"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type Transformer struct {
	index    *Index
	source   string
	warnings *[]Warning
	edits    []edit
}

type edit struct {
	parent ast.Node
	old    ast.Node
	new    []ast.Node
}

var (
	hashRe   = regexp.MustCompile(`(?:^|[^\w])#(\d+)\b`)
	issueRe  = regexp.MustCompile(`(?i)(\bissue[ \t]+(?:closed[ \t]+)?)(\d+)(\b)`)
	pathRe   = regexp.MustCompile(`\b(?:docs/issues(?:/closed)?/)?(\d+)\.(?:md)\b`)
)

func (t *Transformer) warn(source, ref string) {
	*t.warnings = append(*t.warnings, Warning{Source: source, Ref: ref})
}

// splitAndLink walks re.FindSubmatchIndex over val and builds a node
// sequence where each reference match becomes
// [leading-String, link(match), trailing-String ...] fragments.
func (t *Transformer) splitAndLink(val []byte, re *regexp.Regexp, numGroup int) []ast.Node {
	var out []ast.Node
	last := 0
	locs := re.FindAllSubmatchIndex(val, -1)
	if locs == nil {
		return nil
	}
	for _, loc := range locs {
		start, stop := loc[2*numGroup], loc[2*numGroup+1]
		num, _ := strconv.Atoi(string(val[start:stop]))
		href, ok := t.index.Resolve(num)
		if !ok {
			t.warn(t.source, string(val[start:stop]))
			continue
		}
		if start > last {
			out = append(out, str(val[last:start]))
		}
		label := val[loc[0]:loc[1]]
		ln := ast.NewLink()
		ln.Destination = []byte(href)
		ln.AppendChild(ln, ast.NewString(label))
		out = append(out, ln)
		last = loc[1]
	}
	if last < len(val) {
		out = append(out, str(val[last:]))
	}
	return out
}

func str(b []byte) ast.Node { return ast.NewString(b) }

func (t *Transformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.FencedCodeBlock:
			return ast.WalkSkipChildren, nil
		case *ast.Text:
			if nodes := t.rewriteText(n, reader); nodes != nil {
				parent := n.Parent()
				t.edits = append(t.edits, edit{parent: parent, old: n, new: nodes})
			}
		case *ast.Link:
			t.rewriteDest(n, n.Destination)
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		warn(fmt.Sprintf("%v", err))
	}

	for _, e := range t.edits {
		parent.ReplaceChild(parent, e.old, e.new[0])
		prev := e.new[0]
		for _, extra := range e.new[1:] {
			parent.InsertAfter(parent, prev, extra)
			prev = extra
		}
	}
}

func (t *Transformer) rewriteText(n *ast.Text, reader text.Reader) []ast.Node {
	val := n.Segment.Value(reader.Source())
	nodes := t.splitAndLink(val, hashRe, 2)
	if nodes == nil {
		nodes = t.splitAndLink(val, pathRe, 1)
	}
	if nodes == nil {
		nodes = t.splitAndLink(val, issueRe, 2)
	}
	return nodes
}

func (t *Transformer) rewriteDest(link *ast.Link, dst []byte) {
	loc := pathRe.FindSubmatch(dst)
	if loc == nil {
		return
	}
	num := atoi(loc[1])
	href, ok := t.index.Resolve(num)
	if !ok {
		t.warn(t.source, string(dst))
		return
	}
	link.Destination = []byte(href)
}

func atoi(b []byte) int { n, _ := strconv.Atoi(string(b)); return n }
```

(Add `"strconv"` imports as flagged; `warn(...)` in the walk-err branch should be `fmt.Fprintln(os.Stderr, err)`.)

Notes for the engineer:
- CodeSpan children are `ast.Text` nodes, so case `*ast.Text` covers code spans automatically.
- `hashRe` uses capture group 2 (the digits) because group 1 is the boundary prefix.
- `issueRe` (and every other tier) links the FULL match text (`loc[0]` to `loc[1]`), so `` issue 8 `` is a phrase-level link and `docs/issues/closed/21.md` keeps its path label.

- [ ] **Step 4: Run the transformer tests (engine already has NewIssues? No)**

Run: `go build ./...`
Expected: FAIL only at `markdown.NewIssues` call site in the test.

- [ ] **Step 5: Add `markdown.NewIssues`**

Append to `internal/markdown/markdown.go`:

```go
func NewIssues(index *issues.Index, source string, warnings *[]issues.Warning) goldmark.Markdown {
	md := New().(*engine).Options...
}
```

No — the real wrapper (goldmark does not mutate after build):

```go
package markdown

import (
	"github.com/malikbenkirane/mdsick/internal/issues"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
)

// NewIssues is New() plus the issue-reference transformer, which runs
// AFTER the mermaid transformer for the given file (source name is the
// base issue filename, used in warnings).
func NewIssues(index *issues.Index, source string, warnings *[]issues.Warning) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithParserOptions(
			parser.WithASTTransformers(
				util.Prioritized(&transformers.Mermaid{}, 0),
				util.Prioritized(&issues.Transformer{index: index, source: source, warnings: warnings}, 100),
			),
		),
	)
}
```

`goldmark.New()` inherits nothing; the wrapper must repeat every option. The complete correct version:

```go
func NewIssues(index *issues.Index, source string, warnings *[]issues.Warning) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.PrioritizedValue{
				Value: transformers.CodeLiteralRenderer{},
			}),
		),
		goldmark.WithParserOptions(
			parser.WithASTTransformers(
				util.Prioritized(&transformers.Mermaid{}, 0),
				util.Prioritized(&issues.Transformer{index: index, source: source, warnings: warnings}, 100),
			),
		),
		goldmark.WithExtensions(&toc.Extender{}),
		goldmark.WithExtensions(extension.Linkify),
		goldmark.WithExtensions(extension.Strikethrough),
		goldmark.WithExtensions(extension.Table),
	)
}
```

(`util.PrioritizedValue` for nodes, `util.Prioritized` for transformers — both exist in goldmark.)

- [ ] **Step 6: Run tests**

Run: `go test ./internal/issues/ ./internal/markdown/ -v`
Expected: PASS. Iterate until all rewrite kinds pass. Known goldmark subtleties to expect:
- `ast.String` inside a Paragraph renders inline (Linkify extension does exactly this).
- A Link's *label text* is untouched; only `Destination` changes for tier 2.
- If a Text node matches two different regexes, only the FIRST matching kind applies per node (acceptable: a node rarely matches two tiers; if a test fails on a compound sentence, split the test corpus).

- [ ] **Step 7: Commit with caveman-commit**

Message: `feat: issue reference rewriting`
