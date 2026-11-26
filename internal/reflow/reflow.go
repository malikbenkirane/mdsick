// Package reflow unwraps hard-wrapped markdown prose so `cat doc.md | \
// mdsick-reflow | pbcopy` pastes into GitHub without wrap artifacts.
package reflow

import (
	"io"
)

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

// Reflow reads source from r and returns the refilled bytes.
// It is the single failure point: read errors flow through verbatim;
// parsing itself cannot fail.
func Reflow(r io.Reader) ([]byte, error) {
	source, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return reflow(source), nil
}

// collectProse records one whole-block replacement edit for every
// multi-line refillable leaf block anywhere in the tree. A block is
// refillable when it is a Paragraph or — under a ListItem, where tight
// item text lives — a TextBlock, and no Table anywhere in its ancestry
// (table cells must stay untouched).
func collectProse(n ast.Node, source []byte, edits *[]edit) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindParagraph, ast.KindTextBlock:
			if !hasTableAncestor(c) {
				addRefillEdit(c, source, edits)
			}
		default:
			if c.Type() == ast.TypeBlock {
				collectProse(c, source, edits)
			}
		}
	}
}

func hasTableAncestor(n ast.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == extast.KindTable {
			return true
		}
	}
	return false
}

func addRefillEdit(b ast.Node, source []byte, edits *[]edit) {
	s, e := blockRange(b)
	if s < 0 || e <= s {
		return
	}
	start, repl := refillBlock(source, b.Lines())
	if !bytes.Equal(repl, source[start:e]) {
		*edits = append(*edits, edit{start: start, stop: e, repl: repl})
	}
}
