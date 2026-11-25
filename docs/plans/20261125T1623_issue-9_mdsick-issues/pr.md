# PR: specialized mdsick-issues command (issue 9)

## Summary

Adds `cmd/mdsick-issues`, a second mdsick binary that renders a
`docs/issues` + `docs/issues/closed` tree into one GitHub-styled landing
page with flat per-issue pages. Today "glance at all issues" means two
shell commands in two directories and two browser windows of flat
directory renders; cross-references between issues (`#21`,
`[8](closed/8.md)`, `` `43.md` ``) are dead text. After this PR one command
renders every issue page, rewrites every reference tier so links survive
issues moving between open and closed, surfaces unresolvable references as
a warnings table, and opens the landing page in the browser.

The first three sub-plans are pure refactors of the existing `cmd/mdsick`
into `internal/*` packages; the binary's behavior is unchanged (existing
tests pass untouched as far as they test moved code, and they moved with
it: `TestNaturalCompare` became `internal/natural`'s table, `TestDocTitle`
became `internal/markdown`'s).

## Commits (execution order)

| Commit | What |
|---|---|
| `refactor: extract workspace plumbing` | `internal/workspace` (`TempWorkspace`, `CopyStyle`, `RenderFile`) |
| `refactor: extract natural comparison` | `internal/natural` (`Compare` digit-run natural sort) |
| `refactor: extract markdown builder and render loop` | `internal/markdown` (`New` goldmark engine); `cmd/mdsick` imports it all |
| `feat: issue repo discovery` | `issues.FindRepo`: walk-up to `.jj`/`.git`, `--vcs` semantics without the CLI |
| `feat: list and validate issue files` | `issues.List`/`Row`: strict `NN.md` filenames, reverse natural order, hard fail on duplicates or missing dirs |
| `feat: issue number index` | `issues.Index`: number → `NN.html` resolution, `Warning` type |
| `feat: issue reference rewriting` | `issues.Transformer`: all four rewrite tiers, deferred AST mutation after the walk |
| `refactor: move doc title extraction` | `docsTitle` → exported `markdown.DocTitle` |
| `feat: issues index writer` | `writeIssuesIndex`: two fixed sections, disclaimers, warnings table |
| `feat: mdsick-issues binary` | cobra binary, `--vcs` flag, args-joined title, e2e tests |
| `docs(issues): mark issue 9 implemented` | checklist in `docs/issues/9.md` ticked |

## What changed

### 1. Shared plumbing reused, not duplicated
`cmd/mdsick` and `cmd/mdsick-issues` both funnel through
`workspace.TempWorkspace` (fresh temp dir printed to stderr, embedded
stylesheet copied, chdir — the ordering invariant that lets mermaid SVG
sidecars land in the workspace is preserved) and `workspace.RenderFile`
(the GitHub-styled HTML shell, the known double-`</body>` quirk kept).
The goldmark engine is built by `markdown.New`; the issues engine adds
`markdown.NewIssues ix, source, warns` with the same options plus the
`issues.Transformer` running *after* the mermaid transformer.

### 2. Repo discovery and preflight (`internal/issues`)
`FindRepo(vcs)` walks up from cwd, `.jj` preferred over `.git` per level,
`--vcs jj`/`git` restricting detection, explicit error when no marker is
found. `List(root)` requires *both* `docs/issues` and
`docs/issues/closed` to exist, requires every `.md` to be all-digits
`NN.md` (anything else fails hard, naming the file), and fails hard
(`duplicate issue number N`) when the same `NN` exists on both sides —
before one page is rendered, because the flat `NN.html` must be
unambiguous.

### 3. Reference rewriting in temp copies, numbers alone resolve
One walk, deferred mutation (the same post-walk-apply pattern
`internal/transformers/mermaid.go` uses, because mutating mid-walk is
unsafe). A directory part in any reference is ignored — `21.md`,
`docs/issues/21.md`, `closed/21.md`, `[8](closed/8.md)`, `[epic](21.md)`
all resolve to `8.html`/`21.html` respectively by number alone:

| Tier | Example | Behavior |
|---|---|---|
| Hash ref | `fixes #21` | `#21` becomes a link; word-boundary regex |
| Link destination | `[8](closed/8.md)` | dest rewritten to `8.html`, label untouched |
| Prose phrase | `issue 8` / `issue closed 8` | the phrase becomes a link |
| Path in prose or code span | `` `43.md` ``, `docs/issues/closed/21.md` | the path becomes a link, label keeps original text |

Fenced code blocks are skipped entirely. Unresolvable references are *not*
dropped: they append `issues.Warning{Source, Ref}` — the index's
`<h2>Warnings</h2>` block is a two-column table (File | Ref), one row
per source file with broken references, the Ref cell holding one
`<code>` element per distinct broken reference (folded across mentions
and spellings, numeric refs displayed canonically as `#N`):
the File cell links the flat page (`<a href="9.html">#9</a>`) and the Ref
cell is `<code>`-wrapped text (e.g. `<code>#44</code>`) so broken refs
surface to humans instead of stderr.

### 4. The landing page (`writeIssuesIndex`)
Fixed `<h2>Open Issues</h2>` / `<h2>Closed Issues</h2>` sections,
reverse-natural-ordered entries (issue 14 before issue 6), each entry
linking `NN.html` and showing the issue's `# Heading 1` text from
`markdown.DocTitle`. Empty sections render `No open issues.` /
`No closed issues.`, never an error. Title: positional args joined, else
`to/repo Issues` from the repo root's last two components.

### 5. End-to-end coverage
`go test ./cmd/mdsick-issues` renders a fixture repo through the real
`build(root, args, show)` (the browser-open step is injectable) and:
asserts the index's anchor set with `golang.org/x/net/html`'s tokenizer
(no golden snapshots), asserts `#43`→`href="43.html"` and
`closed/10.md`→`href="10.html"` on issue 21's page, asserts back-references
on the closed page, and asserts warnings plus the empty-section disclaimer
surface on the index. Unit tables cover `findRepoFrom`,
duplicate/nonnumeric/missing-dir hard failures, resolution order, and each
rewrite tier.

## Behavior summary (demoed against this repo)

`go run ./cmd/mdsick-issues` from the mdsick repo itself produced (temp dir
printed to stderr, browser opened):

- `<title>` and `<h1>` `sensible/mdsick Issues`
- Open Issues: 14, 13, 11, 9, 7, 6 (reverse numeric order) with titles
- Closed Issues: 10, 5, 3, 1 (reverse numeric order) with titles
- a Warnings block flagging unresolvable `#NN`/path references in
  `9.md`, `14.md` (rendered as the two-column warnings table)

## Verification

- `go test ./...` — all ten packages pass
- `go vet ./...` — clean
- `golangci-lint run ./...` — one accepted finding (QF1012, migrated with
  the shell template into `internal/workspace/workspace.go`); zero net new
- smoke run against the real repo as above

## Deliberate deviations from the plan

Documented in [deviations.md](./deviations.md): macOS symlink-aware temp
test, exported shell constants, import-cycle-driven external test package
plus `NewTransformer`, the capturing `hashRe` boundary group, boundary-free
link labels, whole-node code-span rewriting (a Link inside a `CodeSpan`
crashes the renderer), the fixture-consistent anchor count of 4, and the
nested-fixture-dir helper fix.

## Scope guard

`cmd/mdsick-issues` hardcodes the `docs/issues` +
`docs/issues/closed` layout and cobra is used only by it; `cmd/mdsick`
keeps its single-function stdin/batch behavior intact (its source shrank
because plumbing moved, not because behavior changed).

## Issue

Closes #9.
