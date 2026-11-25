package mermaid

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeShim(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := []byte(`#!/bin/sh
tmp=$(mktemp)
cat > "$tmp"
if grep -q FAIL "$tmp"; then
	printf 'mermaid: parse error near line 2\n' >&2
	printf 'partial stdout\n'
	exit 1
fi
printf '<svg xmlns="x"/>\n'
`)
	if err := os.WriteFile(filepath.Join(dir, "mmdc"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCompileSuccess(t *testing.T) {
	writeShim(t)
	var dst bytes.Buffer
	if err := Compile(strings.NewReader("graph TD; A-->B"), &dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, exp := dst.String(), "<svg xmlns=\"x\"/>\n"; got != exp {
		t.Fatalf("dst:\ngot %q\nexp %q", got, exp)
	}
}

func TestCompileFailureCarriesOutput(t *testing.T) {
	writeShim(t)
	var dst bytes.Buffer
	err := Compile(strings.NewReader("FAIL: broken diagram"), &dst)
	if err == nil {
		t.Fatal("expected error")
	}
	var fail *Failure
	if !errors.As(err, &fail) {
		t.Fatalf("expected *Failure, got %T: %v", err, err)
	}
	if !strings.Contains(string(fail.Output), "parse error") {
		t.Fatalf("Output missing stderr: %q", fail.Output)
	}
	if !strings.Contains(string(fail.Output), "partial stdout") {
		t.Fatalf("Output missing stdout: %q", fail.Output)
	}
	if dst.Len() != 0 {
		t.Fatalf("dst must stay empty on failure, got %q", dst.String())
	}
}
