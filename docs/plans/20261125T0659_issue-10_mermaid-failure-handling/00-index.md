# Mermaid failure handling — Index

Implements [docs/issues/10.md](../../../docs/issues/10.md): fix the mermaid
deadlock on failed render, add the failure artifact, add the environment gate,
and land a subprocess-level regression test. Scope is strictly the four
deliverables from the issue; the adjacent warts (`svg64` dead code, `randHex`
SVG naming, `render()`'s double `</body></html>`) belong to issue 11 and are
untouched.

Sub-plans (execution order):

- [x] 01-capture-and-collector — `*Failure` capture API in `Compile`, collector completes on `done + errorCount == count`, bar wording, joint infra-error exit
- [ ] 02-failure-artifact — surviving line-numbered fence, `text` fence with full mmdc output, `insertAfter` change kind
- [ ] 03-gate-and-regression — pre-flight `mmdc` PATH gate in both modes, fake-`mmdc` shim subprocess regression test

Concurrent-eligible batches: `[01], [02, 03]`

Why this order and split: 01 lays the capture/counting foundation that 02's
artifact and the regression test's assertions depend on; 02 and 03 touch
disjoint files (`internal/transformers/mermaid.go` vs `cmd/mdsick/`) so each
is the safe unit for one concurrent session, and all same-file edits stay
inside a batch.
