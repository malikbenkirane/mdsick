package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/malikbenkirane/mdsick/internal/reflow"
)

// runStd is the testable core. It refuses piped-less stdin and keeps
// stdout reserved for refilled bytes; every diagnostic goes through the
// returned error, which main routes to stderr.
func runStd(input io.Reader, stdout, stderr io.Writer, isTTY bool) error {
	if isTTY {
		return errors.New("mdsick-reflow reads from stdin; pipe a markdown file in, e.g. `cat doc.md | mdsick-reflow`")
	}
	_ = stderr // reserved: runStd itself never writes diagnostics
	out, err := reflow.Reflow(input)
	if err != nil {
		return err
	}
	_, err = stdout.Write(out)
	return err
}

func stdinIsTerminal() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

func main() {
	if err := runStd(os.Stdin, os.Stdout, os.Stderr, stdinIsTerminal()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
