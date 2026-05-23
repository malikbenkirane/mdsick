package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

func run() error {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	var b bytes.Buffer
	_, err := io.Copy(&b, os.Stdin)
	if err != nil {
		return err
	}
	root := md.Parser().Parse(text.NewReader(b.Bytes()))
	walk(root, "")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func walk(node ast.Node, indent string) {
	fmt.Println(indent, node.Kind())
	if node.HasChildren() {
		child := node.FirstChild()
		for range node.ChildCount() {
			walk(child, indent+"+")
			child = child.NextSibling()
		}
	}
}
