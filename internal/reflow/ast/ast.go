// Package ast exposes a textual dump of goldmark's AST for a markdown
// source, so the exact node structure produced by the parser can be
// pinned down by tests.
package ast

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// Dump parses source with a plain goldmark instance plus the GFM table
// extension and returns an indented tree of node kinds. Text nodes carry
// their source content and line-break flags.
func Dump(source []byte) string {
	md := goldmark.New(goldmark.WithExtensions(extension.Table))
	doc := md.Parser().Parse(text.NewReader(source))
	var sb strings.Builder
	walk(doc, source, 0, &sb)
	return sb.String()
}

func walk(n ast.Node, source []byte, depth int, sb *strings.Builder) {
	line := strings.Repeat("  ", depth) + n.Kind().String()
	switch t := n.(type) {
	case *ast.Text:
		line += fmt.Sprintf(" %q", t.Segment.Value(source))
		var tags []string
		if t.SoftLineBreak() {
			tags = append(tags, "soft")
		}
		if t.HardLineBreak() {
			tags = append(tags, "hard")
		}
		if len(tags) > 0 {
			line += " (" + strings.Join(tags, ",") + ")"
		}
	case *ast.Heading:
		line += fmt.Sprintf(" level=%d", t.Level)
	case *ast.FencedCodeBlock:
		line += fmt.Sprintf(" language=%q", t.Language(source))
	}
	sb.WriteString(line)
	sb.WriteByte('\n')
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		walk(c, source, depth+1, sb)
	}
}
