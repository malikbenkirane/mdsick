# AGENTS.md

## What this is

`mdsick` is a Go CLI that reads Markdown from stdin, renders it to a GitHub-styled HTML file, and opens it in the browser. Mermaid code blocks are compiled to SVG via the external `mmdc` CLI and inlined as images. It is a single-purpose tool, driven entirely by pipes: `cat doc.md | mdsick`.

## Commands

- Build: `go build ./cmd/mdsick`
- Test all: `go test ./...`
- Run locally: `go run ./cmd/mdsick < doc.md` (requires `mmdc` on PATH for mermaid blocks; requires macOS `open`)
- Format: `gofmt` / `gofumpt` as usual; linting is done with golangci-lint (staticcheck is active — a known QF1012 warning exists in `cmd/mdsick/root.go`).

## Repo layout

- `cmd/mdsick/root.go` — entire CLI: parses stdin with goldmark, writes `rendered.html` into a fresh temp dir (printed to stderr), embeds styles, opens in browser. Uses stdlib `flag` (not cobra); no subcommands.
- `internal/transformers/mermaid.go` — goldmark AST transformer. Concurrent: spawns one goroutine per mermaid block, communicates completions through the `event` channel, collects `change` structs, and replaces fenced blocks with `ast.NewImage` links only *after* the walk completes. Mutation is deferred because mutating the AST mid-walk is unsafe.
- `internal/mermaid/compile.go` — thin wrapper invoking `mmdc -i - -o -` (mermaid-cli must be installed separately).
- `style/` — `github-markdown.css` embedded via `go:embed` into `style.Github`; HTML shell (padding, stylesheet link) is hard-coded in `render()`.
- `toc/` — superseded experiment, **not imported anywhere**. TOCs are handled by the community extension `go.abhg.dev/goldmark/toc` in `root.go`. This dir contains a custom heading-ID slugger (`toc.Id`) with tests and a fixture (`toc.md`) describing exact slug behavior (spaces and dashes each become one `-`, `@() `*are* stripped while punctuation like `< >` collapses to `-`). Don't wire it back in; treat `go.abhg.dev/goldmark/toc` as the TOC implementation.
- `docs/issues/` — task specs; issues are numbered markdown files.

## Gotchas

- The working copy is versioned with **Jujutsu (`jj`)**, not git — there is a `.jj/` dir and no `.git`. Use `jj st`, `jj commit`, `jj describe` etc. Never run git commands.
- `Transform` relies on channels + `sync.WaitGroup` where the collector goroutine exits when `done == count`; if you touch this code, remember `count` is discovered during the `ast.Walk` *before* `m.ready` is signaled, and errors from renders are logged to stderr and skipped (render failure never aborts the whole render).
- SVG files are written to the current working directory (the temp dir created in `run()`) — the transformer assumes the chdir already happened. Keep that ordering in mind when refactoring `root.go`.
- The `-title` flag is parsed late, after parsing stdin; flags are only for the document title.
- Go version 1.26; module path is `github.com/malikbenkirane/mdsick`, imports of internal packages must match.

## Conventions

- External deps: goldmark (parsing/AST), goldmark `toc.Extender`, `progressbar/v3` for CLI progress.
- Errors are typically composed with `errors.Join` in deferred closes.
- Tests are table-driven with expected output pairs; see `toc/toc_test.go` for the style.
