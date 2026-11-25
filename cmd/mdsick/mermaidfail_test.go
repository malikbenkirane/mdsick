package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMixedMermaidRendersEndToEnd(t *testing.T) {
	// Hermetic fake mmdc: an invalid diagram gets diagnostics + exit 1;
	// anything else gets a one-line SVG.
	shimDir := t.TempDir()
	script := []byte(`#!/bin/sh
tmp=$(mktemp)
cat > "$tmp"
if grep -q FAIL "$tmp"; then
	printf 'mermaid: parse error near line 2\n' >&2
	exit 1
fi
printf '<svg xmlns="x"/>\n'
`)
	if err := os.WriteFile(filepath.Join(shimDir, "mmdc"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	doc := "before\n\n```mermaid\nFAIL: bad\n```\n\n```mermaid\ngraph TD; A-->B\n```\n\nafter\n"
	docPath := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(docPath, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(docPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	bin := filepath.Join(t.TempDir(), "mdsick")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	run := exec.CommandContext(ctx, bin)
	run.Stdin = f

	var errBuf strings.Builder
	run.Stderr = &errBuf

	if err := run.Run(); err != nil {
		t.Fatalf("subprocess did not render to completion: %v\n%s", err, errBuf.String())
	}

	// The child chdirs into a fresh temp workspace whose path it prints to
	// stderr (`tempWorkspace`); find that line and read the render from it.
	var tmpPath string
	for _, line := range strings.Split(strings.TrimSpace(errBuf.String()), "\n") {
		if filepath.IsAbs(line) && isDir(line) {
			tmpPath = line
			break
		}
	}
	if tmpPath == "" {
		t.Fatalf("no temp workspace printed:\n%s", errBuf.String())
	}

	html, err := os.ReadFile(filepath.Join(tmpPath, "rendered.html"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(html)
	for _, exp := range []string{
		`class="language-mermaid">1| FAIL: bad`,
		`class="language-text"`,
		"parse error near line 2",
		".svg",
	} {
		if !strings.Contains(got, exp) {
			t.Fatalf("missing %q in:\n%s", exp, got)
		}
	}
}

func isDir(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.IsDir()
}
