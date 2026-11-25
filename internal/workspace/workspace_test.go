package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
)

func TestTempWorkspaceCopiesStyleAndChdirs(t *testing.T) {
	tmp, err := TempWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(wd)
	if err != nil {
		t.Fatal(err)
	}
	tmpReal, err := filepath.EvalSymlinks(filepath.Clean(tmp))
	if err != nil {
		t.Fatal(err)
	}
	if tmpReal != real {
		t.Fatalf("expected chdir into workspace:\ngot %s\nexp %s", real, tmpReal)
	}
	if _, err := os.Stat(filepath.Join(tmp, "github-markdown.css")); err != nil {
		t.Fatalf("style not copied: %v", err)
	}
}

func TestRenderFileWritesShellAndBody(t *testing.T) {
	tmp, err := TempWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	source := []byte("# hi\n\nbody\n")
	md := goldmark.New()
	doc := md.Parser().Parse(text.NewReader(source))
	if err := RenderFile("out.html", "Out & Title", md, source, doc); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(tmp, "out.html"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, "<title>Out & Title</title>") {
		t.Fatalf("missing escaped title:\n%s", s)
	}
	if !strings.Contains(s, "markdown-body") {
		t.Fatalf("missing body class:\n%s", s)
	}
	if strings.Count(s, "</body>") != 2 {
		t.Fatalf("expected the known double-close rendering quirk, got:\n%s", s)
	}
	if !strings.Contains(s, "body") {
		t.Fatalf("rendered markdown body missing:\n%s", s)
	}
}
