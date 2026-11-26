package reflow

// reflow currently passes the source through. Later tasks add parsing,
// edit computation and normalization on top of this callsite.
func reflow(source []byte) []byte {
	return source
}
