# Issue 17 — Reactor Core Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up `internal/reflow` with the reader contract, the splice-assembler, soft-seam paragraph refill, and top-level blank normalization — the substrate every later rule bolts onto.

**Architecture:** `Reflow(io.Reader)` reads all bytes, parses with goldmark (table extension only), computes non-overlapping byte-range edits on the source, and applies them with a single forward splice. Refill only edits inside blocks; normalization only between top-level blocks; everything else stays byte-exact.

**Tech Stack:** Go 1.26, goldmark (`github.com/yuin/goldmark`, `extension.Table`). Notably new dep within the module that already uses goldmark.

**Related:** decisions from `docs/plans/20261126-1335_issue-17_reflow/00-index.md`; AST facts pre-pinned in `internal/reflow/ast/`.

---

### Task 1: Package skeleton, reader contract and error path

**Files:**

- Create: `internal/reflow/reflow.go`
- Create: `internal/reflow/reflow_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/reflow/reflow_test.go`:

```go
package reflow_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/malikbenkirane/mdsick/internal/reflow"
)

// errReader fails on the first Read, exercising Reflow's error path.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read boom") }

func TestReflowReadsAndForwardsSource(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		want            string
		wantErr         string
	}{
		{
			name:    "reader error is returned verbatim",
			input:   "anything",
			wantErr: "read boom",
		},
		{
			name:  "empty input passes through unchanged",
			input: "",
			want:  "",
		},
		{
			name:  "single-line paragraph passes through",
			input: "some paragraph\n",
			want:  "some paragraph\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r io.Reader
			if tt.wantErr != "" {
				r = errReader{}
			} else {
				r = strings.NewReader(tt.input)
			}
			got, err := reflow.Reflow(r)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				if got != nil {
					t.Fatalf("out = %q, want nil", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("out = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/reflow/`

Expected: FAIL — package `reflow` does not exist yet (cannot resolve imports).

- [ ] **Step 3: Write minimal implementation**

`internal/reflow/reflow.go`:

```go
// Package reflow unwraps hard-wrapped markdown prose so `cat doc.md | \
// mdsick-reflow | pbcopy` pastes into GitHub without wrap artifacts.
package reflow

import (
	"io"
)

// Reflow reads source from r and returns the refilled bytes.
// It is the single failure point: read errors flow through verbatim;
// parsing itself cannot fail.
func Reflow(r io.Reader) ([]byte, error) {
	source, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return reflow(source), nil
}
```

Create `internal/reflow/splice.go` as the pass-through core; the assembler and edit
application come in Task 3:

```go
package reflow

// reflow currently passes the source through. Later tasks add parsing,
// edit computation and normalization on top of this callsite.
func reflow(source []byte) []byte {
	return source
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/reflow/`

Expected: PASS.

- [ ] **Step 5: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence from `core-commands`:

1. `mktemp`
2. Read the temp file (required even though empty)
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

---

### Task 2: Parse, apply-assembler and top-level blank-run normalization with POSIX trailing newline

**Files:**

- Create: `internal/reflow/parse.go`
- Create: `internal/reflow/splice.go`
- Modify: `internal/reflow/reflow_test.go`

`internal/reflow/parse.go` — goldmark with the table extension only, exactly the
instance issue 17 mandates (tables are immune; the extension exists to *recognize*
them):

```go
package reflow

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

func parse(source []byte) ast.Node {
	md := goldmark.New(goldmark.WithExtensions(extension.Table))
	return md.Parser().Parse(text.NewReader(source))
}
```

`internal/reflow/splice.go` — the assembler. An edit replaces a source byte range
with new bytes; edits must be sorted and non-overlapping:

```go
package reflow

import (
	"bytes"
	"sort"
)

type edit struct {
	start, stop int
	repl        []byte
}

func apply(source []byte, edits []edit) []byte {
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var out bytes.Buffer
	pos := 0
	for _, e := range edits {
		if e.start > e.stop || e.start < pos {
			panic("reflow: overlapping or unsorted edit")
		}
		out.Write(source[pos:e.start])
		out.Write(e.repl)
		pos = e.stop
	}
	out.Write(source[pos:])
	return out.Bytes()
}
```

And the reflow driver: parse, get the top-level block list, and normalize blank
runs **only** between top-level siblings. A run is normalized to a single blank
line (`\n\n`) when it carries two or more blank lines; runs with none or one are
already normalized and byte-exact and stay untouched. Leading/trailing blank runs
with blocks missing become the final trailing newline; an entirely empty input
stays empty input.

```go
package reflow

import (
	"github.com/yuin/goldmark/ast"
)

func reflow(source []byte) []byte {
	if len(source) == 0 {
		return source
	}
	doc := parse(source)
	var edits []edit
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

// blankCount counts blank lines in region; region starts at a newline and
// spans the gap between two consecutive top-level blocks.
func blankCount(region []byte) int {
	count := 0
	for _, line := range bytes.Split(region[1:], []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			count++
		}
	}
	return count
}

// normalizeTop collapses extra blank lines between top-level blocks.
func normalizeTop(doc ast.Node, source []byte, edits *[]edit) {
	first := doc.FirstChild()
	if first == nil {
		// Source contains no top-level blocks (blank-only text; empty input
		// already returned above): the whole source collapses to "" and
		// reflow's empty-output guard turns it into a single trailing
		// newline.
		*edits = append(*edits, edit{start: 0, stop: len(source), repl: []byte{}})
		return
	}
	prevEnd := -1
	for b := first; b != nil; b = b.NextSibling() {
		s, e := blockRange(b)
		if s < 0 {
			continue // zero-line block edge; leave the source span untouched
		}
		if prevEnd >= 0 {
			region := source[prevEnd:s]
			if blankCount(region) >= 2 {
				*edits = append(*edits, edit{start: prevEnd, stop: s, repl: []byte("\n\n")})
			}
		}
		prevEnd = e
	}
	// Trailing blank runs after the last block collapse to one newline.
	if prevEnd < len(source) {
		*edits = append(*edits, edit{start: prevEnd, stop: len(source), repl: []byte("\n")})
	}
}

// blockRange returns the start and end byte offsets of a block's source lines.
// Line segments exclude the terminating newline; trailing spaces on the last
// line belong to the block.
func blockRange(b ast.Node) (int, int) {
	seg := b.Lines()
	if seg.Len() == 0 {
		return -1, -1
	}
	return seg.At(0).Start, seg.At(seg.Len() - 1).Stop
}
```

Adjust the earlier normalization call in reflow to no-op for the blank-only input:

Modify `internal/reflow/reflow_test.go` — extend the table:

```go
		{
			name:  "two blank lines between top-level blocks collapse to one",
			input: "a\n\n\n\nb\n",
			want:  "a\n\nb\n",
		},
		{
			name:  "exactly one blank line between blocks stays byte-identical",
			input: "a\n\nb\n",
			want:  "a\n\nb\n",
		},
		{
			name:  "adjacent top-level lines without a blank line stay untouched",
			input: "```go\nx\n```\nnext para\n",
			want:  "```go\nx\n```\nnext para\n",
		},
		{
			name:  "single blank line after last block collapses the output trailing newline",
			input: "a\n\n",
			want:  "a\n",
		},
		{
			name:  "blank-only input yields a POSIX newline",
			input: "  \n\n",
			want:  "\n",
		},
		{
			name:  "missing final newline is guaranteed",
			input: "a",
			want:  "a\n",
		},
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/reflow/`

Expected: PASS.

- [ ] **Step 5: Commit with caveman-commit**

per the core-commands sequence as in Task 1.

---

### Task 3: Paragraph refill — soft-seam join with the seam rule

This task wires top-level `Paragraph` blocks into per-block replacements and adds
the seam mechanics. Two helpers in a new file `internal/reflow/prose.go`; note the
prefix scanner is deliberately deferred to sub-plan 02 (here, line content starts
at the first non-whitespace byte, which is correct for top-level paragraphs).

**Files:**

- Create: `internal/reflow/prose.go`
- Modify: `internal/reflow/reflow.go`

- [ ] **Step 1: Write the failing tests**

`Add to internal/reflow/reflow_test.go`:

```go
		{
			name:  "two-line paragraph joins at the soft break with one space",
			input: "hello\nworld\n",
			want:  "hello world\n",
		},
		{
			name:  "three lines join with one space each; last line keeps trailing space",
			input: "a\nb\nc  \n",
			want:  "a b c  \n",
		},
		{
			name:  "leading indent of the first line and per-line indents are trimmed",
			input: "  indent\n  cont\n",
			want:  "indent cont\n",
		},
		{
			name:  "wrapped inline construct joins naturally, Focus inside Emphasis",
			input: "some *em\nbody* text\n",
			want:  "some *em body* text\n",
		},
		{
			name:  "two paragraphs separated by one blank line stay separate",
			input: "first para\nwrapped\n\nsecond\n",
			want:  "first para wrapped\n\nsecond\n",
		},
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/reflow/`

Expected: FAIL with `some *em` still split — the edit layer does nothing yet.

- [ ] **Step 3: Write minimal implementation**

`internal/reflow/prose.go`:

```go
package reflow

import (
	"bytes"

	"github.com/yuin/goldmark/text"
)

// isSpaceTab reports whether b is a space or tab byte.
func isSpaceTab(b byte) bool { return b == ' ' || b == '\t' }

// contentStart returns the index of a line body's first non-whitespace
// byte. At this stage lines carry no marker prefixes; sub-plan 02
// refines this with the marker-run scanner.
func contentStart(line []byte) int {
	for i := range len(line) {
		if !isSpaceTab(line[i]) {
			return i
		}
	}
	return len(line)
}

// trailingWS counts a line body's trailing whitespace run.
func trailingWS(body []byte) int {
	n := 0
	for n < len(body) && isSpaceTab(body[len(body)-1-n]) {
		n++
	}
	return n
}

// lineBody returns a block line's bytes without the terminating newline.
// Block line segments include the newline whenever they are followed by
// another line inside the block; the final line's segment ends at its
// content. Seam logic consumes one consistent view, so it strips the
// newline up front.
func lineBody(source []byte, seg text.Segment) []byte {
	body := source[seg.Start:seg.Stop]
	if len(body) > 0 && body[len(body)-1] == '\n' {
		body = body[:len(body)-1]
	}
	return body
}

// refillBlock joins soft-broken lines of a prose block with one space per
// seam. The line bodies are trimmed left (except line 0, whose leading
// whitespace is excluded by the returned start offset) and right (except
// the last line, which keeps its trailing whitespace byte-exactly per the
// seam rule). Returns the byte offset where the refillable content starts
// and the replacement bytes spliced over [start, lastLineStop).
func refillBlock(source []byte, lines *text.Segments) (start int, repl []byte) {
	first := lines.At(0)
	start = first.Start + contentStart(lineBody(source, first))
	var out bytes.Buffer
	for i := 0; i < lines.Len(); i++ {
		body := lineBody(source, lines.At(i))
		if i > 0 {
			out.WriteByte(' ')
			body = body[contentStart(body):]
		}
		if i < lines.Len()-1 {
			body = body[:len(body)-trailingWS(body)]
		}
		out.Write(body)
	}
	return start, out.Bytes()
}
```

And wire it into `reflow.go` — inside `normalizeTop`'s block loop, every
top-level `Paragraph` gets a whole-block replacement; the seam joins are
computed inside `refillBlock`, and a no-op `bytes.Equal` guard leaves
single-line paragraphs byte-exact with no edit at all. The full loop body
becomes:

```go
		s, e := blockRange(b)
		if s < 0 {
			continue // zero-line block edge; leave the source span untouched
		}
		if b.Kind() == ast.KindParagraph {
			start, repl := refillBlock(source, b.Lines())
			if !bytes.Equal(repl, source[start:e]) {
				*edits = append(*edits, edit{start: start, stop: e, repl: repl})
			}
		}
		if prevEnd >= 0 {
			region := source[prevEnd:s]
			if blankCount(region) >= 2 {
				*edits = append(*edits, edit{start: prevEnd, stop: s, repl: []byte("\n\n")})
			}
		}
		prevEnd = e
```

Import additions in `reflow.go`: `bytes` and `github.com/yuin/goldmark/ast`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/reflow/`

Expected: PASS.

- [ ] **Step 5: Commit with caveman-commit**

per the core-commands sequence as in Task 1.
