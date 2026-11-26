# PR: `mdsick-reflow` — un-wrap markdown prose for pasting into GitHub (issue 17)

## Summary

This PR ships a new companion CLI, `mdsick-reflow`, that joins hard-wrapped
markdown prose back into single lines, so `cat doc.md | mdsick-reflow | pbcopy`
produces text that pastes into GitHub comment boxes as if it had never been
wrapped. GitHub renders a single newline as a hard break; every wrap newline an
author used at 80 columns becomes a visible break on GitHub today. Refilling is
surgical: the output is the **original source with byte-range edits spliced in**
(a miner, not a re-serializer), so everything outside prose stays byte-exact.

## What changed

### 1. `internal/reflow` — the engine
`Reflow(r io.Reader) ([]byte, error)` reads all input, parses once with
goldmark (table extension only) and applies **non-overlapping, sorted byte-range
edits** with a single forward splice (`splice.go`). Read errors flow through
verbatim; parsing itself cannot fail.

- **Prose refill** (`prose.go`): a seam loop classifies each line boundary from
  the source bytes:
  - *soft break* → trailing whitespace and the next line's marker run are
    trimmed, one space joins;
  - *hard break* (trailing two-plus-space run) → collapses to ` \` and the
    continuation line keeps its prefix/indentation byte-exactly;
  - *raw seam* (line already ending in a backslash) → the seam stays as written,
    so escaped content and literal backslashes inside two-space runs are never
    mangled.
- **Marker-run scanner** (`prefix.go`): whitespace, `>` quote markers and at
  most one list marker (`-`, `+`, `*`, `NN.`, `NN)`) are recognized; a marker
  must be followed by whitespace and a non-empty remainder, so emphasis
  lookalikes like `*em*` stay content.
- **Prose anywhere** (`reflow.go` `collectProse`): every `Paragraph` and
  `TextBlock` (tight list item text) in the tree refills; anything with a
  `Table` ancestor is immune, matching the table non-goal.
- **Immunity is automatic**: blocks without edits splice through untouched —
  headings (incl. setext), fenced/indented code, HTML blocks, tables, thematic
  breaks, and all framing bytes (`>`, item markers, indents).
- **Blank-run normalization** (`normalize.go`): runs of two or more blank lines
  between top-level blocks collapse to a single blank line, and the output
  always ends with exactly one newline (POSIX convention; empty input stays
  empty; blank-only input yields a single `\n`). Blank runs inside lists and
  quotes survive — they are parse-affecting (loose vs tight), not cosmetics.

### 2. `cmd/mdsick-reflow` — the thin wrapper
Stdin only, no args, no flags. Refuses a TTY stdin with a usage hint (`cat
doc.md | mdsick-reflow`), keeps stdout reserved for refilled bytes (diagnostics
go to stderr via a returned error; exit 1 on error). `runStd` takes explicit
readers/writers and an `isTTY` flag so the contracts are testable without
faking char devices.

### 3. Docs and packaging
- `docs/issues/17.md` rewritten to the dump-verified decisions: node type
  (Paragraph/TextBlock) wins over ancestry, byte-wise hard-break seams,
  preserved marker runs, collapse rules — superseding the first draft.
- README gains the `cat doc.md | mdsick-reflow | pbcopy` usage line.

### 4. Tests
- `internal/reflow/reflow_test.go`: one table case per rule, including
  idempotence (reflowing reflowed output is a no-op).
- `internal/reflow/ast`: pre-existing AST-shape suite pinning the
  parser facts the implementation tracks against.
- `cmd/mdsick-reflow/main_test.go`: TTY refusal, pipe-through, stdout-clean on
  error.
- Pre-existing goldmark mermaid failure semantics unchanged.

## Behavior summary

| Input | Output |
|---|---|
| `hello\nworld\n` | `hello world\n` |
| `hello  \nworld\n` (trailing two spaces) | `hello \\\nworld\n` (hard break survives) |
| `hello \\\nworld\n` (backslash) | unchanged byte-for-byte |
| `> wrapping one\n> wrapping two\n` | `> wrapping one wrapping two\n` |
| `- item\n  wrapped\n` | `- item wrapped\n` (marker/indent exact) |
| GFM table, fenced code, HTML block, setext heading | unchanged byte-for-byte |
| `a\n\n\n\nb\n` | `a\n\nb\n` |
| `cat doc.md \| mdsick-reflow` with TTY stdin | usage error to stderr, exit 1, stdout clean |

## Verification

- `go test ./...` — all packages pass
- `go vet ./...` — clean
- `golangci-lint run ./...` — only the long-accepted `QF1012` warning remains
- Hand smoke test: `printf 'a\nb\n' | go run ./cmd/mdsick-reflow` → `a b`

## Deliberate deviations from the plan

Documented in [deviations.md](./deviations.md). In short: goldmark trims
leading/trailing whitespace off prose line segments (indent consumed backward
with a line-start guard; the tail keeps the last line's trailing whitespace),
fence closers / setext underlines / thematic dashes live outside `Lines()` so
blank-run collapse only fires on all-whitespace gaps, raw seams must win over
hard-break detection (the plan's ordering doubled a literal backslash), inline
nodes panic on `Lines()` (walk guards), and `KindTable` lives in
`goldmark/extension/ast`.

## Issue

Closes #17.
