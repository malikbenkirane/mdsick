# 01 — Capture API and collector completion (issue 10)

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `Compile` exposes mmdc's diagnostics on failure while keeping its
signature, and the mermaid transformer's collector completes on
`done + errorCount + infraCount == count` so every failure path terminates the
collector — never deadlocking; infra failures (non-mmdc) print jointly and
exit nonzero after all blocks have finished.

**Architecture:** `internal/mermaid` buffers stdout always and returns an
`errors.As`-able `*Failure{Err, Output}` when mmdc fails. `Transform`'s
collector loops over `m.event` classifying events (success / diagram failure /
infra failure) and counts them against `count`; it returns when the sum
reaches `count`. After `wg.Wait()`, `Transform` hands accumulated infra
errors to an injectable `fatal` hook that prints them and calls `os.Exit(1)`.

**Tech Stack:** Go stdlib, goldmark, progressbar/v3, fake `mmdc` shell shims
on PATH for hermetic tests.

---

## Legend decisions carried by this sub-plan

- [**capture**] signature `Compile(src io.Reader, dst io.Writer) error` unchanged; stdout written to `dst` only on success; `*Failure{Err, Output}` with stderr+stdout merged (capture order) on failure; `Output` only populated when mmdc fails.
- [**count**] completion condition `done + errorCount + infraCount == count`; error events advance the bar too; final label `mermaiding done (N errors)` only when N > 0, otherwise `mermaiding done`.
- [**q1**] infra errors are counted like other failures during collection; the exit happens once, in `Transform`'s own body on the main goroutine, after `wg.Wait()` returned — not inside a spawned goroutine; a package-level `fatal` hook keeps `os.Exit` out of unit tests.

## Shared test machinery

**The shims.** A `mmdc` shell script placed in a temp dir put at the front of
PATH. It reads stdin into a scratch file; if the diagram source contains the
substring `FAIL` it prints a parse error to stderr and exits 1 — otherwise it
prints a one-line SVG. Content-based dispatch keeps each diagram's fate
deterministic inside a single call, so one shim serves the mixed
(valid + invalid) documents used everywhere below.

**The deadlock pre-condition.** The bug reproduces whenever any `Compile`
fails: the collector `continue`s on `event.err` without counting
(`internal/transformers/mermaid.go:112-115` in the pre-fix tree), so
`done == count` never fires and `<-m.event` blocks while `wg.Wait()` waits
for the collector that is blocked — `fatal error: all goroutines are asleep`,
exit 2. Every shim-based test below exercises that exact scenario, so the
Step-2 "verify it fails" runs for the mixed document manifest as a
`-timeout` kill with a goroutine dump showing the collector parked at
`mermaid.go:106` — not as an assertion failure.

**Run discipline.** All transformer tests always run with an explicit
`-timeout 60s` (the default 10m would make the pre-fix hang look like a
freeze nobody wakes from quickly).

**SVG write location.** `render()` writes `<hex>.svg` relative to cwd.
Every transformer test must chdir into `t.TempDir()` first; the repo's
existing pattern for this is `cmd/mdsick/root_test.go:68-80`.

---

## Task 1: `*Failure` capture contract in `Compile`

**Files:**

- Modify: `internal/mermaid/compile.go`
- Create: `internal/mermaid/compile_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/mermaid/compile_test.go`:

```go
package mermaid

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeShim(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := []byte(`#!/bin/sh
tmp=$(mktemp)
cat > "$tmp"
if grep -q FAIL "$tmp"; then
	printf 'mermaid: parse error near line 2\n' >&2
	printf 'partial stdout\n'
	exit 1
fi
printf '<svg xmlns="x"/>\n'
`)
	if err := os.WriteFile(filepath.Join(dir, "mmdc"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCompileSuccess(t *testing.T) {
	writeShim(t)
	var dst bytes.Buffer
	if err := Compile(strings.NewReader("graph TD; A-->B"), &dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, exp := dst.String(), "<svg xmlns=\"x\"/>\n"; got != exp {
		t.Fatalf("dst:\ngot %q\nexp %q", got, exp)
	}
}

func TestCompileFailureCarriesOutput(t *testing.T) {
	writeShim(t)
	var dst bytes.Buffer
	err := Compile(strings.NewReader("FAIL: broken diagram"), &dst)
	if err == nil {
		t.Fatal("expected error")
	}
	var fail *Failure
	if !errors.As(err, &fail) {
		t.Fatalf("expected *Failure, got %T: %v", err, err)
	}
	if !strings.Contains(string(fail.Output), "parse error") {
		t.Fatalf("Output missing stderr: %q", fail.Output)
	}
	if !strings.Contains(string(fail.Output), "partial stdout") {
		t.Fatalf("Output missing stdout: %q", fail.Output)
	}
	if dst.Len() != 0 {
		t.Fatalf("dst must stay empty on failure, got %q", dst.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/mermaid/ -v`
Expected: FAIL — `undefined: Failure` (the type does not exist yet).

- [ ] **Step 3: Write the minimal implementation**

Replace `internal/mermaid/compile.go` with the full resulting file:

```go
package mermaid

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	_ "embed"
)

//go:embed mmdc-config.json
var config []byte

type Failure struct {
	Err    error
	Output []byte
}

func (f *Failure) Error() string { return f.Err.Error() }

func (f *Failure) Unwrap() error { return f.Err }

func tempConfig() (path string, err error) {

	f, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		return "", fmt.Errorf("os: create temp: %w", err)
	}

	defer func() {
		err = errors.Join(err, f.Close())
	}()

	if _, err = f.Write(config); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}

	return f.Name(), nil

}

func Compile(src io.Reader, dst io.Writer) (err error) {

	config, err := tempConfig()
	if err != nil {
		return fmt.Errorf("prepare config file: %w", err)
	}

	defer func() {
		err = errors.Join(err, os.Remove(config))
	}()

	args := []string{
		"-c", config,
		"-b", "#ffffff",
	}

	var out bytes.Buffer
	cmd := exec.Command("mmdc", append(args, "-i", "-", "-o", "-")...)
	cmd.Stdin = src
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return &Failure{
			Err:    err,
			Output: out.Bytes(),
		}
	}

	_, err = dst.Write(out.Bytes())
	return err

}
```

Points that must not drift: `Stdout` and `Stderr` both feed `out` (merged);
`dst` receives bytes only after a zero exit; the signature is unchanged.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/mermaid/ -v && go vet ./...`
Expected: PASS; vet clean.

- [ ] **Step 5: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence from
`core-commands` (mktemp → Read → Write → `jj describe --stdin < <path> && rm
<path> && jj new`).

Subject for this task: `fix(mermaid): capture mmdc output on failure`

---

## Task 2: Collector completes on mixed outcomes

**Files:**

- Modify: `internal/transformers/mermaid.go`
- Create: `internal/transformers/mermaid_test.go`

**Scope guard.** In this sub-plan the failing fence simply survives unstyled
(no artifact yet — that arrives in sub-plan 02), and the collector's only
behavioral change is counting. Task 3 of the prior behavior — joint infra
exit after `wg.Wait()` — is part of the same code shape below; its assertion
lives in this task's tests (classification) and in sub-plan 03 (process
level).

- [ ] **Step 1: Write the failing test**

Create `internal/transformers/mermaid_test.go`:

```go
package transformers

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/malikbenkirane/mdsick/internal/mermaid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
)

func chdirTemp(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	})
}

func writeShim(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := []byte(`#!/bin/sh
tmp=$(mktemp)
cat > "$tmp"
if grep -q FAIL "$tmp"; then
	printf 'mermaid: parse error near line 2\n' >&2
	exit 1
fi
printf '<svg xmlns="x"/>\n'
`)
	if err := os.WriteFile(dir+"/mmdc", script, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func convert(t *testing.T, src string) string {
	t.Helper()
	writeShim(t)
	chdirTemp(t)

	md := goldmark.New(goldmark.WithParserOptions(
		parser.WithASTTransformers(util.PrioritizedValue{
			Value: &Mermaid{},
		}),
	))
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestMixedDiagamsRender(t *testing.T) {
	html := convert(t, "before\n\n```mermaid\nFAIL: bad\n```\n\n```mermaid\ngraph TD; A-->B\n```\n\nafter\n")
	if !strings.Contains(html, "FAIL: bad") {
		t.Fatalf("failed fence dropped:\n%s", html)
	}
	if !strings.Contains(html, ".svg") {
		t.Fatalf("valid diagram missing SVG reference:\n%s", html)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/transformers/ -run TestMixedDiagamsRender -v -timeout 60s`
Expected: FAIL — `test timed out after 1m0s`; the goroutine dump shows the
collector parked at the `<-m.event` receive in the pre-fix loop
(`mermaid.go:106`).

- [ ] **Step 3: Write the minimal implementation**

Replace the whole `Transform` function in `internal/transformers/mermaid.go`
with the version below (the `render` method head stays as-is in this
sub-plan — it still emits bare `event{err}` on any path, which the collector
now classifies):

```go
func (m *Mermaid) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {

	var wg sync.WaitGroup

	var changes []*change
	var mu sync.Mutex

	m.event = make(chan event, 1)
	m.ready = make(chan struct{})
	defer close(m.event)
	defer close(m.ready)

	var count, done, errCount, infraCount int
	var infraErrs []error

	wg.Go(func() {
		_, ok := <-m.ready
		if !ok {
			return
		}
		for {
			event, ok := <-m.event
			if !ok {
				return
			}

			if event.change != nil {
				mu.Lock()
				changes = append(changes, event.change)
				mu.Unlock()
			}

			_ = m.bar.Add(1)

			if event.err != nil {
				var fail *mermaid.Failure
				if errors.As(event.err, &fail) {
					errCount++
				} else {
					mu.Lock()
					infraErrs = append(infraErrs, event.err)
					mu.Unlock()
					infraCount++
				}
			} else {
				done++
			}

			if done+errCount+infraCount == count {
				if errCount+infraCount > 0 {
					m.bar.Describe(fmt.Sprintf("mermaiding done (%d errors)", errCount+infraCount))
				} else {
					m.bar.Describe("mermaiding done")
				}
				return
			}
		}
	})

	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.FencedCodeBlock:
			if string(n.Language(reader.Source())) != "mermaid" {
				return ast.WalkContinue, nil
			}
			wg.Go(func() {
				m.render(n.Lines().Value(reader.Source()),
					change{
						parent:   n.Parent(),
						oldChild: n,
					})
			})
			count++
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	if count > 0 {
		m.bar = progressbar.New(count)
		m.bar.Describe("mermaiding...")
		m.ready <- struct{}{}
		wg.Wait()
	}

	for _, change := range changes {
		change.parent.ReplaceChild(change.parent, change.oldChild, change.newChild)
	}

	if len(infraErrs) > 0 {
		fatal(infraErrs)
	}

}
```

And at file scope (the Legend [**q1**] hook):

```go
var fatal = func(errs []error) {
	for _, err := range errs {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(1)
}
```

Depends on the `*mermaid.Failure` type from Task 1, so `errors` joins the
import block. Note: `bar.Add` and the `changes` append happen for **every**
completed event; the counting-arc before the completion check is
classification only. And the known dead `svg64` block
(`mermaid.go:70-78`) stays untouched — issue 11's cleanup, not ours.

**Subtlety to get right** (from the pre-fix code): the old code incremented
`done` under the same `mu` that guarded `changes`; the new code splits the
two because counters stay private to the collector goroutine and only
`changes`/`infraErrs` are shared — this is why each append has its own small
`mu.Lock()/Unlock()` pair.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/transformers/ -v -timeout 60s && go build ./cmd/mdsick && go test ./... -timeout 60s`
Expected: PASS across all packages.

- [ ] **Step 5: Commit with caveman-commit**

Temp-file pattern again (mktemp → Read → Write → `jj describe --stdin < <path> && rm <path> && jj new`).

Subject for this change: `fix(mermaid): let collector finish on failures`

---

## Task 3: Keep the `fatal` hook test-replaceable

**Files:**

- Test: `internal/transformers/mermaid_test.go`

`os.Exit` cannot run in a unit test — it would kill the test process. So the
`fatal` hook's contract is asserted by swapping it out; this task pins the
hook's shape so no later edit breaks the injection seam that the joint-exit
behavior rides on.

- [ ] **Step 1: Write the test**

Add to `internal/transformers/mermaid_test.go`:

```go
func TestFatalHookInjectable(t *testing.T) {
	var got []error
	saved := fatal
	fatal = func(errs []error) {
		got = append(got, errs...)
	}
	defer func() { fatal = saved }()

	in := []error{errors.New("rand: read"), errors.New("os: create")}
	fatal(in)

	if len(got) != len(in) {
		t.Fatalf("hook lost errors:\ngot %d\nexp %d", len(got), len(in))
	}
}
```

- [ ] **Step 2: Run the whole tree one last time before the sub-plan closes**

Run: `go test ./... -timeout 60s && go vet ./... && golangci-lint run ./...`
Expected: everything PASS / clean except the one long-accepted warning
(`QF1012` at `cmd/mdsick/root.go:171`).

- [ ] **Step 3: No commit — Task 2's single change already contains the hook plus this test**

If your review shows the test landed after the Task 2 change, fold it in:
describe the same change again via the temp-file pattern with the same
subject, and keep one logical change (user confirm before describing an
already-described change).

---

## Handoff

When all tasks are done and committed, tick the 01 line in `00-index.md` in a
separate change (message:
`docs(plans): mark 01-capture-and-collector done in issue-10`), via the
temp-file pattern. Then proceed to `02-failure-artifact.md` — or start
`03-gate-and-regression.md` in a concurrent session, per the index's batch
note (`[02, 03]`).
