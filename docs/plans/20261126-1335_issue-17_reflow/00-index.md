# mdsick-reflow — Index

> **For agentic workers:** Each sub-plan uses the bite-sized TDD task format with
> checkbox tracking and caveman-commit via the `core-commands` temp-file pattern.
> Execute sub-plans in order.

Sub-plans (execution order):

- [ ] 01-core-engine — `internal/reflow` skeleton: reader/error contract, splice assembler, paragraph soft seams, top-level blank-run normalization, POSIX trailing newline
- [ ] 02-containers-and-immunity — prefix-run scanner, prose detection (`Paragraph`/`TextBlock`, Table exclusion), blockquote markers incl. lazy and hard breaks, list/ordered byte-exact frames, immune pass-through
- [ ] 03-cli-and-docs — `cmd/mdsick-reflow` (TTY refusal, stderr routing), cmd test, `docs/issues/17.md` rewrite, README line

Concurrent-eligible batches: none — each sub-plan extends code written by the previous one.

Why this order and split: the seam/prefix helpers in 01 are the substrate; 02
bolts block-structure rules onto them; 03 depends on the finished package. Same-file
edits (`internal/reflow/*.go`) stay within each sub-plan's own batch.

Facts locked by the committed AST-shape tests (`internal/reflow/ast/`): soft/hard
breaks are flags on `Text`; tight list item text is a `TextBlock`, blank lines flip
it to `Paragraph`; a lighter quote marker line merges into the inner paragraph;
tables decompose row-per-line; setext underlines swallow preceding lines. Track
decisions against these facts, never against issue 17's original wording.

Complexity note: the gate fired (single-file estimate > 600 lines); the user
approved this three-way split.
