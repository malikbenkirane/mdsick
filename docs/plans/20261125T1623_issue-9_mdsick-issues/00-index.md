# mdsick-issues — Index

**Spec:** docs/issues/9.md (grilled and updated: cobra in `mdsick-issues` only, `--vcs` selector, plain `## Warnings` block, empty-section disclaimers, reverse natural sort by number, browser-back navigation, html-stdlib invariant tests).

Sub-plans (execution order):

- [ ] 01-shared-plumbing — extract `internal/workspace` (TempWorkspace, CopyStyle, RenderFile), `internal/natural` (Compare), `internal/markdown` (New); `cmd/mdsick` re-imports, behavior unchanged
- [ ] 02-discovery-and-preflight — extract `internal/issues`: FindRepo (walk-up, `--vcs` precedence semantics without the CLI), List (open/closed globs, numeric filenames, duplicates fail hard), reverse-sorted rows
- [ ] 03-resolver-and-rewriting — `internal/issues` Index + AST transformer: `#NN` hash refs, `NN.md` link destinations (directory part ignored), `issue NN` prose, path/code-span linkification; fenced code skipped; warnings collected
- [ ] 04-index-and-binary — `cmd/mdsick-issues` cobra binary: `--vcs` flag, args-joined title (default: last two root components + ` Issues`), writeIssuesIndex (two sections, disclaimers, plain warnings block), e2e invariant tests via `html` stdlib

Concurrent-eligible batches: `[01-shared-plumbing, 03-resolver-and-rewriting]`, then `[02-discovery-and-preflight]`, then `[04-index-and-binary]`.

Why this order and split: 01 is a pure mechanical refactor reviewable in isolation; 03 only adds new files plus one extension point in `internal/markdown`, so it is independent of 01; 02 is a leaf package; 04 assembles all three. Each batch is the safe unit for one concurrent session; same-file edits stay within a batch.

Every plan task commits with the `core-commands` sequence (mktemp → Read → Write → `jj describe --stdin` < tmpfile) using a `caveman-commit` message.
