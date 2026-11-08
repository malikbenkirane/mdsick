# AGENTS.md

## What this is

`mdsick` is a Go CLI that renders Markdown to GitHub-styled HTML and opens it in the browser. Mermaid code blocks are compiled to SVG via the external `mmdc` CLI and included as images. It has two modes: with piped/redirected stdin it reads the document from stdin (`cat doc.md | mdsick`); with a terminal on stdin it renders every `*.md` in the current directory into a fresh temp dir — one `<base>.html` per file plus an `index.html` listing them — leaving the directory untouched.

## Commands

- Build: `go build ./cmd/mdsick`
- Test all: `go test ./...` (only `toc/` has tests; other packages report `[no test files]`)
- Vet: `go vet ./...` (clean)
- Lint: `golangci-lint run ./...` — there is no config file, so defaults apply (staticcheck active). Exactly one known, accepted warning: QF1012 at `cmd/mdsick/root.go:171` (`WriteString(fmt.Sprintf(...))` vs `fmt.Fprintf`).
- Run locally: `go run ./cmd/mdsick < doc.md` (requires `mmdc` on PATH for mermaid blocks; macOS-only in practice — it shells out to `open` with no cross-platform guard). With stdin attached to a terminal instead, it batch-renders every `*.md` in the cwd.
- VCS: Jujutsu, not git — see Gotchas.

## Repo layout

- `cmd/mdsick/root.go` — the entire CLI. Both modes funnel through `tempWorkspace()` (fresh temp dir printed to stderr, embedded CSS copied into it, `chdir`). With piped stdin: builds goldmark (TOC + Linkify + Strikethrough + Table extensions, `parser.WithAutoHeadingID()`, the mermaid transformer) via `newMarkdown()`, parses and renders `rendered.html`, then `open`s it in the browser. With a terminal on stdin: `runDir()` globs `*.md` in the original cwd and captures it via `os.Getwd`, renders one `<base>.html` per file (per-file errors are logged to stderr and skipped), `writeIndex()` writes an `index.html` linking them, then `open`s it. stdlib `flag`; no subcommands.
- `internal/transformers/mermaid.go` — goldmark AST transformer (concurrency notes in Gotchas).
- `internal/mermaid/compile.go` + `mmdc-config.json` — `mermaid.Compile(src, dst)` embeds `mmdc-config.json` via `go:embed`, writes it to a new temp file per invocation (removed in a defer), and shells out to `mmdc -c <tempconfig> -b #ffffff -i - -o -`. The config themes diagrams GitHub-like (elk layout, base theme, GitHub palette, step curves); edit the JSON to change diagram appearance.
- `style/` — `github-markdown.css` embedded as `style.Github`; the HTML shells (padding, `<link>` to the stylesheet, `body class="markdown-body"`) are hardcoded in `render()` and `writeIndex()` in `root.go`.
- `toc/` — superseded experiment, **not imported anywhere**. Runtime TOCs and heading IDs come from `go.abhg.dev/goldmark/toc` plus goldmark's built-in auto heading IDs in `root.go` — `toc.Id` does not run at runtime. This dir contains that custom slugger with tests and a fixture (`toc.md`) describing its exact behavior: `(`, `)`, `@` are stripped; each space and each `-` becomes one `-`; any other non-alphanumeric (e.g. `<`, `>`) is silently dropped — the dashes in `#with---more-symbols-there` come from the spaces around `<>`, not from the angle brackets; everything is lowercased. Don't wire it back in; treat `go.abhg.dev/goldmark/toc` as the TOC implementation.
- `docs/issues/` — task specs as numbered markdown files.
- `README.md` — install/usage. **Stale in one spot**: its "Flags" section documents a `-title` flag that no longer exists (see Gotchas).

## Gotchas

- The working copy is versioned with **Jujutsu (`jj`)**, not git — there is a `.jj/` dir and no `.git`. Use `jj st`, `jj log`, `jj describe`, `jj commit`. Never run git commands. jj auto-snapshots the working copy into `@` on every invocation.
- **Known bug: a failed mermaid render deadlocks the process.** In `Transform`, error events are logged and `continue`d without incrementing `done` (`internal/transformers/mermaid.go:112-115`), so when `count > 0` and any `Compile` fails (invalid diagram, `mmdc` not on PATH), the collector never reaches `done == count`, blocks on `<-m.event` (mermaid.go:106), and `wg.Wait()` (mermaid.go:159) trips `fatal error: all goroutines are asleep - deadlock!` (exit code 2). Verified experimentally. If you touch this code, decide deliberately whether to fix the `done` counting or keep the skip-and-continue semantics. In terminal/batch mode this is per-batch lethal: one invalid diagram in one file crashes the whole run.
- The title is **not a flag anymore** (commit `feat: title is cat of args`): `flag.Parse()` runs after the document is parsed, and the title is `strings.Join(flag.Args(), " ")`, defaulting to `rendered.html`. No flags are registered, so passing `-anything` (including the old `-title`) makes the flag package exit 2 with "flag provided but not defined". In terminal/batch mode, `runDir()` parses then ignores positional args: each file's `<title>` is its base name, and the index is titled `Index`.
- Order in `run()` matters: temp dir is created, style copied, and `os.Chdir(tmp)` performed **before** `md.Parser().Parse`, so the transformer's SVG files (named `<64-hex-chars>.svg`) land in the temp dir and image nodes reference bare filenames. Keep that ordering when refactoring `root.go`. Both modes share this via `tempWorkspace()`; in terminal/batch mode `runDir()` must glob and capture the original cwd first, because after the chdir it re-reads sources via that captured path.
- `Transform` concurrency: one goroutine per mermaid block (via `wg.Go`), completions sent as `event`s over a buffered channel (cap 1) to a single collector goroutine, released by `m.ready` only **after** the `ast.Walk` has discovered `count`. AST mutation is deferred: `change` structs are collected and `parent.ReplaceChild` applied after the walk completes, because mutating the AST mid-walk is unsafe.
- `render()` writes `</body></html>` before the markdown body and again after it (`cmd/mdsick/root.go:190-201`), so every rendered file (stdin mode and batch mode alike) is technically malformed; browsers recover. Don't "clean this up" without checking the visual result.
- `internal/transformers/mermaid.go:70-78` base64-encodes the SVG into `svg64` that is never used — the image `src` is the SVG filename, not a data URI. Dead code; know it's there before refactoring.
- SVG images are size-capped via a `style` attribute (`max-height: 500px; width: auto;`) set on the image node.

## Conventions

- Commit messages follow conventional-commit style (`feat:`, `fix(mermaid):`, `docs(agents):`), as seen in `jj log`.
- Errors from deferred closes are composed with `errors.Join` (see `render()`, `copyStyle()`, `Compile()`).
- Skippable errors are logged to stderr with `fmt.Fprintln` and execution continues — at least, that is the intent (see the deadlock bug above).
- External deps: goldmark (parsing/AST), `go.abhg.dev/goldmark/toc`, `schollz/progressbar/v3` for CLI progress. Go 1.26; module path `github.com/malikbenkirane/mdsick`, imports of internal packages must match.
- Tests are table-driven with expected output pairs; see `toc/toc_test.go` for the style.
