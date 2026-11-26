package reflow

// isDigit reports whether b is an ASCII digit byte.
func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// isListMarker reports whether line[i:] starts a list marker token:
// one of '-', '+' , '*' , or a digit run followed by '.' or ')'.
// In both cases a whitespace byte must follow the marker and a
// non-empty remainder must exist after that whitespace, so an inline
// '*' at line start is never mistaken for a marker.
func isListMarker(line []byte, i int) bool {
	switch line[i] {
	case '-', '+', '*':
		return i+1 < len(line) && isSpaceTab(line[i+1]) &&
			hasContentAfter(line, i+1)
	}
	if isDigit(line[i]) {
		j := i
		for j < len(line) && isDigit(line[j]) {
			j++
		}
		if j < len(line) && (line[j] == '.' || line[j] == ')') {
			return j+1 < len(line) && isSpaceTab(line[j+1]) &&
				hasContentAfter(line, j+1)
		}
	}
	return false
}

// listMarkerEnd returns the offset one past the marker token: its
// characters only; the following whitespace is consumed by prefixEnd's
// generic whitespace case.
func listMarkerEnd(line []byte, i int) int {
	if isDigit(line[i]) {
		j := i
		for j < len(line) && isDigit(line[j]) {
			j++
		}
		return j + 1 // digits plus one of '.', ')'
	}
	return i + 1 // '-', '+' or '*'
}

// hasContentAfter reports whether a non-whitespace byte exists after
// line[from:]; guards the required non-empty remainder.
func hasContentAfter(line []byte, from int) bool {
	for from < len(line) {
		if !isSpaceTab(line[from]) {
			return true
		}
		from++
	}
	return false
}

// prefixEnd returns the offset at which a source line's marker run ends
// and the line's content begins, after scanning any sequence of
// whitespace, '>' quote markers and at most one list marker. Returns -1
// when the line has no content after the run (caller treats it as blank).
func prefixEnd(line []byte) int {
	i := 0
	markerSeen := false
	for i < len(line) {
		switch {
		case isSpaceTab(line[i]):
			i++
		case line[i] == '>':
			i++
		case !markerSeen && isListMarker(line, i):
			i = listMarkerEnd(line, i)
			markerSeen = true
		default:
			return i
		}
	}
	return -1
}
