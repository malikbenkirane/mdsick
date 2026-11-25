# 02 — Failure artifact (issue 10)

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a mermaid block fails to compile, the rendered HTML keeps the
fence alive with every body line prefixed by its diagram-relative line number
(`1| …`), and a `text` fence with mmdc's full `Failure.Output` appears
directly after it — silently, no stderr noise, run continues.

**Architecture:** goldmark's AST is strictly source-offset based (every
textual payload is a `text.Segment` resolved against the document source at
render time), so literal new content (`1| …` prefixed lines, mmdc output)
cannot ride native `FencedCodeBlock` segments. Both artifact carriers are
therefore a custom block node, `CodeLiteral{lang, lines}`, rendered by a
registered custom renderer whose output is byte-identical to what the default
fenced-code renderer emits (`<pre><code class="language-X">…</code></pre>`).
Insertion uses `InsertAfter` (goldmark v1.8.2, `ast/ast.go:121`); the two per-
failure changes are ordered **insert-then-replace** (see Why below).

**Tech Stack:** Go stdlib, goldmark (custom node kind + node renderer), the `*mermaid.Failure` type from sub-plan 01.

---

## Legend decisions carried by this sub-plan

- [**artifact**] the original mermaid fence survives with each *body* line prefixed `1|`, `2|`, …; the fence header reads plain `mermaid` (extra info strings are not reconstructed — Q1.4 option 1).
- [**artifact**] directly after it, a fenced `text` block holds `Failure.Output` in full, no truncation.
- [**artifact**] a single new change kind `insertAfter` reuses `InsertAfter`; no fragment node is invented.
- [**q1**] infra errors (`randHex`, `os.Create`, write) do **not** get artifacts — they stay stderr+joint-exit (settled Q2.1 option 2).

## Why insert-then-replace (the ordering constraint)

The deferred apply loop walks `changes` in event arrival order. For one
failed block, the two changes are `InsertAfter(parent, oldChild, artifact)`
and `ReplaceChild(parent, oldChild, numberedFence)`. If the replace ran
first, `oldChild` would already be detached, and goldmark's `InsertBefore`
falls back to *appending to the tail* when `v1.Parent() != self`
(`ast/ast.go:388`) — putting the artifact at the end of the document instead
of next to its block. Because both events are sent from the same `render`
goroutine onto the same FIFO channel, "insert event first" guarantees the
correct order for every block, whatever the interleaving across blocks.
The plan encodes this ordering in `render`, and a test asserts artifact
adjacency to lock it.

## Verified goldmark facts (v1.8.2, from the module cache)

- `ast.Node` interface exposes `InsertAfter(self, v1, insertee)` (`ast/ast.go:121`).
- `ReplaceChild(self, v1, insertee)` = `InsertBefore` + `RemoveChild` (`ast/ast.go:370`).
- Default fenced renderer emits `<pre><code` + ` class="language-<lang>"` (when any) + lines + `</code></pre>\n` (`renderer/html/html.go:360-377`); HTML escaper available as `util.EscapeHTML` (`util/util.go:555`).
- `renderer.WithNodeRenderers(util.PrioritizedValue{Value: r})` extends the HTML renderer (`renderer/renderer.go:43-45`).
- Custom block: embed `ast.BaseBlock` (gives `Type`, `Lines`, `Has/SetBlankPreviousLines`, sibling plumbing); override `Kind()`, force `IsRaw() = true`, provide `Dump`; `Text` has a `BaseNode` default (`ast/ast.go:423`).

---

## Task 1: The `CodeLiteral` node and its renderer

**Files:**

- Create: `internal/transformers/codeliteral.go`
- Modify: `cmd/mdsick/root.go` (`newMarkdown()`, renderer registration only)
- Test: `internal/transformers/codeliteral_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/transformers/codeliteral_test.go`. The test builds a small
document holding one `codeLiteral` by hand and renders it through a goldmark
instance configured exactly the way `newMarkdown()` will be:

```go
package transformers

import (
	"bytes"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

func TestCodeLiteralRendering(t *testing.T) {
	doc := ast.NewDocument()
	blk := &codeLiteral{lang: "mermaid", lines: []string{"1| flowchart A", "2| <emo> & stuff", "3| end"}}
	blk.SetBlankPreviousLines(true)
	doc.AppendChild(doc, blk)

	md := goldmark.New(
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.PrioritizedValue{
				Value: CodeLiteralRenderer{},
			}),
		),
	)
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, nil, doc); err != nil {
		t.Fatal(err)
	}

	exp := "<pre><code class=\"language-mermaid\">1| flowchart A\n2| &lt;emo&gt; &amp; stuff\n3| end\n</code></pre>\n"
	if buf.String() != exp {
		t.Fatalf("unexpected rendering:\ngot %q\nexp %q", buf.String(), exp)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/transformers/ -run TestCodeLiteralRendering -v`
Expected: FAIL — `undefined: codeLiteral` (node does not exist yet).

- [ ] **Step 3: Write the node and renderer**

Create `internal/transformers/codeliteral.go`:

```go
package transformers

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

var KindCodeLiteral = ast.NewNodeKind("CodeLiteral")

// codeLiteral carries literal content that has no counterpart in the
// document source: goldmark segments resolve against the source buffer, so
// cannot host synthesized lines.
type codeLiteral struct {
	ast.BaseBlock
	lang  string
	lines []string
}

func (c *codeLiteral) Kind() ast.NodeKind { return KindCodeLiteral }

func (c *codeLiteral) IsRaw() bool { return true }

func (c *codeLiteral) Dump(source []byte, level int) {}

// CodeLiteralRenderer renders codeLiteral exactly like the default HTML
// renderer renders a fenced code block with the same language and lines.
type CodeLiteralRenderer struct{}

func (CodeLiteralRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindCodeLiteral, renderCodeLiteral)
}

func renderCodeLiteral(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		w.WriteString("</code></pre>\n")
		return ast.WalkContinue, nil
	}

	c := node.(*codeLiteral)
	if c.lang != "" {
		w.WriteString(`<pre><code class="language-`)
		w.WriteString(c.lang)
		w.WriteString(`">`)
	} else {
		w.WriteString("<pre><code>")
	}
	for _, line := range c.lines {
		w.Write(util.EscapeHTML([]byte(line)))
		w.WriteString("\n")
	}
	return ast.WalkContinue, nil
}
```

(No comments requirement — remove the doc comment if the project lints
against it; the `lang` constants are `"mermaid"` and `"text"` only, so no
attribute-escaping risk.)

- [ ] **Step 4: Register the renderer in `newMarkdown()`**

Modify `cmd/mdsick/root.go` `newMarkdown()` — add the
`goldmark.WithRendererOptions(...)` call before the parser options, plus
import `"github.com/yuin/goldmark/renderer"`:

```go
func newMarkdown() goldmark.Markdown {
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

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/transformers/ -v && go build ./cmd/mdsick && go test ./... -timeout 60s && go vet ./...`
Expected: PASS everywhere; vet clean.

- [ ] **Step 6: Commit with caveman-commit**

Temp-file pattern from `core-commands` (mktemp → Read → Write →
`jj describe --stdin < <path> && rm <path> && jj new`).

Subject for this task: `feat(mermaid): ledger the code-literal block`

---

## Task 2: `Transform`'s failure changes (insert-then-replace)

**Files:**

- Modify: `internal/transformers/mermaid.go`
- Test: `internal/transformers/mermaid_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/transformers/mermaid_test.go` (existing `convert` helper
from sub-plan 01 stays):

```go
func TestFailureArtifactShape(t *testing.T) {
	html := convert(t, "```mermaid\nFAIL: bad\n```\n")

	margin := `<pre><code class="language-mermaid">1| FAIL: bad
</code></pre>
<pre><code class="language-text">mermaid: parse error near line 2
</code></pre>`
	if !strings.Contains(html, margin) {
		t.Fatalf("artifact missing or misplaced:\n%s", html)
	}
}

func TestArtifactAdjacencyAndPosition(t *testing.T) {
	html := convert(t, "before\n\n```mermaid\nFAIL: bad\n```\n\n```mermaid\ngraph TD; A-->B\n```\n\nafter\n")

	iArtifact := strings.Index(html, `class="language-text"`)
	iImage := strings.Index(html, ".svg")
	if iArtifact < 0 || iImage < 0 {
		t.Fatalf("missing artifact or image:\n%s", html)
	}
	if iArtifact > iImage {
		t.Fatalf("artifact not before the healthy diagram:\n%s", html)
	}
}
```

The `margin` string pins the exact carrier output: numbered surviving fence
with `mermaid` class, immediately followed by the diagnostics fence with
`text` class containing all of mmdc's output.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/transformers/ -run 'TestFailureArtifactShape|TestArtifactAdjacencyAndPosition' -v -timeout 60s`
Expected: FAIL — `artifact missing or misplaced` (the pre-02 behavior keeps
the failing fence as an untouched `<pre><code class="language-mermaid">…`
block without `1| ` prefixes, and emits no `language-text` block).

- [ ] **Step 3: Write the minimal implementation**

Extend the `change` struct with an action discriminator, and rework
`render`'s error path. In `internal/transformers/mermaid.go`:

Replace

```go
type change struct {
	oldChild ast.Node
	newChild ast.Node
	parent   ast.Node
}
```

with:

```go
type change struct {
	action   int
	oldChild ast.Node
	newChild ast.Node
	parent   ast.Node
}

const (
	setNewChild = iota
	insertAfter
)
```

Replace the whole `render` method with:

```go
func (m *Mermaid) render(code []byte, prepare change) {
	var svg bytes.Buffer
	err := mermaid.Compile(bytes.NewBuffer(code), &svg)
	if err != nil {
		var fail *mermaid.Failure
		if errors.As(err, &fail) {
			numbered := &codeLiteral{lang: "mermaid"}
			for i, line := range strings.Split(strings.TrimSuffix(string(code), "\n"), "\n") {
				numbered.lines = append(numbered.lines, fmt.Sprintf("%d| %s", i+1, line))
			}
			numbered.SetBlankPreviousLines(prepare.oldChild.HasBlankPreviousLines())

			logFence := &codeLiteral{lang: "text"}
			for _, line := range strings.Split(strings.TrimSuffix(string(fail.Output), "\n"), "\n") {
				logFence.lines = append(logFence.lines, line)
			}
			logFence.SetBlankPreviousLines(true)

			m.event <- event{change: &change{
				action:   insertAfter,
				parent:   prepare.parent,
				oldChild: prepare.oldChild,
				newChild: logFence,
			}}
			m.event <- event{change: &change{
				action:   setNewChild,
				parent:   prepare.parent,
				oldChild: prepare.oldChild,
				newChild: numbered,
			}}
			return
		}
		m.event <- event{err: err}
		return
	}

	name, err := randHex(32)
	if err != nil {
		m.event <- event{err: err}
		return
	}

	f, err := os.Create(name + ".svg")
	if err != nil {
		m.event <- event{err: err}
		return
	}

	_, err = f.Write(svg.Bytes())
	if err != nil {
		m.event <- event{err: err}
		return
	}

	var svg64 bytes.Buffer
	{
		enc := base64.NewEncoder(base64.StdEncoding, &svg64)
		_, err := enc.Write(svg.Bytes())
		if err != nil {
			m.event <- event{err: err}
			return
		}
	}
	ln := ast.NewLink()
	ln.Destination = []byte(f.Name())
	prepare.newChild = ast.NewImage(ln)
	prepare.newChild.SetAttributeString("style", "max-height: 500px; width: auto;")
	m.event <- event{change: &prepare}
}
```

(Keep the dead `svg64` block byte-for-byte — issue 11's cleanup.)

The `Transform` apply loop gains a switch AND the collector's classification
must become **per-block**, because a failed block now emits *two* events for
one expected slot in `count` — without this, the completion condition
`done+errCount+infraCount == count` fires after ~half the events and the
remaining senders block on the buffered channel forever (a new deadlock
shape, this time in the correct-looking code). Replace the collector loop
body added in sub-plan 01 Task 2 with the version below; the discriminating
fact: `action == insertAfter` events only ever exist for failures, while
`setNewChild` covers both the success image and the failure's numbered fence
(and must only count on the success path — a `codeLiteral` setNewChild event
carries `err == nil` but belongs to an already-counted failure):

```go
			event, ok := <-m.event
			if !ok {
				return
			}

			if event.err != nil {
				var fail *mermaid.Failure
				if errors.As(event.err, &fail) {
					errCount++
				} else {
					mu.Lock()
					infraErrs = append(infraErrs, event.err)
					mu.Unlock()
					infraCount++
				}
				_ = m.bar.Add(1)
			} else if event.change != nil {
				mu.Lock()
				changes = append(changes, event.change)
				mu.Unlock()
				switch event.change.action {
				case insertAfter:
					errCount++ // the failure-resolver event; its setNewChild partner is not counted
				case setNewChild:
					if _, isLiteral := event.change.newChild.(*codeLiteral); !isLiteral {
						done++ // healthy block resolved into an image
					}
				}
				_ = m.bar.Add(1)
			}

			if done+errCount+infraCount == count {
				if errCount+infraCount > 0 {
					m.bar.Describe(fmt.Sprintf("mermaiding done (%d errors)", errCount+infraCount))
				} else {
					m.bar.Describe("mermaiding done")
				}
				return
			}
```

Then extend the final apply loop; replace the
`for _, change := range changes` loop:

```go
	for _, change := range changes {
		switch change.action {
		case insertAfter:
			change.parent.InsertAfter(change.parent, change.oldChild, change.newChild)
		default:
			change.parent.ReplaceChild(change.parent, change.oldChild, change.newChild)
		}
	}
```

Imports gain `"strings"`. The insertion subtleties that must not drift:

- Artifact events carry **no** `err` field: the `err` free path stays
  reserved for infra failures, which skip artifacts entirely (Legend [**q1**]) and
  make the browser-exiting `fatal` joint exit at the end of `Transform`.
- The insert event is enqueued **before** the replace event — the Why
  section's ordering constraint — and the collector counts the failure slot
  on the *insert* event only, so a failed block consumes exactly one slot in
  the `count` accounting as its two events arrive.
- Bar accounting: one `Add(1)` per resolving event (insert events advance
  the bar for failures; setNewChild image events for successes) — one
  resolved block advances the bar once.
- Trailing-newline handling: both `code` and `fail.Output` end with `\n` and
  are `TrimSuffix`ed so `Split` yields exactly the visual line set.
- Non-mmdc errors inside `render` (after a successful `Compile`): none of
  `randHex` / `os.Create` / `f.Write` produce artifacts nor attach changes —
  they send `event{err}` and continue down the infra path of sub-plan 01.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/transformers/ -v -timeout 60s && go build ./cmd/mdsick && go test ./... -timeout 60s`
Expected: all PASS — the earlier `TestMixedDiagamsRender` from sub-plan 01
still passes since `1| FAIL: bad` still contains `FAIL: bad` and the healthy
diagram still references `.svg`.

- [ ] **Step 5: Commit with caveman-commit**

Temp-file pattern (mktemp → Read → Write → `jj describe --stdin < <path> && rm <path> && jj new`).

Subject for this change: `feat(mermaid): render failure artifacts inline`

---

## Handoff

When all tasks are done and committed, tick the 02 line in `00-index.md` in a
separate change (message:
`docs(plans): mark 02-failure-artifact done in issue-10`) via the temp-file
pattern. Sub-plan 03 (`03-gate-and-regression.md`) adds the environment gate
and the subprocess-level regression test that closes the issue; it can run
after this one even when started concurrently.
