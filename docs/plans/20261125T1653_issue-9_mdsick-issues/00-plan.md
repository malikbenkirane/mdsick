# Warnings table (issue 9, post-grill) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the warnings `<pre>` block on the `mdsick-issues` landing page with a two-column HTML table: one deduplicated, deterministically sorted row per broken reference, with the file cell linking to its flat page (`<a href="N.html">#N</a>`) and the ref cell wrapped in `<code>`.

**Grill verdict (settled decisions):**
- Q1.a: two columns **File | Ref**; the constant "unresolvable" text is dropped.
- Q1.b: fold duplicate `(source, ref)` pairs into one row; sort by File (natural) then Ref — deterministic, independent of first-occurrence order.
- Q1.c: the File cell is a link to the companion flat page, label `#N` (the issue number), **not** `N.md`; the Ref cell is `<code>`-wrapped plain text, never a link (its target does not exist).
- Q1.d: when there are no warnings, omit the whole block (heading included) — presence *is* the signal.
- Q1.e: amend the spec bullets in `docs/issues/9.md` (this is a spec fix, not a deviation: markdown-body table styling is still "no bespoke alert styling"); also refresh the PR doc's warnings description.

**Tech Stack:** Go 1.26 (slices, strconv), `internal/natural.Compare` for the File sort, `internal/issues` types unchanged, existing `cmd/mdsick-issues` tests, Jujutsu via `core-commands`.

**Parsing note:** `Warning.Source` is always a base filename `NN.md` with an all-digit base (enforced by `issues.List`), so the table writer derives the issue number from it; if parsing ever fails the File cell falls back to plain text `Source` (no link), never a crash.

---

## Current state (read this first)

- `cmd/mdsick-issues/index.go` emits, only `if len(warns) > 0`:
  `"\t\t<h2>Warnings</h2>\n\t\t<pre>\n"` + one escaped `w.String()` line per warning + `"\t\t</pre>\n"`.
- `Warning{Source, Ref}` (`internal/issues/resolver.go`) currently renders as `"%s: %s — unresolvable"` via `String()`; `String()` is used **only** by the index writer and its test after this plan — decide its fate in Task 3 (delete it, "unresolvable" wording no longer appears).
- `cmd/mdsick-issues/run_test.go` asserts `"43.md: #99"` in the index HTML — this assertion dies in Task 2 (the string no longer exists); its replacement asserts the table row's parts.
- `docs/issues/9.md` warnings bullet says: "plain `## Warnings` section … no bespoke alert styling" — Task 3 rewrites that bullet and the rewriting-table row in the "Link rewriting" prose intro is unaffected.
- `docs/plans/20261125T1623_issue-9_mdsick-issues/pr.md` describes `<h2>Warnings</h2>` + pre-style block and quotes `9.md: #44 — unresolvable` in the walkthrough — Task 3 refreshes both spots.
- Deduplication input: duplicate warnings arise because *each mention* of an unresolvable ref in one file warns (the smoke run warned `9.md: 8` twice). Folding is a presentation concern of the landing page, so it lives in `cmd/mdsick-issues`, not in the transformer.

---

### Task 1: Fold and sort warnings in `cmd/mdsick-issues`

**Files:**
- Create: `cmd/mdsick-issues/warnings.go`
- Test: `cmd/mdsick-issues/warnings_test.go`

- [ ] **Step 1: Write the failing test**

```go
package main

import "testing"

import (
	"github.com/malikbenkirane/mdsick/internal/issues"
)

func TestFoldWarnings(t *testing.T) {
	for _, test := range []struct {
		name string
		in   []issues.Warning
		want []issues.Warning
	}{
		{
			name: "duplicates fold, natural file order, ref order",
			in: []issues.Warning{
				{Source: "9.md", Ref: "36"},
				{Source: "43.md", Ref: "#9"},
				{Source: "9.md", Ref: "#8"},
				{Source: "9.md", Ref: "36"},
				{Source: "13.md", Ref: "#21"},
				{Source: "13.md", Ref: "#2"},
			},
			want: []issues.Warning{
				{Source: "9.md", Ref: "#8"},
				{Source: "9.md", Ref: "36"},
				{Source: "43.md", Ref: "#9"},
				{Source: "13.md", Ref: "#2"},
				{Source: "13.md", Ref: "#21"},
			},
		},
		{
			name: "empty stays empty",
			in:   nil,
			want: nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := foldWarnings(test.in)
			if len(got) != len(test.want) {
				t.Fatalf("foldWarnings:\ngot %v\nexp %v", got, test.want)
			}
			for i := range test.want {
				if got[i] != test.want[i] {
					t.Fatalf("foldWarnings[%d]:\ngot %v\nexp %v", i, got[i], test.want[i])
				}
			}
		})
	}
}
```

Note: `13.md` sorts *after* `9.md` numerically but the smoke run already
renders files in reverse order; the fold is presentation-ordered
(ascending natural, one File column read top-down) — this test pins that
`natural.Compare` ascending order and that refs within one file sort
lexically (`#2` before `#21`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/mdsick-issues/ -run TestFoldWarnings -v`
Expected: FAIL (`foldWarnings` undefined).

- [ ] **Step 3: Implement**

Create `cmd/mdsick-issues/warnings.go`:

```go
package main

import (
	"slices"

	"github.com/malikbenkirane/mdsick/internal/issues"
	"github.com/malikbenkirane/mdsick/internal/natural"
)

// foldWarnings deduplicates exact (source, ref) pairs — each *mention*
// of one broken ref produces its own warning — and orders rows
// deterministically: by file (natural), then by ref (lexical).
func foldWarnings(warns []issues.Warning) []issues.Warning {
	seen := make(map[issues.Warning]bool)
	var rows []issues.Warning
	for _, w := range warns {
		if seen[w] {
			continue
		}
		seen[w] = true
		rows = append(rows, w)
	}
	slices.SortStableFunc(rows, func(a, b issues.Warning) int {
		if c := natural.Compare(a.Source, b.Source); c != 0 {
			return c
		}
		return natural.Compare(a.Ref, b.Ref)
	})
	return rows
}
```

(Comment on the sort if natural-vs-lexical for Ref reads oddly:
`natural.Compare` on refs keeps `#21` before `#99` the same way it keeps
`3.md` before `43.md`; falling back to it keeps one comparator, no
separate `cmp.Reduce` ladder.)

- [ ] **Step 4: Run tests**

Run: `go test ./cmd/mdsick-issues/`
Expected: PASS (index tests unchanged so far).

- [ ] **Step 5: Commit with caveman-commit**

Message: `feat: fold duplicate warnings`

---

### Task 2: Render the `<h2>Warnings</h2>` table

**Files:**
- Modify: `cmd/mdsick-issues/index.go` (replace the `<pre>` emission)
- Test: `cmd/mdsick-issues/index_test.go` (updated assertions)

- [ ] **Step 1: Update the tests to the new contract (failing first)**

Replace the warnings assertions in `TestWriteIssuesIndex`:

- Keep: `"<h2>Warnings</h2>"` present when warnings exist.
- Replace `"3.md: #44"` with both row parts:
  `` `<a href="3.html">#3</a>` `` (File cell) and
  `` `<code>#44</code>` `` (Ref cell).
- Add a stale-`<pre>` tripwire: `strings.Contains(h, "<pre>")` must be **false**.
- Add a duplicate-fold inclusion: the `warns` fixture gains a second
  `{Source: "3.md", Ref: "#44"}` and the assertion becomes: exactly one
  `3.html` anchor in the warnings table region — simplest robust check is
  `strings.Count(h, `<code>#44</code>`) == 1`.
- In `TestBuildWarningsSurfaceOnIndex` (run_test.go): replace
  `"43.md: #99"` with `` `<code>#99</code>` `` and
  `` `<a href="43.html">#43</a>` ``.

Run: `go test ./cmd/mdsick-issues/`
Expected: FAIL (writer still emits `<pre>`, anchors missing, no `#4` label).

- [ ] **Step 2: Implement the table writer**

In `cmd/mdsick-issues/index.go`, replace the warnings block with:

```go
	if len(warns) > 0 {
		if err := writeWarningsTable(f, foldWarnings(warns)); err != nil {
			return err
		}
	}
```

and add:

```go
// issueNumberOf extracts the flat-page number from a warning's source
// file ("43.md" -> 43). The List contract guarantees all-digit bases;
// ok=false is a defensive fallback for direct callers.
func issueNumberOf(source string) (n int, ok bool) {
	base := strings.TrimSuffix(source, ".md")
	n, err := strconv.Atoi(base)
	if err != nil || base == "" {
		return 0, false
	}
	return n, true
}

func writeWarningsTable(f *os.File, rows []issues.Warning) (err error) {
	if _, err = fmt.Fprint(f, "\t\t<h2>Warnings</h2>\n\t\t<table>\n\t\t\t<thead>\n\t\t\t\t<tr><th>File</th><th>Ref</th></tr>\n\t\t\t</thead>\n\t\t\t<tbody>\n"); err != nil {
		return err
	}
	for _, w := range rows {
		cell := fmt.Sprintf(`<td>%s</td>`, html.EscapeString(w.Source))
		if n, ok := issueNumberOf(w.Source); ok {
			cell = fmt.Sprintf(`<td><a href="%d.html">#%d</a></td>`, n, n)
		}
		if _, err = fmt.Fprintf(f, "\t\t\t\t<tr>%s<td><code>%s</code></td></tr>\n", cell, html.EscapeString(w.Ref)); err != nil {
			return err
		}
	}
	_, err = fmt.Fprint(f, "\t\t\t</tbody>\n\t\t</table>\n")
	return err
}
```

(Add `"strconv"` to the import list.)

- [ ] **Step 3: Delete `Warning.String()`**

Remove `String()` from `internal/issues/resolver.go` and drop its test
usage if any (check `grep -rn "\.String()" internal/issues cmd/mdsick-issues`
first — expect only the old index emission). The "unresolvable" wording
migrates to the spec bullet (Task 3), the heading + table headers
communicate it at render time.

Run: `go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 4: Commit with caveman-commit**

Message: `feat: warnings table on index`

---

### Task 3: Amend the spec and refresh the PR doc

**Files:**
- Modify: `docs/issues/9.md`
- Modify: `docs/plans/20261125T1623_issue-9_mdsick-issues/pr.md`

- [ ] **Step 1: Rewrite the warnings bullet in `docs/issues/9.md`**

Current bullet:

```
- Warnings block as a plain `## Warnings` section at the bottom of the
  index, `markdown-body` styling only (no bespoke alert styling).
```

Replacement:

```
- Warnings block as an `<h2>Warnings</h2>` HTML table at the bottom of
  the index, `markdown-body` styling only (no bespoke alert styling).
  Two columns — **File | Ref** — one row per distinct broken
  reference, deduplicated across repeated mentions and sorted by file
  (natural) then ref. The File cell links the issue's flat page
  (`<a href="N.html">#N</a>`); the Ref cell is `<code>`-wrapped text and
  is never linked (its target does not resolve). With zero warnings the
  whole block is omitted; an empty page never carries an empty table.
```

Also, in the "Unresolvable references …" paragraph just above, replace
"they collect into a **warnings section at the bottom of the index
page**" with "**the Warnings table at the bottom of the index page** (one
deduplicated row per broken reference)" — same paragraph, it still says
references "are not dropped", which remains true.

- [ ] **Step 2: Refresh `pr.md`**

In `docs/plans/20261125T1623_issue-9_mdsick-issues/pr.md`:
1. The "warnings block so broken refs surface" sentence in section 3:
   describe the two-column table (fold-duplicated rows, `#N` file link,
   `<code>` ref cell, omitted when empty).
2. The smoke-run walkthrough item "a Warnings block flagging
   unresolvable `#NN`/path references": append "(rendered as the two-column
   warnings table)".
3. The example string `9.md: #44 — unresolvable` in that doc: replace with
   the row form `<a href="9.html">#9</a>` + `<code>#44</code>`.

- [ ] **Step 3: Commit**

Message: `docs(issues): spec warnings as table`

---

### Task 4: Final sweep

**Files:** none

- [ ] **Step 1: Full suite**

Run: `go test ./... && go vet ./... && golangci-lint run ./...`
Expected: tests pass, vet clean, only the accepted QF1012 in
`internal/workspace/workspace.go`.

- [ ] **Step 2: Smoke against the real repo**

Run: `go run ./cmd/mdsick-issues`
Expected: Warnings table with two columns, no duplicate rows (`9.md`
mentions of `#8`, `#22`, … appear once each), File cells reading `#9`,
`#14`, links pointing at `9.html`, `14.html`; ref cells like `#8`,
`30`, `36` in `<code>`; no `<pre>` remnant.

- [ ] **Step 3: Tick the plan checkboxes**

Mark the run complete; record any smoke surprise in
`docs/plans/20261125T<same>_issue-9_mdsick-issues/deviations.md` if the
table needed a further slip (fold-order surprises, markdown-body table
regression) — spec text stays source-of-truth.

Message: `docs(issues): tick issue 9 warnings-table plan`
