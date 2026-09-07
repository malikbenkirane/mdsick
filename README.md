# mdsick

Render Markdown to GitHub-styled HTML and open it in your browser, straight from stdin.

Mermaid code blocks are compiled to SVG (via
[mermaid-cli](https://github.com/mermaid-js/mermaid-cli)) and inlined as images.

Tables, strikethrough, autolinks and a table of contents are supported via
goldmark extensions.

## Install

Requires Go 1.26+:

```sh
go install github.com/malikbenkirane/mdsick/cmd/mdsick@latest
```

For Mermaid support, [mermaid-cli](https://github.com/mermaid-js/mermaid-cli) must be installed and on your `PATH`:

```sh
npm install -g @mermaid-js/mermaid-cli
```

or

```sh
bun i -g @mermaid-js/mermaid-cli
```

## Usage

```sh
cat README.md | mdsick
```

The rendered HTML is written to a temp directory (path is printed to stderr) and
opened in your browser. Mermaid blocks render with a progress bar; SVGs inline
as images capped at 500px height.

### Flags

| Flag | Description |
|------|-------------|
| `-title` | Document title (default `rendered.html`) |

```sh
cat notes.md | mdsick -title "My Notes"
```

## Behavior

- Reads all of stdin, parses with [goldmark](https://github.com/yuin/goldmark),
and writes `rendered.html` into a fresh temp dir styled via the embedded GitHub
Markdown CSS.
- Fenced ```` ```mermaid ```` blocks are replaced with the compiled SVG image.
A failed Mermaid render is logged to stderr and skipped; it does not abort the
document.
- Headings get automatic IDs and a TOC via the community
[goldmark/toc](https://github.com/abhinav/goldmark-toc) extension.
