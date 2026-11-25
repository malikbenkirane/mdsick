# 03 — Environment gate and subprocess regression (issue 10)

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A whole run without `mmdc` on PATH aborts up front with one explicit
message and a nonzero exit in both modes (browser never opens), and one
end-to-end subprocess-level test proves a mixed document (one valid + one
invalid diagram) renders to completion with the artifact present — the
observable boundary is the process finishing.

**Architecture:** the gate is a pre-flight check in `cmd/mdsick/root.go`,
per run, before any rendering. It keys off cheap source scanning
(```` ```mermaid ```` occurrence) in stdin mode and off "any `*.md` exists"
in batch mode, so transformer code stays exit-free for the missing-tool case
(Legend [**gate**]). The regression test builds the mdsick binary, runs it as
a subprocess with a fake `mmdc` on PATH and a context deadline, parses the
child's printed temp workspace from its stderr buffer, and asserts on
`rendered.html`.

**Tech Stack:** Go stdlib (`os/exec`, `context`), the shim pattern from
sub-plans 01/02, `context` deadline to fail fast on any reintroduced hang.

---

## Legend decisions carried by this sub-plan

- [**gate**] pre-flight `exec.LookPath("mmdc")` check; once per run; stdin mode only when the source plausibly contains mermaid fences; batch mode when at least one `*.md` sits in the cwd; on miss: one explicit stderr message, nonzero exit, nothing opened.
- [**test**] subprocess-level, hermetic via the `mmdc` shim; real-mmdc verification stays manual/optional.

**Reading of the exit code.** `run()` and `runDir()` already return errors to
a `main` that exits nonzero (verify in `cmd/mdsick/main.go` while wiring);
the gate simply becomes the first error a misconfigured run can produce, and
the browser `open` call is never reached.

---

## Task 1: The gate in `root.go`

**Files:**

- Modify: `cmd/mdsick/root.go` (`run()`, `runDir()`, plus two new helpers)
- Test: `cmd/mdsick/root_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `cmd/mdsick/root_test.go` (add `"path/filepath"` to the import block):

```go
func TestHasMermaid(t *testing.T) {
	for _, test := range []struct {
		name string
		doc  string
		want bool
	}{
		{name: "fence", doc: "x\n\n```mermaid\ngraph TD\n```\n", want: true},
		{name: "tilde fence does not count", doc: "~~~mermaid\ngraph TD\n~~~\n", want: false},
		{name: "no mermaid", doc: "# hi\n\nbody\n", want: false},
		{name: "inline mention", doc: "use ```mermaid fences\n", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := hasMermaid([]byte(test.doc)); got != test.want {
				t.Fatalf("hasMermaid:\ngot %v\nexp %v", got, test.want)
			}
		})
	}
}

func TestGateBehaviors(t *testing.T) {
	dir := t.TempDir()

	t.Run("missing mmdc errors", func(t *testing.T) {
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		err := checkMmdc([]byte("```mermaid\ngraph TD\n```\n"))
		if err == nil {
			t.Fatal("expected error with mmdc missing")
		}
		if !strings.Contains(err.Error(), "mmdc") {
			t.Fatalf("error must mention mmdc:\n%v", err)
		}
	})

	t.Run("shim present passes", func(t *testing.T) {
		shim := filepath.Join(dir, "mmdc")
		if err := os.WriteFile(shim, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		if err := checkMmdc([]byte("```mermaid\ngraph TD\n```\n")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("no mermaid skips the gate", func(t *testing.T) {
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		if err := checkMmdc([]byte("# hi\n\nbody\n")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/mdsick/ -run 'TestHasMermaid|TestGateBehaviors' -v`
Expected: FAIL — `undefined: hasMermaid` and `undefined: checkMmdc`.

- [ ] **Step 3: Write the implementation**

Add to `cmd/mdsick/root.go`:

```go
func hasMermaid(source []byte) bool {
	return strings.Contains(string(source), "```mermaid")
}

func checkMmdc(source []byte) error {
	if !hasMermaid(source) {
		return nil
	}
	if _, err := exec.LookPath("mmdc"); err != nil {
		return fmt.Errorf("mdsick: mmdc not found on PATH; mermaid blocks require it")
	}
	return nil
}

func checkMmdcBatch() error {
	if _, err := exec.LookPath("mmdc"); err != nil {
		return fmt.Errorf("mdsick: mmdc not found on PATH; directory markdown may contain mermaid blocks that require it")
	}
	return nil
}
```

`os/exec` is already imported by `root.go`.

Wire the stdin-mode gate into `run()` right after the source is read, and
**before** `tempWorkspace()` and Parse:

```go
	if err := checkMmdc(source); err != nil {
		return err
	}
```

Wire the batch gate into `runDir()` right after the zero-files check (so a
directory without markdown keeps its friendly message, and a missing tool
prevents any rendering and any browser open):

```go
	if err := checkMmdcBatch(); err != nil {
		return err
	}
```

While wiring, confirm `cmd/mdsick/main.go` exits nonzero on a `run` error —
that is what turns the gate's message into the issue's "one explicit
message, exit nonzero, open nothing".

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/mdsick/ -v && go vet ./...`
Expected: PASS; vet clean.

- [ ] **Step 5: Commit with caveman-commit**

Temp-file pattern from `core-commands` (mktemp → Read → Write →
`jj describe --stdin < <path> && rm <path> && jj new`).

Subject for this task: `feat: gate mermaid rendering on mmdc presence`

---

## Task 2: The subprocess-level regression test

**Files:**

- Create: `cmd/mdsick/mermaidfail_test.go`

- [ ] **Step 1: Write the test (complete file)**

Create `cmd/mdsick/mermaidfail_test.go`:

```go
package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMixedMermaidRendersEndToEnd(t *testing.T) {
	// Hermetic fake mmdc: an invalid diagram gets diagnostics + exit 1;
	// anything else gets a one-line SVG.
	shimDir := t.TempDir()
	script := []byte(`#!/bin/sh
tmp=$(mktemp)
cat > "$tmp"
if grep -q FAIL "$tmp"; then
	printf 'mermaid: parse error near line 2\n' >&2
	exit 1
fi
printf '<svg xmlns="x"/>\n'
`)
	if err := os.WriteFile(filepath.Join(shimDir, "mmdc"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	doc := "before\n\n```mermaid\nFAIL: bad\n```\n\n```mermaid\ngraph TD; A-->B\n```\n\nafter\n"
	docPath := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(docPath, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(docPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	bin := filepath.Join(t.TempDir(), "mdsick")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	run := exec.CommandContext(ctx, bin)
	run.Stdin = f

	var errBuf strings.Builder
	run.Stderr = &errBuf

	if err := run.Run(); err != nil {
		t.Fatalf("subprocess did not render to completion: %v\n%s", err, errBuf.String())
	}

	// The child chdirs into a fresh temp workspace whose path it prints to
	// stderr (`tempWorkspace`); find that line and read the render from it.
	var tmpPath string
	for _, line := range strings.Split(strings.TrimSpace(errBuf.String()), "\n") {
		if filepath.IsAbs(line) && isDir(line) {
			tmpPath = line
			break
		}
	}
	if tmpPath == "" {
		t.Fatalf("no temp workspace printed:\n%s", errBuf.String())
	}

	html, err := os.ReadFile(filepath.Join(tmpPath, "rendered.html"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(html)
	for _, exp := range []string{
		`class="language-mermaid">1| FAIL: bad`,
		`class="language-text"`,
		"parse error near line 2",
		".svg",
	} {
		if !strings.Contains(got, exp) {
			t.Fatalf("missing %q in:\n%s", exp, got)
		}
	}
}

func isDir(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.IsDir()
}
```

The `context deadline exceeded` failure mode **is** the deadlock regression
signal: any future reintroduction of the collector hang surfaces as the
subprocess failing to finish in 30s, exactly the observable boundary the
issue demands.

Window note inherited from the issue's ground truth (`AGENTS.md`): the run
ends with `open`, so the subprocess (like every real mdsick run) is
macOS-bound; no platform guard is added, matching the project's existing
acceptance.

- [ ] **Step 2: Run the test**

Run: `go test ./cmd/mdsick/ -run TestMixedMermaidRendersEndToEnd -v`
Expected: PASS on the tree that contains sub-plans 01 and 02. (This test is
written *after* those land; its pre-fix "fail" state is only reproducible by
checking out an earlier tree — the deadlock predates this plan. The
execution contract is a mandatory PASS now, and a loud context-deadline
failure if the bug ever returns.)

- [ ] **Step 3: Run the whole battery before closing the issue**

Run: `go build ./cmd/mdsick && go test ./... -timeout 60s && go vet ./... && golangci-lint run ./...`
Expected: PASS everywhere; lint clean except the long-accepted `QF1012`
warning.

- [ ] **Step 4: Commit with caveman-commit**

Temp-file pattern (mktemp → Read → Write → `jj describe --stdin < <path> && rm <path> && jj new`).

Subject for this change: `test: assert mixed mermaid completes as subprocess`

---

## Task 3: Optional real-mmdc manual verification

Not automated (Legend [**test**]). With real (puppeteer-backed) `mmdc` on
PATH:

1. Create a doc with one clean diagram (`graph TD; A-->B`) plus one corrupt
   diagram (`flowchart; nonsense /*{`),
2. Run `go run ./cmd/mdsick < /tmp/x.md`,
3. Expect: temp dir printed, browser opened, the numbered surviving fence
   plus real mmdc diagnostics in a `text` fence next to it, valid diagram as
   SVG, stderr silent about diagram failures.
4. Remove `mmdc` from PATH and rerun: expect the single gate message and
   nonzero exit.

Nothing is committed in this task; it only certifies behavior CI never
exercises.

---

## Handoff

When all tasks are done and committed, tick the 03 line in `00-index.md` in
a separate change (message:
`docs(plans): mark 03-gate-and-regression done in issue-10`) via the
temp-file pattern, run the full battery once more, and the issue is complete.
