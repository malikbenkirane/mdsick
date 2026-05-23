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
	var b bytes.Buffer
	_, err := io.Copy(&b, os.Stdin)
	if err != nil {
		return err
	}

	transformer := NewMarshal(b.Bytes(), WithTransform())
	transformer.walk()

	printer := NewMarshal(b.Bytes(), WithPrint())
	printer.walk()

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type Marshal struct {
	source    []byte
	indent    string
	transform bool
	print     bool
	root      ast.Node
}

func NewMarshal(source []byte, opts ...MarshalOption) Marshal {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	root := md.Parser().Parse(text.NewReader(source))
	m := Marshal{
		indent: "+",
		root:   root,
		source: source,
	}
	for _, opt := range opts {
		m = opt(m)
	}
	return m
}

func (m Marshal) walk() ast.Node {

	if m.print {
		fmt.Println(m.indent, m.root.Kind())
	}

	if m.root.HasChildren() {

		child := m.root.FirstChild()

		for range m.root.ChildCount() {

			mc := WithRoot(child)(m)
			mc = WithIndentIncrement(" +")(mc)

			trans := mc.walk()

			if m.transform {
				m.root.ReplaceChild(m.root, child, trans)
			}

			child = trans.NextSibling()

		}

	}

	return m.root
}

type MarshalOption func(Marshal) Marshal

func WithTransform() MarshalOption {
	return func(m Marshal) Marshal {
		m.transform = true
		return m
	}
}
func WithPrint() MarshalOption {
	return func(m Marshal) Marshal {
		m.print = true
		return m
	}
}
func WithRoot(r ast.Node) MarshalOption {
	return func(m Marshal) Marshal {
		m.root = r
		return m
	}
}
func WithIndentIncrement(incr string) MarshalOption {
	return func(m Marshal) Marshal {
		m.indent += incr
		return m
	}
}
