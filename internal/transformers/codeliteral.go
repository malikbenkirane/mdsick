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
		_, err := w.WriteString("</code></pre>\n")
		return ast.WalkContinue, err
	}

	c := node.(*codeLiteral)
	if c.lang != "" {
		_, err := w.WriteString(`<pre><code class="language-`)
		if err != nil {
			return ast.WalkContinue, err
		}
		_, err = w.WriteString(c.lang)
		if err != nil {
			return ast.WalkContinue, err
		}
		_, err = w.WriteString(`">`)
		if err != nil {
			return ast.WalkContinue, err
		}
	} else if _, err := w.WriteString("<pre><code>"); err != nil {
		return ast.WalkContinue, err
	}
	for _, line := range c.lines {
		if _, err := w.Write(util.EscapeHTML([]byte(line))); err != nil {
			return ast.WalkContinue, err
		}
		if _, err := w.WriteString("\n"); err != nil {
			return ast.WalkContinue, err
		}
	}
	return ast.WalkContinue, nil
}
