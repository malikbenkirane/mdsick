package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRunStdRefusesTTY(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runStd(strings.NewReader("# doc\n"), &stdout, &stderr, true)
	if err == nil {
		t.Fatal("want an error for terminal stdin")
	}
	if !strings.Contains(err.Error(), "cat doc.md | mdsick-reflow") {
		t.Errorf("err = %q, want usage hint", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout must stay clean for pbcopy, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("runStd must not print to stderr; main routes errors itself")
	}
}

func TestRunStdPipesThroughReflow(t *testing.T) {
	var stdout bytes.Buffer
	err := runStd(strings.NewReader("hello\nworld\n"), &stdout, &bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := stdout.String(); got != "hello world\n" {
		t.Errorf("out = %q, want %q", got, "hello world\n")
	}
}

// failingReader exercises Reflow's error path through runStd.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read boom") }

func TestRunStdPropagatesReadErrors(t *testing.T) {
	var stdout bytes.Buffer
	err := runStd(failingReader{}, &stdout, &bytes.Buffer{}, false)
	if err == nil || err.Error() != "read boom" {
		t.Fatalf("err = %v, want read boom", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout must stay clean on error, got %q", stdout.String())
	}
}
