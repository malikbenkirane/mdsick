# Deviations

## Initial plan execution

- Task 1, Step 1: the plan's test expected warnings ordered `9.md`,
  `43.md`, `13.md`, which contradicts both the plan's own "ascending
  natural order" note and the goal's "sort by File (natural)". Fixed the
  expectation to `9.md`, `13.md`, `43.md` (ascending natural); the
  implementation keeps `natural.Compare` ascending for both columns.
- Task 2, Step 1 (beyond plan): `TestBuild` in `run_test.go` counted the
  index anchors and now sees a fifth one — the warnings table's
  `<a href="43.html">#43</a>` row. Raised the expected count from 4 to 5
  and updated the message; otherwise the plan's assertions were applied
  as written.

## Folding clarifications (post-plan commit `feat: group warnings rows by source file`)

- User clarification 1: the same broken number under different spellings
  (`#8`, `8.md`, `docs/issues/closed/8.md`, `issue 8`, bare `8`) must
  not produce one row per spelling. The plan's fold was exact on
  `(source, ref)` text; it now normalizes each ref to its underlying
  issue number (first digit run → canonical `#N`) and deduplicates on
  `(source, number)`. Numberless refs (defensive path only) keep their
  raw text and do not fold.
- User clarification 2: rows should group **per source file**, not per
  broken reference — one `File | Ref` row per file, the Ref cell holding
  one `<code>` element per distinct broken number, one per line.
  `foldWarnings` therefore returns `[]fileWarnings{Source, Refs}` and
  `writeWarningsTable` renders the stacked-`<code>` Ref cell.
- Spec text (`docs/issues/9.md` bullet) and PR doc
  (`20261125T1623.../pr.md`) were updated again to the row-per-file
  shape after these clarifications; the earlier one-row-per-pair wording
  written by the initial execution is superseded there.
