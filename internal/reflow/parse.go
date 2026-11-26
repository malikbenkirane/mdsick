package reflow

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

func parse(source []byte) ast.Node {
	md := goldmark.New(goldmark.WithExtensions(extension.Table))
	return md.Parser().Parse(text.NewReader(source))
}
