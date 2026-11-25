package markdown

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var titleMarkdown = goldmark.New()

// DocTitle returns the document's leading level-1 heading text, or ""
// when the first block is not an h1. It parses raw source with a plain
// goldmark instance so the TOC transformer's heading never leaks.
func DocTitle(source []byte) string {
	doc := titleMarkdown.Parser().Parse(text.NewReader(source))
	heading, ok := doc.FirstChild().(*ast.Heading)
	if !ok || heading.Level != 1 {
		return ""
	}

	var title bytes.Buffer
	if err := ast.Walk(heading, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindText {
			title.Write(n.(*ast.Text).Value(source))
		}
		return ast.WalkContinue, nil
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}

	resolved := util.ResolveNumericReferences(title.Bytes())
	resolved = util.ResolveEntityNames(resolved)
	resolved = util.UnescapePunctuations(resolved)
	return strings.TrimSpace(string(resolved))
}
