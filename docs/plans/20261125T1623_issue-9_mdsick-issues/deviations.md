# Deviations from the plan: mdsick-issues (issue 9)

You can roughly think "bug in the plan text" vs "command decision during
implementation". Every deviation listed here keeps the plan's intent; each
one was driven by either a failing test or macOS/build reality, not taste.

## Sub-plan 01 — shared plumbing

### 1. `TempWorkspace` test must resolve the macOS temp-dir symlink
The plan asserts `Getwd == Clean(tmp)`. On macOS `$TMPDIR` sits under
`/var/folders/...` (a symlink to `/private/var/folders/...`), so the
process reports the `/private/...` form and `filepath.Clean` never
equalizes them. The test now `EvalSymlinks` both sides before comparing.

### 2. `RenderFile` tests: title is not escaped, and the tripwire is explicit
The plan's test expected `<title>Out &amp; Title</title>`, but the existing
`render()` (moved verbatim into `RenderFile`) interpolates the title raw —
"byte-identical behavior" outranks the plan's escaping assumption, so the
test asserts the raw form. The plan also suggested its bogus
`out.html-body-anchor` assertion be replaced; we took that suggestion and
assert `strings.Count(s, "</body>") == 2` (the known double-close quirk).

### 3. Shell constants are exported from `internal/workspace`
The plan moves `styleRef`, `paddingHorizontal`, `paddingVertical` into the
workspace package but keeps `writeIndex` (which uses all three) in
`cmd/mdsick`. Unexported constants would make `writeIndex` uncompilable;
duplicating them in two packages invites drift. They are now
`workspace.StyleRef` / `Workspace.PaddingHorizontal` / `PaddingVertical`,
used by `RenderFile`, `cmd/mdsick`'s `writeIndex`, and the new
`cmd/mdsick-issues` index writer — one source of truth.

## Sub-plan 02 — discovery and preflight

### 4. Nested fixture dirs must be passed as one slash-containing arg
The plan calls `makeTree(t, ".jj", "docs", "issues")`, expecting
`docs/issues` nested — but the helper `MkdirAll`s each argument standalone,
producing sibling dirs, and the walking-up tests fail with `chdir: no such
file or directory`. Tests (and the helper) now use `"docs/issues"` /
`"repo/.git"` style slash-joined args; the implementation is exactly the
plan's.

## Sub-plan 03 — resolver and rewriting

### 5. External test package + `NewTransformer` constructor (import cycle)
`internal/markdown` imports `internal/issues` (it registers
`issues.Transformer`), and `refs_test.go` needs `markdown.NewIssues`. An
in-package test would make the cycle `issues(test) → markdown → issues`,
which Go rejects. `refs_test.go` therefore lives in `package issues_test`,
and `Transformer` fields stay unexported behind an
`issues.NewTransformer(index, source, warnings)` constructor that
`markdown.NewIssues` calls.

### 6. `hashRe` boundary group must be capturing
The plan's regex `(?:^|[^\w])#(\d+)\b` makes the digits group 1, but the
plan (and its call site) index group 2 — that panics with
`index out of range [4] with length 4`. The group is now capturing
(`(^|[^\w])#(\d+)\b`), preserving the plan's intended group numbering.

### 7. The link label excludes the boundary character
Tests demand `<a href="21.html">#21</a>`, but a literal
"full match is the label" produced ` #21`. `splitAndLink` takes a `hash`
flag: for the `#NN` tier the label starts one byte before the digits (at
the `#`), leaving any preceding non-word character in the leading
`ast.String` fragment. Unresolvable `#NN` warnings likewise carry the
leading `#` (`Ref: "#44"`).

### 8. Code spans rewrite as whole nodes
The plan claims "CodeSpan children are `ast.Text` nodes, so the `*ast.Text`
case covers code spans automatically" and the test asserts code-span
linkification. Replacing a `Text` *inside* a `CodeSpan` by a `Link`/`String`
crashes the renderer with `interface conversion: ast.Node is *ast.Link, not
*ast.Text` — the CodeSpan renderer expects Text children. Instead
`Transform` now has a `*ast.CodeSpan` case: it concatenates the span's
children, runs the `NN.md` tier over the combined value, `WalkSkipChildren`s
the span, and replaces the whole `CodeSpan` node with the fragment sequence
(the backticks are lost, the link survives).
Also: the folded duplicate call site (the plan contained
`splitAndLink`'s body twice, embedded) was de-duplicated into one function.

## Sub-plan 04 — index and binary

### 9. Test-asserted strings conformed to reality (per the plan's own notes)
The plan's engineer notes already flag these; applied as stated:
- `No closed issues` assertion kept (writer emits `No closed issues.`);
- `"## Warnings"` relaxed to `<h2>Warnings</h2>`.
The e2e anchor count is **4, not 3**: the plan's own fixture defines
`43.md` as an *open* issue, so the index legitimately lists
`<a href="43.html">` for it — the plan's expectation contradicted its
fixture.

### 10. Constants fetched from `internal/workspace` instead of duplicated
`writeIssuesIndex` uses `workspace.StyleRef` / `PaddingHorizontal` /
`PaddingVertical` rather than re-declaring the trio (see deviation 3).

### 11. Severity boundary: the plan's "title default" uses the args fallback
Not a deviation, just recorded: `build(root, nil, show)` yields the
`lastTwoComponents(root) + " Issues"` default title exactly as planned; the
"Args: cobra.ArbitraryArgs" surface was kept, and the redundant
`os.Stat(docs/issues)` check was kept per the plan's note 3.

## Cross-cutting

### 12. Commit granularity followed the plan's per-task list
One `jj describe` per task with caveman-commit subjects
(`refactor: extract workspace plumbing`, `refactor: extract natural
comparison`, `refactor: extract markdown builder and render loop`,
`feat: issue repo discovery`, `feat: list and validate issue files`,
`feat: issue number index`, `feat: issue reference rewriting`,
`refactor: move doc title extraction`, `feat: issues index writer`,
`feat: mdsick-issues binary`, `docs(issues): mark issue 9 implemented`).
`mktemp` → Read → Write → `jj describe --stdin` was used for every message;
no heredocs, no `-m`.

### 13. Lint baseline unchanged
`golangci-lint run ./...` reports exactly one issue: the accepted QF1012
(staticcheck) on the embedded shell-template `WriteString(fmt.Sprintf(..))`
in `internal/workspace/workspace.go` — the same warning that used to live
at `cmd/mdsick/root.go`. Zero net new findings.
