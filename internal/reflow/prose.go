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
func refillBlock(source []byte, lines *text.Segments) (start int, repl []byte) {
	first := lines.At(0)
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
	for i := 0; i < lines.Len(); i++ {
		body := lineBody(source, lines.At(i))
		body = body[contentStart(body):]
		if i > 0 {
			out.WriteByte(' ')
		}
		if i < lines.Len()-1 {
			body = body[:len(body)-trailingWS(body)]
		}
		out.Write(body)
	}
	return start, out.Bytes()
}
