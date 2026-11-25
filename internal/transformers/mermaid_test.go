package transformers

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

func chdirTemp(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	})
}

func writeShim(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := []byte(`#!/bin/sh
tmp=$(mktemp)
cat > "$tmp"
if grep -q FAIL "$tmp"; then
	printf 'mermaid: parse error near line 2\n' >&2
	exit 1
fi
printf '<svg xmlns="x"/>\n'
`)
	if err := os.WriteFile(dir+"/mmdc", script, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func convert(t *testing.T, src string) string {
	t.Helper()
	writeShim(t)
	chdirTemp(t)

	md := goldmark.New(goldmark.WithRendererOptions(
		renderer.WithNodeRenderers(util.PrioritizedValue{
			Value: CodeLiteralRenderer{},
		}),
	), goldmark.WithParserOptions(
		parser.WithASTTransformers(util.PrioritizedValue{
			Value: &Mermaid{},
		}),
	))
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestMixedDiagamsRender(t *testing.T) {
	html := convert(t, "before\n\n```mermaid\nFAIL: bad\n```\n\n```mermaid\ngraph TD; A-->B\n```\n\nafter\n")
	if !strings.Contains(html, "FAIL: bad") {
		t.Fatalf("failed fence dropped:\n%s", html)
	}
	if !strings.Contains(html, ".svg") {
		t.Fatalf("valid diagram missing SVG reference:\n%s", html)
	}
}

func TestFatalHookInjectable(t *testing.T) {
	var got []error
	saved := fatal
	fatal = func(errs []error) {
		got = append(got, errs...)
	}
	defer func() { fatal = saved }()

	in := []error{errors.New("rand: read"), errors.New("os: create")}
	fatal(in)

	if len(got) != len(in) {
		t.Fatalf("hook lost errors:\ngot %d\nexp %d", len(got), len(in))
	}
}

func TestFailureArtifactShape(t *testing.T) {
	html := convert(t, "```mermaid\nFAIL: bad\n```\n")

	margin := `<pre><code class="language-mermaid">1| FAIL: bad
</code></pre>
<pre><code class="language-text">mermaid: parse error near line 2
</code></pre>`
	if !strings.Contains(html, margin) {
		t.Fatalf("artifact missing or misplaced:\n%s", html)
	}
}

func TestArtifactAdjacencyAndPosition(t *testing.T) {
	html := convert(t, "before\n\n```mermaid\nFAIL: bad\n```\n\n```mermaid\ngraph TD; A-->B\n```\n\nafter\n")

	iArtifact := strings.Index(html, `class="language-text"`)
	iImage := strings.Index(html, ".svg")
	if iArtifact < 0 || iImage < 0 {
		t.Fatalf("missing artifact or image:\n%s", html)
	}
	if iArtifact > iImage {
		t.Fatalf("artifact not before the healthy diagram:\n%s", html)
	}
}
