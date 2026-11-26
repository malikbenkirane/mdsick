package reflow

import (
	"bytes"

	"github.com/yuin/goldmark/text"
)

// isSpaceTab reports whether b is a space or tab byte.
func isSpaceTab(b byte) bool { return b == ' ' || b == '\t' }

// contentStart returns the index of a line body's first content byte:
// past its marker run, then past remaining whitespace. Returns
// len(line) when the line carries no content.
func contentStart(line []byte) int {
	if pe := prefixEnd(line); pe >= 0 {
		return pe
	}
	return len(line)
}

// trailingWS counts a line body's trailing whitespace run.
func trailingWS(body []byte) int {
	n := 0
	for n < len(body) && isSpaceTab(body[len(body)-1-n]) {
		n++
	}
	return n
}

// lineBody returns a block line's bytes without the terminating newline.
// Block line segments include the newline whenever they are followed by
// another line inside the block; the final line's segment ends at its
// content. Seam logic consumes one consistent view, so it strips the
// newline up front.
func lineBody(source []byte, seg text.Segment) []byte {
	body := source[seg.Start:seg.Stop]
	if len(body) > 0 && body[len(body)-1] == '\n' {
		body = body[:len(body)-1]
	}
	return body
}

// refillBlock joins soft-broken lines of a prose block with one space per
// seam. The line bodies are trimmed left (except line 0, whose leading
// whitespace is excluded by the returned start offset) and right (except
// the last line, which keeps its trailing whitespace byte-exactly per the
// seam rule). Returns the byte offset where the refillable content starts
// and the replacement bytes spliced over [start, lastLineStop).
// refillBlock joins the soft-broken lines of a prose block with one space
// per seam. Seam classification is decided from the bytes of the line
// before the newline:
//
//   - a backslash as the last content byte: the seam stays raw (it is
//     already the canonical hard-break form, or the backslash is literal
//     content inside a two-space run);
//   - a trailing whitespace run of two or more: the run collapses to one
//     backslash terminating the line ("\\\n"), and the following line
//     keeps its own prefix and content byte-exactly;
//   - otherwise: a pure soft break — trailing whitespace and the next
//     line's marker run and indentation are trimmed, one space joins.
//
// Returns the offset where the refillable content starts and the
// replacement bytes to splice over [start, lastLineStop).
func refillBlock(source []byte, lines *text.Segments) (start int, repl []byte) {
	first := lines.At(0)
	last := lines.At(lines.Len() - 1)
	start = first.Start
	// Top-level paragraph line segments exclude leading whitespace, so
	// the first line's indent sits just before the segment; consume it
	// only when the run backs onto a line start, never inside a marker.
	t := start
	for t > 0 && isSpaceTab(source[t-1]) {
		t--
	}
	if t == 0 || source[t-1] == '\n' {
		start = t
	}
	var out bytes.Buffer
	pos := first.Start + contentStart(lineBody(source, first))
	for i := 0; i < lines.Len()-1; i++ {
		prev, next := lines.At(i), lines.At(i+1)
		prevBody := lineBody(source, prev)
		if len(prevBody) > 0 && prevBody[len(prevBody)-1] == '\n' {
			prevBody = prevBody[:len(prevBody)-1]
		}
		pw := trailingWS(prevBody)
		wsStart := prev.Start + len(prevBody) - pw
		keep := prevBody[:len(prevBody)-pw]
		csNext := next.Start + contentStart(lineBody(source, next))
		switch {
		case len(keep) > 0 && keep[len(keep)-1] == '\\':
			// Raw seam: the line already carries its termination bytes.
			out.Write(source[pos:prev.Stop])
			pos = csNext
		case pw >= 2:
			// Hard break: the trailing run collapses to one space plus
			// one backslash; the continuation keeps its own prefix and
			// indentation raw.
			out.Write(source[pos:wsStart])
			out.WriteByte(' ')
			out.WriteByte('\\')
			out.WriteByte('\n')
			pos = prev.Stop
		default:
			out.Write(source[pos:wsStart])
			out.WriteByte(' ')
			pos = csNext
		}
	}
	out.Write(source[pos:last.Stop])
	return start, out.Bytes()
}
