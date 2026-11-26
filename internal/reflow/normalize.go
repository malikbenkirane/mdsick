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

// blankCount counts blank lines in region; region starts at a newline and
// spans the gap between two consecutive top-level blocks.
func blankCount(region []byte) int {
	if len(region) == 0 {
		return 0
	}
	count := 0
	for line := range bytes.SplitSeq(region[1:], []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
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
				*edits = append(*edits, edit{start: prevEnd, stop: s, repl: []byte("\n\n")})
			}
		}
		prevEnd = e
	}
	// Trailing blank runs after the last block collapse to one newline,
	// while the last line's own trailing whitespace (sitting just before
	// the first newline of the tail, outside its segment) stays intact.
	if prevEnd < len(source) {
		region := source[prevEnd:]
		ws := 0
		for ws < len(region) && isSpaceTab(region[ws]) {
			ws++
		}
		if ws < len(region) && region[ws] == '\n' {
			tail := region[:ws+1]
			if len(region) > ws+1 {
				tail = append(tail[:ws:ws], '\n')
			}
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
