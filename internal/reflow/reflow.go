// Package reflow unwraps hard-wrapped markdown prose so `cat doc.md | \
// mdsick-reflow | pbcopy` pastes into GitHub without wrap artifacts.
package reflow

import (
	"io"
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
