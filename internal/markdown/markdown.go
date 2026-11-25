package markdown

import (
	"github.com/malikbenkirane/mdsick/internal/issues"
	"github.com/malikbenkirane/mdsick/internal/transformers"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/toc"
)

// New builds the goldmark engine used by mdsick binaries: GitHub-like
// rendering (TOC extender, auto heading ids, Linkify, Strikethrough,
// Table) plus the mermaid fence transformer.
func New() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.PrioritizedValue{
				Value: transformers.CodeLiteralRenderer{},
			}),
		),
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
}

// NewIssues is New() plus the issue-reference transformer, which runs
// AFTER the mermaid transformer for the given file (source name is the
// base issue filename, used in warnings).
func NewIssues(index *issues.Index, source string, warnings *[]issues.Warning) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.PrioritizedValue{
				Value: transformers.CodeLiteralRenderer{},
			}),
		),
		goldmark.WithParserOptions(
			parser.WithASTTransformers(
				util.Prioritized(&transformers.Mermaid{}, 0),
				util.Prioritized(issues.NewTransformer(index, source, warnings), 100),
			),
			parser.WithAutoHeadingID(),
		),
		goldmark.WithExtensions(&toc.Extender{}),
		goldmark.WithExtensions(extension.Linkify),
		goldmark.WithExtensions(extension.Strikethrough),
		goldmark.WithExtensions(extension.Table),
	)
}
