package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/malikbenkirane/mdsick/internal/transformers"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/toc"
)

func run() error {
	var b bytes.Buffer
	_, err := io.Copy(&b, os.Stdin)
	if err != nil {
		return err
	}
	source := b.Bytes()

	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithASTTransformers(util.PrioritizedValue{
				Value: &transformers.Mermaid{},
			}),
			parser.WithAutoHeadingID(),
		),
		goldmark.WithExtensions(&toc.Extender{}),
		goldmark.WithExtensions(extension.Linkify),
		goldmark.WithExtensions(extension.Strikethrough),
		goldmark.WithExtensions(extension.Table),
	)

	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, tmp)

	if err := copyStyle(tmp); err != nil {
		return err
	}

	if err := os.Chdir(tmp); err != nil {
		return err
	}

	r := text.NewReader(source)
	doc := md.Parser().Parse(r)

	const dst = "rendered.html"

	if err := render(dst, md, source, doc); err != nil {
		return err
	}

	return exec.Command("open", filepath.Join(tmp, dst)).Run()
}

func render(to string, md goldmark.Markdown, source []byte, doc ast.Node) (err error) {
	f, err := os.Create(to)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	_, err = f.WriteString(fmt.Sprintf(`<!doctype html>
<html>
	<head>
		<link rel="stylesheet" href="%[1]s">
		<style>
			body {
				padding-left: %[2]s;
				padding-right: %[2]s;
				padding-top: %[3]s;
				padding-bottom: %[3]s;
			}
		</style>
	</head>
	<body class="markdown-body">`, style, paddingHorizontal, paddingVertical))
	if err != nil {
		return err
	}

	_, err = f.WriteString(`
	</body>
</html>`)
	if err != nil {
		return err
	}

	if err := md.Renderer().Render(f, source, doc); err != nil {
		return err
	}

	_, err = f.WriteString("</body></html>")
	if err != nil {
		return err
	}

	return nil
}

const (
	style             = "github-markdown.css"
	paddingVertical   = "6.47rem"
	paddingHorizontal = "4rem"
)

func copyStyle(to string) (err error) {
	dst, err := os.Create(filepath.Join(to, style))
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, dst.Close())
	}()

	src, err := os.Open(style)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, src.Close())
	}()

	if _, err = io.Copy(dst, src); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
