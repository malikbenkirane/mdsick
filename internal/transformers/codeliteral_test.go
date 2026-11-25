package transformers

import (
	"bytes"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

func TestCodeLiteralRendering(t *testing.T) {
	doc := ast.NewDocument()
	blk := &codeLiteral{lang: "mermaid", lines: []string{"1| flowchart A", "2| <emo> & stuff", "3| end"}}
	blk.SetBlankPreviousLines(true)
	doc.AppendChild(doc, blk)

	md := goldmark.New(
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.PrioritizedValue{
				Value: CodeLiteralRenderer{},
			}),
		),
	)
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, nil, doc); err != nil {
		t.Fatal(err)
	}

	exp := "<pre><code class=\"language-mermaid\">1| flowchart A\n2| &lt;emo&gt; &amp; stuff\n3| end\n</code></pre>\n"
	if buf.String() != exp {
		t.Fatalf("unexpected rendering:\ngot %q\nexp %q", buf.String(), exp)
	}
}
