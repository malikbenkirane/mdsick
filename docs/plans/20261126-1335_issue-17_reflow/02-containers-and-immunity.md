# Issue 17 — Containers and Immunity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Teach the substrate the block-structure rules: prefix-run handling, prose anywhere in the tree (with table exclusion), blockquote marker semantics (including lazy lines and the `>>`-fold fact), hard-break seams in every container, ordered-list bytes, and byte-exact immunity for everything else.

**Architecture:** Every rule lands as either (a) a generalization of `contentStart` (the marker-run scanner `prefixEnd`), (b) a rewrite of `refillBlock` from a straight-trim loop into a seam loop that copies raw source bytes wherever a seam is not a pure soft join, or (c) the recursion scope of prose collection. Immunity stays automatic: a block that receives no edit is spliced through byte-exactly.

**Tech Stack:** Go 1.26, goldmark AST kinds (`ast.KindParagraph`, `ast.KindTextBlock`, `ast.KindTable`).

**Related:** runs after `01-core-engine.md`; key facts were pinned by `internal/reflow/ast/` tests: tight item text is a `TextBlock`, blank lines flip items loose, a lighter quote marker line merges into the inner paragraph (lazy continuation), tables decompose row-per-line, hard breaks are a flag on the preceding `Text` (and, byte-wise, trailing whitespace before a newline).

---

### Task 4: Marker-run scanner and prose anywhere in the tree

**Files:**

- Create: `internal/reflow/prefix.go`
- Modify: `internal/reflow/reflow.go` (prose collection over the whole tree)
- Modify: `internal/reflow/reflow_test.go`

- [ ] **Step 1: Write the failing tests**

`Add to internal/reflow/reflow_test.go`:

```go
		{
			name: "wrapped lines inside loose and tight list items refill; markers and indents preserved byte-exactly",
			input: "- some list item\n  paragraph\n  - some sublist item\n    to reflow\n  - another item\n- one more item\n",
			want:  "- some list item paragraph\n  - some sublist item to reflow\n  - another item\n- one more item\n",
		},
		{
			name: "ordered markers survive byte-exactly, no renumbering",
			input: "1. one\n   wrapped continuation\n10. ten stays ten\n",
			want:  "1. one wrapped continuation\n10. ten stays ten\n",
		},
		{
			name: "list marker lookalike text inside an item is not treated as a marker",
			input: "- a**bold* start\n  continues here\n",
			want:  "- a**bold* start continues here\n",
		},
		{
			name: "a wrapped GFM table row is immune, byte-exactly",
			input: "| a | b |\n| - | - |\n| one wrapped over\n| lines | |\n",
			want:  "| a | b |\n| - | - |\n| one wrapped over\n| lines | |\n",
		},
```

(The wrapped-table fixture stays byte-identical: github breaks such tables
regardless; reflow refuses to touch anything with a `Table` ancestor and
never joins between rows.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/reflow/`

Expected: FAIL — list items are not collected, so item text stays wrapped.

- [ ] **Step 3: Write the implementation**

`internal/reflow/prefix.go`:

```go
package reflow

// isSpaceTab reports whether b is a space or tab byte.
func isSpaceTab(b byte) bool { return b == ' ' || b == '\t' }

// isDigit reports whether b is an ASCII digit byte.
func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// isListMarker reports whether line[i:] starts a list marker token:
// one of '-', '+' , '*' , or a digit run followed by '.' or ')'.
// In both cases a whitespace byte must follow the marker and a
// non-empty remainder must exist after that whitespace, so an inline
// '*' at line start is never mistaken for a marker.
func isListMarker(line []byte, i int) bool {
	switch line[i] {
	case '-', '+', '*':
		return i+1 < len(line) && isSpaceTab(line[i+1]) &&
			hasContentAfter(line, i+1)
	}
	if isDigit(line[i]) {
		j := i
		for j < len(line) && isDigit(line[j]) {
			j++
		}
		if j < len(line) && (line[j] == '.' || line[j] == ')') {
			return j+1 < len(line) && isSpaceTab(line[j+1]) &&
				hasContentAfter(line, j+1)
		}
	}
	return false
}

// listMarkerEnd returns the offset one past the marker token: its
// characters only; the following whitespace is consumed by prefixEnd's
// generic whitespace case.
func listMarkerEnd(line []byte, i int) int {
	if isDigit(line[i]) {
		j := i
		for j < len(line) && isDigit(line[j]) {
			j++
		}
		return j + 1 // digits plus one of '.', ')'
	}
	return i + 1 // '-', '+' or '*'
}

// hasContentAfter reports whether a non-whitespace byte exists after
// line[from:]; guards the required non-empty remainder.
func hasContentAfter(line []byte, from int) bool {
	for from < len(line) {
		if !isSpaceTab(line[from]) {
			return true
		}
		from++
	}
	return false
}

// prefixEnd returns the offset at which a source line's marker run ends
// and the line's content begins, after scanning any sequence of
// whitespace, '>' quote markers and at most one list marker. Returns -1
// when the line has no content after the run (caller treats it as blank).
func prefixEnd(line []byte) int {
	i := 0
	markerSeen := false
	for i < len(line) {
		switch {
		case isSpaceTab(line[i]):
			i++
		case line[i] == '>':
			i++
		case !markerSeen && isListMarker(line, i):
			i = listMarkerEnd(line, i)
			markerSeen = true
		default:
			return i
		}
	}
	return -1
}
```

Then upgrade `contentStart` in `internal/reflow/prose.go` so every seam consumer
is prefix-aware in one move (same slice-based signature as sub-plan 01, new body):

```go
// contentStart returns the index of a line body's first content byte:
// past its marker run, then past remaining whitespace. Returns
// len(line) when the line carries no content.
func contentStart(line []byte) int {
	if pe := prefixEnd(line); pe >= 0 {
		return pe
	}
	return len(line)
}
```

Finally, generalize prose collection in `internal/reflow/reflow.go`. Replace the
per-paragraph branch in `normalizeTop`'s loop with a separate whole-tree
collection pass, called before `normalizeTop` (same file):

```go
package reflow

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
)

func reflow(source []byte) []byte {
	if len(source) == 0 {
		return source
	}
	doc := parse(source)
	var edits []edit
	collectProse(doc, source, &edits)
	normalizeTop(doc, source, &edits)
	out := apply(source, edits)
	if len(out) == 0 {
		// Input carried only blank lines: POSIX convention yields a single
		// trailing newline; empty input exited above.
		return []byte{'\n'}
	}
	if out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return out
}

// collectProse records one whole-block replacement edit for every
// multi-line refillable leaf block anywhere in the tree. A block is
// refillable when it is a Paragraph or — under a ListItem, where tight
// item text lives — a TextBlock, and no Table anywhere in its ancestry
// (table cells must stay untouched).
func collectProse(n ast.Node, source []byte, edits *[]edit) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindParagraph, ast.KindTextBlock:
			if !hasTableAncestor(c) {
				addRefillEdit(c, source, edits)
			}
		default:
			collectProse(c, source, edits)
		}
	}
}

func hasTableAncestor(n ast.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == ast.KindTable {
			return true
		}
	}
	return false
}

func addRefillEdit(b ast.Node, source []byte, edits *[]edit) {
	s, e := blockRange(b)
	if s < 0 || e <= s {
		return
	}
	start, repl := refillBlock(source, b.Lines())
	if !bytes.Equal(repl, source[s:e]) {
		*edits = append(*edits, edit{start: start, stop: e, repl: repl})
	}
}
```

Note that `addRefillEdit` computes its own `s, e` from `blockRange` (a leaf block
might not be reachable from `normalizeTop`'s top-level list — that is the point of
the tree walk). Keep the normalization loop exactly as Task 3 left it.

Upgrade `blockRange` in `internal/reflow/splice.go` too: `List` and `Blockquote`
nodes carry no line segments of their own (dump-verified), so normalization
between top-level containers would lose their byte span. Compute the container's
span from its descendants instead — semantics unchanged for leaf blocks that
already carry their own lines:

```go
// blockRange returns the start and end byte offsets covering a block's
// source lines. A container node's span comes from its descendants,
// since List and Blockquote carry no line segments of their own.
func blockRange(b ast.Node) (int, int) {
	min, max := -1, -1
	var visit func(n ast.Node)
	visit = func(n ast.Node) {
		if segs := n.Lines(); segs.Len() != 0 {
			if first := segs.At(0).Start; min < 0 || first < min {
				min = first
			}
			if last := segs.At(segs.Len() - 1).Stop; last > max {
				max = last
			}
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			visit(c)
		}
	}
	visit(b)
	if min < 0 {
		return -1, -1
	}
	return min, max
}
```

Footnote: `edit` ranges of nested prose blocks sit strictly inside their
container's span and never overlap `normalizeTop` ranges between top-level
siblings, so `apply`'s sort-by-start order holds regardless of edit kind.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/reflow/`

Expected: PASS. If the marker-lookalike case fails, re-read `isListMarker`'s
`hasContentAfter` guard; the `**bold*` inline must never be consumed (it needs a
whitespace after the bare marker and the `*bold*` child has none).

- [ ] **Step 5: Commit with caveman-commit**

per the core-commands sequence.

---

### Task 5: Prose inside blockquotes — strip the run, refill the remainder, one re-emitted marker

**Files:**

- Modify: `internal/reflow/reflow_test.go`

No implementation change is expected in this task: the generic machinery already
strips prefix runs (`contentStart`), joins soft seams, and re-emits line 0's raw
prefix by leaving it outside the edit range. These tests exercise the quote
rules (Q1.2, Q4.2) for the first time. The two uncertain shapes were already
verified against the parser with the AST dump: `> x / >> y` is byte-identical
(the `>>` line opens a fresh inner quote, not a lazy continuation), and so is
`> first-wrapped / >\n> second` (the bare-marker line separates quote
paragraphs and stays raw).

- [ ] **Step 1: Write the failing-proving tests**

Run first to know what actually happens; here are the expected inputs, all single-q
quote fixtures only for now:

```go
		{
			name: "quoted paragraph refills underneath one re-emitted marker run",
			input: "> wrapping line one\n> wrapping line two\n",
			want:  "> wrapping line one wrapping line two\n",
		},
		{
			name: "lazy continuation without its own marker joins the quote",
			input: "> quoted content\nlazy tail here\n",
			want:  "> quoted content lazy tail here\n",
		},
		{
			name: "deeper-opened quote with lazy line refills into the inner paragraph",
			input: ">> deep quote\n> plain continuation\n",
			want:  ">> deep quote plain continuation\n",
		},
		{
			name: "two quote paragraphs separated by a blank quote line stay separate",
			input: "> first para\n> wrapped\n>\n> second\n",
			want:  "> first para wrapped\n>\n> second\n",
		},
		{
			name: "nested quote marker atoms strip and stack from the outermost",
			input: "> outer block quote\n>> inner nested\n",
			want:  "> outer block quote\n>> inner nested\n",
		},
```

Both separator-sensitive expectations above are verified against the parser
(the AST dump in `internal/reflow/ast`); details are in the task intro above.

Add idempotence as its own fixture — refilling an already-refilled form must
not change bytes:

```go
		{
			name:  "reflowing an already-refilled quote is a no-op (idempotence)",
			input: ">> deep quote plain continuation\n",
			want:  ">> deep quote plain continuation\n",
		},
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./internal/reflow/`

Expected: PASS.

- [ ] **Step 3: Run the whole suite once**

Run: `go test ./... && go vet ./... && golangci-lint run ./internal/reflow/...`

Expected: PASS; clean lint; sub-plan 01 cases unchanged.

- [ ] **Step 4: Run the rest of the suite**

Run: `go test ./...`

Expected: PASS; sub-plan 01 and 02 green.

- [ ] **Step 5: Commit with caveman-commit**

per the core-commands sequence.

---

### Task 6: Hard-break seams — backslash re-emission with raw preservation

**Files:**

- Modify: `internal/reflow/prose.go` (rewrite `refillBlock` as a seam loop)
- Modify: `internal/reflow/reflow_test.go`

- [ ] **Step 1: Write the failing tests**

```go
		{
			name:  "two-space wrap line becomes a hard break re-emitted as backslash",
			input: "hello  \nworld\n",
			want:  "hello \\\nworld\n",
		},
		{
			name:  "a line already ending with a backslash is left untouched",
			input: "hello \\\nworld\n",
			want:  "hello \\\nworld\n",
		},
		{
			name:  "literal backslash inside a two-space wrap line keeps the seam raw",
			input: "a\\  \nnext\nwide\n",
			want:  "a\\  \nnext wide\n",
		},
		{
			name:  "hard break inside a quote keeps the marker on both sides",
			input: "> wrapping one  \n> after the break\n",
			want:  "> wrapping one \\\n> after the break\n",
		},
		{
			name:  "hard break inside a list item keeps the continuation indent raw",
			input: "- item before  \n  item after\n",
			want:  "- item before \\\n  item after\n",
		},
```

The literal-backslash cases keep their raw bytes: a backslash immediately
before the newline is already the canonical hard-break form (Q1.1), and a
backslash inside a two-space run is parser-verified literal content followed
by a hard break (dump-verified: `Text "a\\"` carries the hard flag) —
appending a backslash there would produce `\\`, an escaped backslash, a parse
change. So both seams are left fully untouched.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/reflow/`

Expected: FAIL — current soft-seam code joins the two-space line as one.

- [ ] **Step 3: Write the implementation**

Rewrite `refillBlock` in `internal/reflow/prose.go` as a seam loop.
Position arithmetic: `pos` is the copy cursor into `source`; per seam the
previous line's body is split at the start of its trailing whitespace run
(`wsStart`), and `csNext` is the absolute offset of the next line's first
content byte. Raw and hard seams continue at `next.Start` so the next line
keeps its own prefix and indentation byte-exactly; only soft seams strip the
next line's prefix:

```go
// refillBlock joins the soft-broken lines of a prose block with one space
// per seam. Seam classification is decided from the bytes of the line
// before the newline:
//
//   - a backslash as the last content byte: the seam stays raw (it is
//     already the canonical hard-break form, or the backslash is literal
//     content inside a two-space run);
//   - a trailing whitespace run of two or more: the run collapses to one
//     backslash terminating the line (`\\\n`), and the following line
//     keeps its own prefix and content byte-exactly;
//   - otherwise: a pure soft break — trailing whitespace and the next
//     line's marker run and indentation are trimmed, one space joins.
//
// Returns the offset where the refillable content starts and the
// replacement bytes to splice over [start, lastLineStop).
func refillBlock(source []byte, lines *text.Segments) (start int, repl []byte) {
	first := lines.At(0)
	last := lines.At(lines.Len() - 1)
	start = first.Start + contentStart(lineBody(source, first))
	var out bytes.Buffer
	pos := start
	for i := 0; i < lines.Len()-1; i++ {
		prev, next := lines.At(i), lines.At(i+1)
		prevBody := lineBody(source, prev)
		pw := trailingWS(prevBody)
		wsStart := prev.Start + len(prevBody) - pw
		keep := prevBody[:len(prevBody)-pw]
		csNext := next.Start + contentStart(lineBody(source, next))
		switch {
		case pw < len(keep) && keep[len(keep)-1] == '\\':
			out.Write(source[pos:prev.Stop])
			pos = next.Start
		case pw >= 2:
			out.Write(source[pos:wsStart])
			out.WriteByte('\\')
			out.WriteByte('\n')
			pos = next.Start
		default:
			out.Write(source[pos:wsStart])
			out.WriteByte(' ')
			pos = csNext
		}
	}
	out.Write(source[pos:last.Stop])
	return start, out.Bytes()
}
```

The final loop tail copies the last line's body (its preserved trailing
whitespace included) byte-exactly.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/reflow/`

Expected: PASS; then `go test ./...` and confirm sub-plan 01 and 02 remain green.

- [ ] **Step 5: Commit with caveman-commit**

per the core-commands sequence.

---

### Task 7: Immunity and blank-run scope under containers

**Files:**

- Modify: `internal/reflow/reflow_test.go`

No expected implementation changes: this task pins the parse-account ends (frame
blocks byte-identical, blank runs inside lists and quotes preserved) that earlier
tasks only relied on for a couple of shapes.

- [ ] **Step 1: Write the proving tests**

```go
		{
			name: "a loose list keeps its blank line: too-narrow normalization is out of scope",
			input: "- mine\n\n- two\n",
			want:  "- mine\n\n- two\n",
		},
		{
			name:  "a blank quote line inside a blockquote is preserved byte-exactly",
			input: "> wrap inside\n> more\n\n> second\n",
			want:  "> wrap inside more\n\n> second\n",
		},
		{
			name:  "fenced code, indented code, html block and thematic break pass through untouched",
			input: "```go\nx\n```\n\n    indented\n\n<div>\nraw\n</div>\n\n---\n\n# Heading setext-free\n",
			want:  "```go\nx\n```\n\n    indented\n\n<div>\nraw\n</div>\n\n---\n\n# Heading setext-free\n",
		},
		{
			name:  "a setext heading underlines its wrapped lines, left untouched",
			input: "Heading\nsecond\n===\n",
			want:  "Heading\nsecond\n===\n",
		},
		{
			name:  "a wrapped GFM table row is immune, byte-exactly",
			input: "| a | b |\n| - | - |\n| very long cell content that wraps\n| across |\n",
			want:  "| a | b |\n| - | - |\n| very long cell content that wraps\n| across |\n",
		},
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./internal/reflow/`

Expected: PASS — immunity is by construction. If the fenced-code test fails,
the likely cause is the paragraph detector treating the code lines as prose;
re-check `collectProse`'s kind switch.

- [ ] **Step 3: Full suite once more, in the done-bar shape**

Run: `go test ./... && go vet ./... && golangci-lint run ./internal/reflow/...`

Expected: PASS; lint clean for the new package.

- [ ] **Step 4: Commit with caveman-commit**

per the core-commands sequence.
