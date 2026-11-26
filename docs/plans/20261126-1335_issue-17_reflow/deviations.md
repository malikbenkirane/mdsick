# Deviations from the plan (`00-index.md` + sub-plans)

Facts found while executing that the plan assumed differently, and the
adjustments made. All locked-in choices are covered by tests in
`internal/reflow/reflow_test.go`.

- **goldmark trims leading/trailing whitespace off prose line segments**
  (dump-verified). The plan assumed "trailing spaces on the last line
  belong to the block" via `blockRange`'s segments. Consequences:
  - the first line's leading indent sits before `seg.Start`; the refill
    edit start consumes it backward, but only when the whitespace run
    backs onto a line start (never across a `>` or list marker);
  - the last line's trailing whitespace lies after the block's range;
    the tail blank-run edit preserves the whitespace before its first
    newline instead of collapsing it (keeps `a b c  \n` byte-exact).
- **fence closers, setext underlines and thematic dashes are parser
  residue outside `Lines()`** (dump-verified). The plan's blanket
  blank-run collapse deleted closing fences (`"```go\nx\n```\n\n`
  lost the ` ``` ```) and setext underlines. Blank-run normalization
  now applies **only** to regions whose every line is pure whitespace;
  contaminated regions are left byte-exact. The collapse also preserves
  the last region piece (the next block's leading indent) so indented
  code survives a preceding blank run.
- **refillBlock is a seam loop with different arithmetic than the plan's
  draft**: raw (backslash) seams win over hard-break detection (a literal
  backslash inside a two-space run stays raw — the plan's
  `pw < len(keep)` ordering misclassified it as a hard break and
  doubled the backslash); hard seams collapse the trailing run to
  `" \"` (one space kept, per the pinned tests, not the plan's bare
  `"\n"`) and resume copying at `prev.Stop` so the continuation line's
  prefix (`> ` or item indent) is emitted byte-exactly — the plan's
  `pos = next.Start` dropped container prefixes and failed its own
  quote/list hard-break tests.
- **inline nodes panic on `Lines()` in goldmark 1.8.2** (BaseInline
  panic). `blockRange` and `collectProse` skip inline nodes;
  `ast.KindTable` lives in `goldmark/extension/ast`, not `goldmark/ast`.
- **`blankCount` semantics** redefined: blank lines counted strictly
  between the blocks (excluding the previous block's trailing-whitespace
  piece and the next block's leading-indent piece), so a trailing two
  spaces before a single blank line stay byte-exact.
- **normalizeTop no longer refills paragraphs in its own loop**: prose
  collection happens once in `collectProse` (a duplicate top-level
  Paragraph edit would make `apply` panic on overlap).
- Plan files listed `internal/reflow/reflow.go` for the driver
  (`reflow()`, `normalizeTop` etc.); they live in `normalize.go` for
  file focus. No behavior difference.
