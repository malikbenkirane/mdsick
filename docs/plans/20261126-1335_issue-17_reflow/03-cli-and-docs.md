# Issue 17 — CLI and Docs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the user-facing side: `cmd/mdsick-reflow` (TTY refusal, stderr routing, stdout kept clean for `pbcopy`), its cmd-level test, the fact-corrected `docs/issues/17.md` rewrite, and the README usage line.

**Architecture:** The cmd is a thin wrapper over `reflow.Reflow`: a testable `runStd` takes explicit readers/writers and an `isTTY` flag so the refusals and the stdout-clean contract are testable without faking char devices; `main` wires `os.Stdin`/`os.Stdout`/`os.Stderr` and exits 1 on error.

**Tech Stack:** Go 1.26 stdlib only.

**Related:** runs after `01-core-engine.md` and `02-containers-and-immunity.md`.

---

### Task 1: `cmd/mdsick-reflow` with testable core

**Files:**

- Create: `cmd/mdsick-reflow/main.go`
- Create: `cmd/mdsick-reflow/main_test.go`

- [ ] **Step 1: Write the failing tests**

`cmd/mdsick-reflow/main_test.go`:

```go
package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRunStdRefusesTTY(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runStd(strings.NewReader("# doc\n"), &stdout, &stderr, true)
	if err == nil {
		t.Fatal("want an error for terminal stdin")
	}
	if !strings.Contains(err.Error(), "cat doc.md | mdsick-reflow") {
		t.Errorf("err = %q, want usage hint", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout must stay clean for pbcopy, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("runStd must not print to stderr; main routes errors itself")
	}
}

func TestRunStdPipesThroughReflow(t *testing.T) {
	var stdout bytes.Buffer
	err := runStd(strings.NewReader("hello\nworld\n"), &stdout, &bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := stdout.String(); got != "hello world\n" {
		t.Errorf("out = %q, want %q", got, "hello world\n")
	}
}

// failingReader exercises Reflow's error path through runStd.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read boom") }

func TestRunStdPropagatesReadErrors(t *testing.T) {
	var stdout bytes.Buffer
	err := runStd(failingReader{}, &stdout, &bytes.Buffer{}, false)
	if err == nil || err.Error() != "read boom" {
		t.Fatalf("err = %v, want read boom", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout must stay clean on error, got %q", stdout.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/mdsick-reflow/`

Expected: FAIL — the package does not exist yet.

- [ ] **Step 3: Write the implementation**

`cmd/mdsick-reflow/main.go`:

```go
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/malikbenkirane/mdsick/internal/reflow"
)

// runStd is the testable core. It refuses piped-less stdin and keeps
// stdout reserved for refilled bytes; every diagnostic goes through the
// returned error, which main routes to stderr.
func runStd(input io.Reader, stdout, stderr io.Writer, isTTY bool) error {
	if isTTY {
		return errors.New("mdsick-reflow reads from stdin; pipe a markdown file in, e.g. `cat doc.md | mdsick-reflow`")
	}
	_ = stderr // reserved: runStd itself never writes diagnostics
	out, err := reflow.Reflow(input)
	if err != nil {
		return err
	}
	_, err = stdout.Write(out)
	return err
}

func stdinIsTerminal() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

func main() {
	if err := runStd(os.Stdin, os.Stdout, os.Stderr, stdinIsTerminal()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Run test to verify it passes and the binary builds**

Run: `go test ./cmd/mdsick-reflow/ && go build ./cmd/mdsick-reflow`

Expected: PASS; binary builds.

Smoke-test by hand (requires a real terminal to see the refusal):

```sh
echo -e "a\nb" | (cd . && go run ./cmd/mdsick-reflow)
```

Expected: `a b` on stdout.

- [ ] **Step 5: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence from `core-commands`:

1. `mktemp`
2. Read the temp file (required even though empty)
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

---

### Task 2: Fact-corrected `docs/issues/17.md` rewrite

The original issue text predates the dump tests; several statements are now known
wrong. Rewrite the file to the decisions recorded in
`docs/plans/20261126-1335_issue-17_reflow/00-index.md`:

**Files:**

- Modify: `docs/issues/17.md`

- [ ] **Step 1: Replace the file with the corrected specification**

```markdown
# 17. `mdsick-reflow`: un-wrap markdown prose for pasting into GitHub

GitHub comment boxes (issues, PRs, reviews) render a single newline as a
hard break. A markdown file written against an 80-column terminal is
therefore full of wrap artifacts: every soft newline the author used for
wrapping becomes a visible break on GitHub. `mdsick-reflow` joins those
wrap lines back, so `cat doc.md | mdsick-reflow | pbcopy` produces text
that pastes to GitHub as if it had never been wrapped.

The decisions below supersede the first draft of this issue: they were
settled against facts pinned by the AST-shape tests in
`internal/reflow/` (see the latest dump-verified structural facts).

## Scope

- Stdin only, no args, no flags. Refuses (errors to stderr, exit 1)
  when stdin is a TTY, with a usage hint
  (`cat doc.md | mdsick-reflow`). Output goes to stdout, which stays
  byte-clean on both success and error; errors never touch stdout.
- The output always ends with exactly one newline (POSIX convention),
  even when the input lacked one. Empty input stays empty; input made
  only of blank lines yields a single newline.
- Packaging: thin `cmd/mdsick-reflow`; logic in `internal/reflow`
  exposing `Reflow(r io.Reader) ([]byte, error)` — the package owns the
  read and is the single failure point; goldmark parsing cannot fail.

## Approach: miner, not re-serializer

Parse with a plain `goldmark.New()` plus only the table extension
(`extension.Table`). No renderer runs; the output is the original
source with byte-range edits spliced in.

- Prose blocks — where refill applies — are `Paragraph` nodes and
  `TextBlock` nodes (the shape tight list item text takes), anywhere in
  the tree. Node type wins over ancestry: a wrapped list item's text
  refills, and so does a paragraph inside a blockquote inside a list.
- Every other block type is frame or immune content and is left
  byte-identical: `Heading` (including setext form, whose wrapped lines
  the parser folds into the heading), `ThematicBreak`, `FencedCodeBlock`,
  `CodeBlock` (indented), `HTMLBlock`, `Table`, and everything with a
  `Table` in its ancestry (table cells must stay untouched).
- Seam rule: trim leading/trailing whitespace of each joined segment,
  join with exactly one space. The last line keeps its trailing
  whitespace and the block keeps its own line content byte-exact
  otherwise.
- Wrapped inline constructs need no special handling: the miner joins
  at the soft break regardless of what inline node spans the seam.

## Hard breaks

CommonMark pads the hard-break detection onto the source bytes: trailing
two spaces (or a backslash) immediately before a newline. The seams are
classified from those bytes:

- A two-space trailing run becomes a single `\` and the following line is
  emitted byte-exactly (prefix and indentation preserved) — the
  hard-break form survives editors that trim trailing whitespace.
- A line already ending with `\` is untouched; appending another backslash
  would corrupt escaped content (a literal backslash inside a two-space
  run would become a single escaped backslash). The whole seam stays raw.

## Blockquote

The refill uses the common rotation: the first line's marker run
(already outside the computed edit range) is re-emitted byte-exactly;
lazy lines — which goldmark folds into the quote's paragraph — join with
it, and the joined output stays parse-identical (reflow is idempotent).
A hard break inside a quote re-emits the per-line prefix intact on the
continuation line.

## Lists

List item text refills like any prose block. The item marker bytes
(`-`, `+`, `*`, `1.`, `1)`, and their indent) are preserved byte-exactly;
ordered markers are never renumbered. A line starting with something that
looks like a marker but is not one (e.g. emphasis `*nice*`) is content,
not a marker — the prefix scanner requires whitespace-plus-nonempty-remainder
after the marker characters.

## Blank-line runs

Top-level blank runs with two or more blank lines collapse to a single
blank line; runs with none or one are byte-exact. Blank runs inside
lists and blockquotes survive byte-exactly: they are parse-affecting
(loose vs tight items), not cosmetics.

## Tables (explicit non-goal)

GFM table rows are atomic: a newline ends a row, so a row wrapped at
80 columns already renders as a broken table on GitHub, and joining
between rows would delete a row from the output. The table extension is
enabled **only** so the joiner recognizes tables as immune. Long cell
text is not fixable by reflow; fix the sentence instead.

## Tests

Table-driven in `internal/reflow` (one case per rule), an AST-shape suite
in `internal/reflow/ast` pinning the parser facts, and a cmd-level test
in `cmd/mdsick-reflow` guarding the TTY refusal and the stdout-clean
contract. Idempotence is an explicit test: feeding reflow's output back
through reflow changes nothing.

Done bar: `go test ./...`, `go vet ./...`,
`golangci-lint run ./...` (only the accepted QF1012 warning).
```

- [ ] **Step 2: Verify the file on disk matches**

Run: `go run ./cmd/mdsick-reflow < docs/issues/17.md 2>&1 | head -2`

Alternative quick check: `grep -c 17.md docs/issues/*.md` style sanity — really, just re-read the file to confirm the whole replacement landed.

- [ ] **Step 3: Commit with caveman-commit**

per the core-commands sequence.

---

### Task 3: README usage line

**Files:**

- Modify: `README.md`

- [ ] **Step 1: Add the reflow one-liner to the Usage section**

Insert immediately after the block under `## Usage` (which currently begins
with `cat README.md | mdsick`), as its own fenced block plus a sentence:

Insert immediately after the fenced block under `## Usage` (which currently begins
with `cat README.md | mdsick`). Add this sentence and indented block (indented so
the fence nesting stays valid in this plan file; in `README.md` write it as a
normal fenced `sh` block):

    ```sh
    cat doc.md | mdsick-reflow | pbcopy
    ```

surrounded by this prose, one blank line before and after the fenced block:

Exactly this sentence, directly below the fenced block:

    Refills hard-wrapped Markdown so single newlines don't turn into hard
    breaks when pasted into GitHub comment boxes.

And this is the complete addition to write into README.md (the fenced block
dedented to a real `sh` fence when applied), placed directly below the existing
`cat README.md | mdsick` fenced block:

    ```sh
    cat doc.md | mdsick-reflow | pbcopy
    ```

    Refills hard-wrapped Markdown so single newlines don't turn into hard
    breaks when pasted into GitHub comment boxes.

- [ ] **Step 2: Verify**

Run: `go test ./... && go vet ./... && golangci-lint run ./...`

Expected: PASS; only the accepted QF1012 warning remains.

- [ ] **Step 3: Commit with caveman-commit**

per the core-commands sequence.
