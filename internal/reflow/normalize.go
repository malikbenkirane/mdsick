package reflow

import (
	"bytes"
	"github.com/yuin/goldmark/ast"
)

func reflow(source []byte) []byte {
	if len(source) == 0 {
		return source
	}
	doc := parse(source)
	var edits []edit
	collectProse(doc, source, &edits)
	normalizeTop(doc, source, &edits)
	out := apply(source, edits)
	if len(out) == 0 {
		// Input carried only blank lines: POSIX convention yields a single
		// trailing newline; empty input exited above.
		return []byte{'\n'}
	}
	if out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return out
}

// gapPieces splits the region between two blocks into its lines.
func gapPieces(region []byte) [][]byte { return bytes.Split(region, []byte("\n")) }

// gapIsBlankLines reports whether every line of the region is pure
// whitespace. Parser residue that lives outside block line segments
// (fence closers, setext underlines, thematic dashes) shows up as
// non-whitespace pieces; such regions are never touched.
func gapIsBlankLines(region []byte) bool {
	if len(region) == 0 {
		return false
	}
	for _, p := range gapPieces(region) {
		if len(bytes.TrimSpace(p)) > 0 {
			return false
		}
	}
	return true
}

// blankCount counts blank lines strictly between the two blocks: it
// excludes the first piece (trailing whitespace of the previous block's
// last line) and the last piece (leading indent of the next block's
// first line).
func blankCount(region []byte) int {
	if !gapIsBlankLines(region) {
		return 0
	}
	pieces := gapPieces(region)
	count := 0
	for j := 1; j < len(pieces)-1; j++ {
		if len(bytes.TrimSpace(pieces[j])) == 0 {
			count++
		}
	}
	return count
}

// normalizeTop collapses extra blank lines between top-level blocks.
func normalizeTop(doc ast.Node, source []byte, edits *[]edit) {
	first := doc.FirstChild()
	if first == nil {
		// Source contains no top-level blocks (blank-only text; empty input
		// already returned above): the whole source collapses to "" and
		// reflow's empty-output guard turns it into a single trailing
		// newline.
		*edits = append(*edits, edit{start: 0, stop: len(source), repl: []byte{}})
		return
	}
	prevEnd := -1
	for b := first; b != nil; b = b.NextSibling() {
		s, e := blockRange(b)
		if s < 0 {
			continue // zero-line block edge; leave the source span untouched
		}
		if prevEnd >= 0 {
			region := source[prevEnd:s]
			if blankCount(region) >= 2 {
				// Whitespace regions are blank runs; the last piece is the
				// next block's leading indent and must survive the collapse.
				pieces := gapPieces(region)
				repl := append([]byte("\n\n"), pieces[len(pieces)-1]...)
				*edits = append(*edits, edit{start: prevEnd, stop: s, repl: repl})
			}
		}
		prevEnd = e
	}
	// Trailing blank runs after the last block collapse to one newline,
	// while the last line's own trailing whitespace (sitting just before
	// the first newline of the tail, outside its segment) stays intact.
	if prevEnd < len(source) {
		region := source[prevEnd:]
		if gapIsBlankLines(region) {
			pieces := gapPieces(region)
			// The first piece is the last line's own trailing whitespace
			// (outside its segment); keep it and one newline.
			tail := append([]byte{}, pieces[0]...)
			tail = append(tail, '\n')
			if !bytes.Equal(tail, region) {
				*edits = append(*edits, edit{start: prevEnd, stop: len(source), repl: tail})
			}
		}
	}
}

// blockRange returns the start and end byte offsets covering a block's
// source lines. A container node's span comes from its descendants,
// since List and Blockquote carry no line segments of their own.
func blockRange(b ast.Node) (int, int) {
	min, max := -1, -1
	var visit func(n ast.Node)
	visit = func(n ast.Node) {
		// Inline nodes panic on Lines().
		if n.Type() != ast.TypeInline {
			if segs := n.Lines(); segs.Len() != 0 {
				if first := segs.At(0).Start; min < 0 || first < min {
					min = first
				}
				if last := segs.At(segs.Len() - 1).Stop; last > max {
					max = last
				}
			}
		}
		// Children of leaf text blocks are inline nodes; inline nodes
		// panic on Lines().
		switch n.Kind() {
		case ast.KindParagraph, ast.KindTextBlock:
			return
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if c.Type() != ast.TypeInline {
				visit(c)
			}
		}
	}
	visit(b)
	if min < 0 {
		return -1, -1
	}
	return min, max
}
