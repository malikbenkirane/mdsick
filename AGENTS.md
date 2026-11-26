# AGENTS.md

## What this is

`mdsick` is a pair of Go CLIs that render Markdown to GitHub-styled HTML and open it in the browser.

- `cmd/mdsick` — main viewer. With piped/redirected stdin it renders the document to `rendered.html`; with a terminal on stdin it batch-renders every `*.md` in the cwd into a fresh temp dir (one `<base>.html` per file plus an `index.html`), leaving the directory untouched.
- `cmd/mdsick-issues` — cobra-based tool that renders `docs/issues/` and `docs/issues/closed/` into flat per-issue pages (`NN.html`) plus a landing page with Open/Closed sections and a warnings table. `--vcs auto|jj|git` picks repo detection (auto: `.jj` wins).

Mermaid code blocks are compiled to SVG via the external `mmdc` CLI and included as images. Pre-flight `checkMmdc` gates stdin mode only when the source contains a mermaid fence, and always gates batch mode.

## Commands

- Build: `go build ./...` (two binaries)
- Test all: `go test ./...` — every package has tests except `style/`
- Vet: `go vet ./...` (clean)
- Lint: `golangci-lint run ./...` — no config file, defaults apply. Exactly one known, accepted warning: QF1012 at `internal/workspace/workspace.go:73` (`WriteString(fmt.Sprintf(...))` vs `fmt.Fprintf`).
- Run main CLI: `go run ./cmd/mdsick < doc.md` (macOS-only in practice — shells out to `open`). With stdin attached to a terminal, batch mode over the cwd.
- Run issues CLI: `go run ./cmd/mdsick-issues` from the repo root (renders `docs/issues/`).

## Repo layout

- `cmd/mdsick/root.go` — main CLI orchestration: mode selection via `stdinIsTerminal()`, batch glob + `natural.Compare` sort + per-file render (per-file errors logged and skipped), `writeIndex()` (each entry shows its leading `# Heading 1` when present). stdlib `flag`, no subcommands, no registered flags.
- `cmd/mdsick-issues/` — `main.go` (cobra command + `--vcs`), `index.go` (landing page: Open/Closed sections in reverse natural order), `warnings.go` (folded warnings table; warnings link to the offending issue page). Issue rows are rendered before the index; mutable `Row.Head` values are updated via `setHead` afterward so heads still appear on the index.
- `internal/workspace/` — `TempWorkspace()` (fresh temp dir printed to **stderr**, embedded CSS copied, `os.Chdir` in), `CopyStyle`, `RenderFile`. `PaddingHorizontal` is `"6.47rem"`. Both binaries depend on this global chdir.
- `internal/markdown/` — `New()` (goldmark: toc extender + Linkify + Strikethrough + Table + `parser.WithAutoHeadingID()` + mermaid transformer) and `NewIssues(index, source, &warnings)` (same, plus an issues transformer at priority 100, after Mermaid's 0). `DocTitle(source)` uses a separate plain `goldmark.New()` so the toc-injected `Table of Contents` heading never leaks into titles; resolves numeric refs/entities/escapes; `""` if no leading h1.
- `internal/issues/` — repo detection (`FindRepo`), `List(root)` → open/closed `[]Row` (sorted descending, natural order), ref resolution, warnings (`Warning{Source, Ref}`, collected through a shared slice via `NewTransformer`). Resolves `#NN`, `docs/issues[/closed]/NN.md` paths (incl. inside code spans), bare `NN.md`, and `issue [closed] NN` prose — first matching tier wins per Text node; fenced code blocks are skipped; unresolvable refs produce warnings, not dropped links.
- `internal/transformers/mermaid.go` — goldmark AST transformer for mermaid blocks (concurrency in Gotchas). `codeliteral.go` — custom `codeLiteral` AST node + `CodeLiteralRenderer` producing byte-identical fenced-code HTML (synthesized lines can't ride goldmark source-offset blocks); `Dump` is a no-op, debugger-unfriendly.
- `internal/mermaid/compile.go` + `mmdc-config.json` — `Compile(src, dst)` embeds the config via `go:embed`, writes a fresh temp config per call, and runs `mmdc -c <tempconfig> -b #ffffff -i - -o -`. Nonzero exit returns `*mermaid.Failure{Err, Output}` (use `errors.As`); stdout+stderr merged into one buffered output. Edit the JSON to change diagram appearance.
- `internal/natural/` — `Compare(a, b)`: byte-based natural sort; digit runs compare numerically with leading zeros trimmed; ASCII-only (no unicode digits).
- `style/` — `github-markdown.css` embedded as `style.Github`; HTML shells (padding, `<link>`, `body class="markdown-body"`) live in `workspace.RenderFile` and the index writers.
- `toc/` — superseded experiment, **not imported anywhere**. Runtime TOCs and heading IDs come from `go.abhg.dev/goldmark/toc` plus goldmark's auto heading IDs. This dir holds a custom slugger with tests and a fixture (`toc.md`): `(`, `)`, `@` stripped; each space and each `-` becomes one `-`; any other non-alphanumeric silently dropped (the dashes in `#with---more-symbols-there` come from the spaces around `<>`); everything lowercased. Don't wire it back in.
- `toc_test/` and `.kilo/plans/` — **empty directories**, referenced by nothing; leftovers, safe to delete.
- `docs/issues/NN.md` (open) and `docs/issues/closed/NN.md` (closed) — task specs; first line is `# Title`. `docs/plans/YYYYMMDDTHHMM_issue-NN_slug/` holds execution plans: `00-index.md`, numbered phase files, plus `deviations.md` and `pr.md` after execution. Plans are re-planned (new dated dir) rather than edited in place; see the two issue-9 plan dirs.
- `README.md` — install/usage.

## Mermaid failure semantics (the old deadlock is fixed)

The formerly documented "failed render deadlocks the process" bug is fixed (closed issue 10; see `docs/plans/*issue-10*/pr.md`). Current behavior:

- Per mermaid block: one goroutine (`wg.Go`), completions sent as events over a buffered channel (cap 1) to a single collector, released by `m.ready` only **after** `ast.Walk` has discovered `count`. AST mutation is deferred: `change` structs are collected and `parent.ReplaceChild` applied after the walk completes — mutating the AST mid-walk is unsafe.
- The collector completes when `done + errCount + infraCount == count`; every outcome increments the progress bar.
- **Diagram failure** (invalid mermaid, `*Failure`): logged and turned into an artifact in place — a log fence plus a numbered fail fence showing the source as `N| line` and the mmdc diagnostic. Rendering continues.
- **Infra failure** (e.g. `mmdc` missing): counted and reported via injectable `fatal(exit)` at the end (`os.Exit` stays out of tests).
- `mermaid.go` still base64-encodes into `svg64` (~lines 108–116), which is **never used** — the image `src` is the SVG filename, not a data URI — and the encoder error path is effectively ignorable. Dead code, tracked in open issue 11.
- SVG images are size-capped via a `style` attribute (`max-height: 500px; width: auto;`) on the image node.

## Gotchas

- The working copy is versioned with **Jujutsu (`jj`)**, not git — there is a `.jj/` dir and no `.git`. Use `jj st`, `jj log`, `jj describe`, `jj commit`. Never run git commands; jj auto-snapshots the working copy into `@` on every invocation.
- Order matters: the temp dir must be created and `os.Chdir` performed **before** parsing, so the transformer's SVG sidecars (named `<64-hex>.svg`) land there as bare filenames that image nodes reference. Anything that globs or reads input sources (batch mode, issues CLI) must capture the original cwd **before** calling `TempWorkspace()` and re-read via that captured path — several call sites carry comments spelling this out.
- The title is positional, not a flag: `flag.Parse()` runs after parsing, and the title is `strings.Join(flag.Args(), " ")` (stdin default `rendered.html`, batch default `Index`, issues title derived from repo meta). No flags are registered in `cmd/mdsick`, so `-anything` exits 2 with "flag provided but not defined". In stdin mode the title is HTML-escaped before use.
- `workspace.RenderFile` deliberately writes `</body></html>` before the markdown body **and** after it, so every rendered file is technically malformed; browsers recover. Documented in-code — don't clean it up without checking the visual result.
- `hasMermaid` string-sniffs only the exact prefix ` ```mermaid ` — indented or spaced fence variants miss the pre-flight check.
- `mdsick-issues`: `issues.List` **errors on any non-`NN.md` filename** in `docs/issues/` (a stray README breaks the tool) and on duplicate numbers across sides; the prose-tier regex also resolves bare `NN.md` anywhere (open issue 14 tracks prose refs). Legacy format like `NN-side-title.md` or `NN-title.md` will fail.
- The `open` invocation hardcodes macOS (open issue 7); no cross-platform guard. `cmd/mdsick` `main` prints errors with `fmt.Println` (stdout, not stderr) — quirk, not intent.
- Errors from deferred closes are composed with `errors.Join` (e.g. `Compile`, `RenderFile`).
- External deps: goldmark, `go.abhg.dev/goldmark/toc`, `schollz/progressbar/v3`, `spf13/cobra` (only `mdsick-issues`), `golang.org/x/net` (via Linkify). Go 1.26; module `github.com/malikbenkirane/mdsick` — internal imports must match.

## Conventions

- Commit messages follow conventional-commit style (`feat:`, `fix(mermaid):`, `docs(issues):`), as seen in `jj log`.
- Tests are table-driven with got/expected pairs (see `toc/toc_test.go` for the style). Newer patterns: `t.Setenv` PATH shims running a fake `mmdc` shell script (transformers tests), `t.Cleanup`-based temp-dir helpers, injectable `fatal`/output hooks keeping `os.Exit` out of tests.
- Skippable errors are logged to stderr with `fmt.Fprintln` and execution continues; per-file/per-issue failures never abort a batch.
