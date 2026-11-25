# PR: fix mermaid failure handling (issue 10)

## Summary

This PR fixes the known mermaid render deadlock and turns failed diagrams
into useful output instead of a crashed process. A run with an invalid
mermaid block used to hit `fatal error: all goroutines are asleep` (exit 2)
because the render collector skipped error events without counting them —
in batch mode a single bad diagram in one file killed the whole directory
run.

## What changed

### 1. mmdc diagnostics are captured, not lost
`internal/mermaid/compile.go` buffers mmdc's merged stdout+stderr and, on
non-zero exit, returns an `errors.As`-able `*Failure{Err, Output}`. The
public signature `Compile(src, dst)` is unchanged and `dst` still receives
bytes only on success. Covered by `internal/mermaid/compile_test.go` with a
content-dispatching fake `mmdc` shell shim.

### 2. The collector terminates on every outcome
`internal/transformers/mermaid.go` classifies each completion (success /
diagram failure / infra failure) and completes on
`done + errorCount + infraCount == count`, so no failure path can deadlock.
Diagram failures render silently at the failure site (below); infra errors
(no/lost tooling) are printed jointly after the batch finishes and exit
nonzero via an injectable `fatal` hook, keeping `os.Exit` out of unit tests.

### 3. Failed blocks become failure artifacts
Instead of dropping or crashing, a failed block keeps its fence with every
line numbered (`1| FAIL: bad`, plain `mermaid` header) and is immediately
followed by a fenced `text` block holding mmdc's full diagnostic output —
no truncation, no stderr noise. This is carried by a new `CodeLiteral` node
with a custom HTML renderer byte-identical to the default fenced-code
output (goldmark's AST is source-offset based, so synthesized lines cannot
ride native code blocks). Both artifact changes ride one event per block,
which provably terminates the collector on any document shape.

### 4. Missing-tool gate, both modes
A pre-flight `exec.LookPath("mmdc")` check runs once per invocation:
- stdin mode: only when the source plausibly contains ` ```mermaid `
- batch mode: whenever `*.md` files exist

On miss: one explicit stderr message and a nonzero exit, nothing rendered,
browser never opened.

### 5. Subprocess-level regression test
`cmd/mdsick/mermaidfail_test.go` builds the binary and runs it as a child
process with a fake `mmdc` on a hermetic `PATH`, feeding a mixed document
(one valid + one invalid diagram). It asserts the child finishes (a 30s
deadline turns any reintroduced collector hang into a loud failure), reads
the workspace path the child prints, and asserts `rendered.html` contains
the numbered surviving fence, the `text` diagnostics fence, and the healthy
diagram's `.svg` reference.

## Behavior summary

| Scenario | Before | After |
|---|---|---|
| One invalid diagram, `cat doc.md \| mdsick` | whole process deadlocks, exit 2 | browser opens; document shows numbered fence + mmdc diagnostics; valid diagrams still render |
| Same in batch mode (one bad file) | entire run crashes | that file shows its failure artifacts; other files fine |
| `mmdc` missing from PATH | deadlock again, exit 2 | single explicit message, nonzero exit, nothing opened |

## Verification

- `go test ./...` — all packages pass (transformer tests run with an
  explicit `-timeout 60s`)
- `go vet ./...` — clean
- `golangci-lint run ./...` — only the long-accepted `QF1012` warning at
  `cmd/mdsick/root.go` remains
- Currently, the mixed-document end-to-end test passes as a subprocess

## Deliberate deviations from the plan

Documented in
[deviations.md](./deviations.md). The plan's two-events-per-failure shape
had a window where the second artifact change was dropped whenever a
failure resolved the last counting slot; all changes are consolidated into
a single event per block to preserve the plan semantics and terminate
correctly. Two test hermeticity adjustments (PATH pinning, renderer
registration in the `convert` helper) are also described there.

## Issue

Closes #10.
