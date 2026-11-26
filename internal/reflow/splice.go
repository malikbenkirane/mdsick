package reflow

import (
	"bytes"
	"sort"
)

type edit struct {
	start, stop int
	repl        []byte
}

func apply(source []byte, edits []edit) []byte {
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var out bytes.Buffer
	pos := 0
	for _, e := range edits {
		if e.start > e.stop || e.start < pos {
			panic("reflow: overlapping or unsorted edit")
		}
		out.Write(source[pos:e.start])
		out.Write(e.repl)
		pos = e.stop
	}
	out.Write(source[pos:])
	return out.Bytes()
}
