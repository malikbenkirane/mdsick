package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/malikbenkirane/mdsick/internal/transformers"
	"github.com/yuin/goldmark"
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

	if err := os.Chdir(tmp); err != nil {
		return err
	}

	r := text.NewReader(source)
	doc := md.Parser().Parse(r)

	f, err := os.Create("rendered.html")
	if err != nil {
		return err
	}

	if err := md.Renderer().Render(f, source, doc); err != nil {
		return err
	}

	return exec.Command("open", filepath.Join(tmp, f.Name())).Run()
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
