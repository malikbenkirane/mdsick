# PR draft: warnings table on the `mdsick-issues` index (issue 9, post-grill)

Plan: `docs/plans/20261125T1653_issue-9_mdsick-issues/00-plan.md`

Replaces the `<pre>`-style warnings block on the `mdsick-issues` landing
page with a real HTML table, per the grill-settled decisions.

## What changed

### 1. Warnings table (`feat: fold duplicate warnings`, `feat: warnings table on index`)

The old block — `<h2>Warnings</h2>` plus one escaped
`Source: Ref — unresolvable` line per warning inside `<pre>` — is gone.
Only rendered when there is at least one warning; with zero warnings the
heading and the whole block disappear (presence is the signal).

New writer (`cmd/mdsick-issues/index.go`, `writeWarningsTable`):

- two columns, **File | Ref**, `markdown-body` table styling only
  (no bespoke alert styling);
- **one row per source file** that has broken references, sorted by
  file name (natural, ascending);
- the File cell links the file's flat page
  (`<td><a href="9.html">#9</a></td>`, falling back to plain text
  if the base name is not all digits — the `issues.List` contract
  already guarantees digit bases, so the fallback is defensive);
- the Ref cell holds one `<code>` element per distinct broken reference,
  one per line, never linked (the target does not resolve).

Rendering as produced by the smoke run against this repo:

```html
<tr><td><a href="9.html">#9</a></td><td>
    <code>#4</code>
    <code>#8</code>
    <code>#22</code>
    <code>#30</code>
    <code>#33</code>
    <code>#36</code>
    <code>#39</code>
    <code>#43</code>
</td></tr>
<tr><td><a href="14.html">#14</a></td><td>
    <code>#39</code>
    <code>#43</code>
</td></tr>
```

### 2. Dedup by the underlying issue number (`feat: group warnings rows by source file`)

Each *mention* of an unresolvable ref warns, and the raw ref text
reaches the collector under different spellings depending on which
rewrite tier matched it (`#8`, `8.md`, `docs/issues/closed/8.md`,
`issue 8`, bare `8`). Exact-text dedup would therefore render the same
broken number several times; user clarification pinned the wanted
behavior: fold **all spellings into one row per file**, one `<code>`
per broken number.

`foldWarnings` (in `cmd/mdsick-issues/warnings.go`) therefore:

- normalizes each ref to its canonical display: the first digit run of
  the raw ref becomes `#N`; a ref with no digits (defensive; all warn
  call sites emit number-based matches) keeps its raw text;
- deduplicates on `(source, number)`;
- groups rows by source file and sorts refs within a file.

### 3. `Warning.String()` deleted

The `"%s: %s — unresolvable"` wording was only ever consumed by the old
index emission and its assertions; with the table communicating the
meaning via the heading and column headers, the method and its wording
are removed from `internal/issues/resolver.go`.

### 4. Spec and PR-doc refresh (`docs(issues): spec warnings as table`)

- `docs/issues/9.md`: the warnings bullet now describes the HTML table
  (`File | Ref`, row per file, one `<code>` per distinct broken ref,
  folded across spellings to `#N`, omitted when zero) and the
  "Unresolvable references" paragraph points at **the Warnings table**
  instead of a generic warnings section.
- `docs/plans/20261125T1623_issue-9_mdsick-issues/pr.md`: warnings
  description updated to the two-column table (row per file, folded
  refs, `<code>` cells, omitted when empty) and the smoke-run
  walkthrough annotates the warnings block as "rendered as the
  two-column warnings table".

## Deviations from the plan (see `deviations.md`)

- Task 1's plan test expected the non-natural order `9.md, 43.md,
  13.md`; fixed to ascending natural (`9.md, 13.md, 43.md`) to match the
  plan's own "sorted by File (natural)" goal.
- Task 2: `TestBuild`'s index-anchor count went 4→5 (the table adds the
  `43.html` File-cell anchor).
- Execution-time clarification beyond the plan: pages fold to **one row
  per file** with all broken refs collapsed **by underlying number**
  (spelling variants `#8`/`8.md`/`closed/8.md`/`issue 8` → a single
  `<code>#8</code>`), instead of one row per `(file, ref)` textual
  pair. `foldWarnings` gained `refNumber`/`canonicalRef` and returns
  `[]fileWarnings`; `writeWarningsTable` renders stacked `<code>`
  elements in the Ref cell; `Warning.String()` was first deleted per
  plan, then the grouping rework kept the deletion.

## Verification

- `go test ./...` — all packages pass (updated: `warnings_test.go`
  covers grouping, spelling fold, natural order; `index_test.go` asserts
  the row anchor + `<code>` cells, a stale-`<pre>` tripwire, and a
  duplicate-fold case; `run_test.go` asserts `<code>#99</code>` and the
  `#43` anchor on the index)
- `go vet ./...` — clean
- `golangci-lint run ./...` — only the accepted QF1012 in
  `internal/workspace/workspace.go`
- smoke run `go run ./cmd/mdsick-issues` — table renders exactly as the
  snippet above: `#9` and `#14` rows, no duplicate rows, no `<pre>`,
  all refs shown once each.
