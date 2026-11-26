# PR: relax issue discovery — skip non-NUM.md files (issue 18)

## Summary

`mdsick-issues` previously exited 1 whenever `docs/issues` or
`docs/issues/closed` contained a `.md` file whose base was not a bare
number (`28-original-grill-me.md`, `notes.md`, any other tooling's file
dropped into the directory). The strict `NN.md` rule in
`ListNames` made one descriptive note a hard failure of the whole tree
scan, and the failure bought nothing: files without an issue number
carry nothing for the tooling to index. This change replaces the hard
error with a skip, announced by a one-line stderr note, on both sides
of the tree.

Valid `NN.md` files — including duplicate detection, reverse natural
order, and all downstream rendering — behave exactly as before. Trees
whose every `.md` file is `NN.md` see no change at all.

## Decisions made before implementation

Settled by grilling the proposal against issue 18 before any edit:

- **Skip plus stderr note, not fully silent.** A silent skip hides the
  classic typo case (`12a.md` where `12.md` was meant): issue 12 would
  quietly vanish from the listing with no hint. One stderr line per
  skipped file keeps that surprise visible at near-zero cost, and
  execution always continues.
- **Filename gate only.** The skip happens purely on the base name at
  the `strconv.Atoi`/digits check; no content is ever read for skipped
  files, and `Head` extraction is untouched. Skipped means invisible.
- **The `NN-slug.md` alternative stays rejected.** Parsing a leading
  number out of descriptively named files would create a second way to
  claim an issue number and move "what is an issue file?" from "digits
  only" to a naming convention every slug must obey. Revisit only if
  numbered slugs routinely need indexing.

## What changed

`ListNames` in `internal/issues/list.go` no longer returns an error for
a non-numeric `.md` base. It prints
`issues: skipping "NAME" in open|closed, not NN.md` to stderr and
`continue`s. The existing skip for directories and non-`.md` entries is
unchanged; the duplicate-number hard failure in `List` and the missing
directories hard failure are unchanged.

Both sides (`open` and `closed`) share the same rule, so duplicate
detection still considers only real `NN.md` files and cannot be tripped
by descriptive names.

## Tests

The old `non numeric filename fails hard` subtest became
`non numeric filenames are skipped`, seeded with
`28-original-grill-me.md` on the open side and `notes.md` plus a
`9-original.md` (a duplicate-looking descriptive name) on the closed
side. It asserts `List` returns no error, open yields only issue 7,
closed only issue 9, and dedup is untouched by the descriptive `9-`
name. All existing `List` subtests (ordering, duplicates, missing
dirs) pass unchanged.

## Verification

- `go vet ./...` — clean
- `golangci-lint run ./...` — only the pre-existing, accepted QF1012
  finding in `internal/workspace/workspace.go:73`; zero net new
- `go test ./...` — all packages pass

## Issue

Closes #18.
