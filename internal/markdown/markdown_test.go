package markdown

import (
	"strings"
	"testing"

	"github.com/yuin/goldmark/text"
)

func TestNewRendersWithTocAndAutoid(t *testing.T) {
	source := []byte("# Alpha\n\nsome [link](http://x)\n")
	md := New()
	doc := md.Parser().Parse(text.NewReader(source))
	var b strings.Builder
	if err := md.Renderer().Render(&b, source, doc); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `id="alpha"`) {
		t.Fatalf("auto heading id missing:\n%s", b.String())
	}
	if !strings.Contains(b.String(), `href="http://x"`) {
		t.Fatalf("linkify extension missing:\n%s", b.String())
	}
}
