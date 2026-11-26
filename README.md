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

```sh
cat doc.md | mdsick-reflow | pbcopy
```

Refills hard-wrapped Markdown so single newlines don't turn into hard
breaks when pasted into GitHub comment boxes.

The rendered HTML is written to a temp directory (path is printed to stderr) and
opened in your browser. Mermaid blocks render with a progress bar; SVGs inline
as images capped at 500px height.

### Directory mode

If stdin is a terminal (no pipe or redirection), mdsick does not block waiting
for input: it renders every `*.md` file in the current directory instead. Like
stdin mode, all output goes to a fresh temp directory (path printed to stderr) —
one `<doc>.html` per file plus an `index.html` listing them all, which is then
opened in your browser. The current directory is left untouched. Files are
listed in natural order (`2.md` before `10.md`), and a file that starts with a
`# Heading 1` shows that title next to its name in the index.

```sh
mdsick
```

### Arguments

Positional arguments set the title of the generated HTML page. In stdin mode
they title `rendered.html` (default `rendered.html`); in directory mode they
title `index.html` (default `Index`).

```sh
cat notes.md | mdsick "My Notes"
mdsick "My Notes"
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
