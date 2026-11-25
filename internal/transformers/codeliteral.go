package transformers

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

var KindCodeLiteral = ast.NewNodeKind("CodeLiteral")

// codeLiteral carries literal content that has no counterpart in the
// document source: goldmark segments resolve against the source buffer, so
// cannot host synthesized lines.
type codeLiteral struct {
	ast.BaseBlock
	lang  string
	lines []string
}

func (c *codeLiteral) Kind() ast.NodeKind { return KindCodeLiteral }

func (c *codeLiteral) IsRaw() bool { return true }

func (c *codeLiteral) Dump(source []byte, level int) {}

// CodeLiteralRenderer renders codeLiteral exactly like the default HTML
// renderer renders a fenced code block with the same language and lines.
type CodeLiteralRenderer struct{}

func (CodeLiteralRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindCodeLiteral, renderCodeLiteral)
}

func renderCodeLiteral(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		w.WriteString("</code></pre>\n")
		return ast.WalkContinue, nil
	}

	c := node.(*codeLiteral)
	if c.lang != "" {
		w.WriteString(`<pre><code class="language-`)
		w.WriteString(c.lang)
		w.WriteString(`">`)
	} else {
		w.WriteString("<pre><code>")
	}
	for _, line := range c.lines {
		w.Write(util.EscapeHTML([]byte(line)))
		w.WriteString("\n")
	}
	return ast.WalkContinue, nil
}
