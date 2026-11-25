# Deviations from the plan

The plan was executed as written, except for the deliberate changes below.
None change observable behavior; all preserve the plan's semantics.

## 1. One event per block, carrying both artifact changes (02 Task 2)

The plan sends **two events per failed block** (`insertAfter` first, then
`setNewChild`) and counts the failure slot on the insert event only.
That design has a window: when a failure resolves the **last** slot
(`done + errCount + infraCount == count` fires after the insert event), the
collector returns immediately and the partner `setNewChild` event is never
collected — the numbered surviving fence is silently dropped. The plan's own
`TestFailureArtifactShape` (a single failing block, `count == 1`) exposes
exactly this; only the mixed two-block test survives.

Fix: `event` now carries `changes []*change` and `render` emits **one event
per block** containing its change(s). The collector appends all changes and
counts the block once (failure if any `insertAfter` is present, success on a
non-literal `setNewChild`). This keeps every plan guarantee — one slot per
block, insert-then-replace ordering inside each event (the apply loop
processes a single event's changes in order), per-event bar `Add(1)` — and
terminates on every document shape.

## 2. Test `PATH` pinned hermetically (03 Task 1)

`TestGateBehaviors` prepends the shim dir to the ambient `PATH`. This is
not enough on machines where a real `mmdc` is installed: the gate's
`exec.LookPath("mmdc")` finds the real binary and the "missing mmdc errors"
subtest fails. The subtests now pin `PATH` to *only* the shim directory (`t.
Setenv("PATH", dir)`), making the gate behavior independent of the host
environment. The shim-present subtest is unaffected (same result either way).

## 3. Lint-driven touch-ups (02)

`golangci-lint` flagged three things the plan's snippets ignored:

- `errcheck` on `w.WriteString` / `w.Write` in `renderCodeLiteral` — the
  renderer propagates write errors (presentation only; assertions unchanged).
- `S1011` — the `text` fence's lines are appended via a variadic `append`,
  no loop.
- `errcheck` on `defer f.Close()` in `mermaidfail_test.go:40` — the command
  body uses the file for stdin only; errors on close are intentionally
  discarded.

## 4. Test helper registered the `CodeLiteralRenderer` (02 Task 2)

The plan's `convert` helper (carried over from 01) registers only the
AST transformer. Any document containing a `codeLiteral` crashes the goldmark
renderer with `index out of range` — the `nodeRendererFuncs` slice is sized
from the *registered* renderers, and rendering an unknown kind indexes past
its end. `convert` now registers `CodeLiteralRenderer` the same way
`newMarkdown()` does.

## 5. Cosmetic

- The `mermaid` import appears in the 01 test file from the start per the
  plan (it is then used by later tests); during execution it was removed
  until the import that needed it actually existed, and re-added by the
  eventual consumer tests.
- `event.change` (singular) was renamed to `event.changes` (slice) per
  deviation #1; the `style` attribute on the success-path image is set
  exactly as before, byte-for-byte.
