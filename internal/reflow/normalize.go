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
	// Trailing blank runs after the last block collapse to one newline.
	if prevEnd < len(source) {
		*edits = append(*edits, edit{start: prevEnd, stop: len(source), repl: []byte("\n")})
	}
}

// blockRange returns the start and end byte offsets of a block's source lines.
// Line segments exclude the terminating newline; trailing spaces on the last
// line belong to the block.
func blockRange(b ast.Node) (int, int) {
	seg := b.Lines()
	if seg.Len() == 0 {
		return -1, -1
	}
	return seg.At(0).Start, seg.At(seg.Len() - 1).Stop
}
