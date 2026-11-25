package issues_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/malikbenkirane/mdsick/internal/issues"
	"github.com/malikbenkirane/mdsick/internal/markdown"
	"github.com/yuin/goldmark/text"
)

func renderIssues(t *testing.T, ix *issues.Index, src string) (string, []issues.Warning) {
	t.Helper()
	var warns []issues.Warning
	md := markdown.NewIssues(ix, "9.md", &warns)
	doc := md.Parser().Parse(text.NewReader([]byte(src)))
	var b bytes.Buffer
	if err := md.Renderer().Render(&b, []byte(src), doc); err != nil {
		t.Fatal(err)
	}
	return b.String(), warns
}

func TestRewrites(t *testing.T) {
	ix := issues.NewIndex([]issues.Row{{Number: 21}, {Number: 8}, {Number: 43}}, nil)

	t.Run("hash ref in prose", func(t *testing.T) {
		out, _ := renderIssues(t, ix, "fixes #21 now\n")
		if !strings.Contains(out, `<a href="21.html">#21</a>`) {
			t.Fatalf("hash ref not linked:\n%s", out)
		}
	})

	t.Run("code fence skipped, code span rewritten", func(t *testing.T) {
		out, _ := renderIssues(t, ix, "```\nsee #21 raw\n```\n\nref `43.md` inline\n")
		if strings.Contains(out, `<a href="21.html">`) {
			t.Fatalf("fenced code was rewritten:\n%s", out)
		}
		if strings.Contains(out, "<code>21") {
			t.Fatalf("hash inside code span unexpectedly rewritten:\n%s", out)
		}
		if !strings.Contains(out, fmt.Sprintf("<code>%s", "43")) && !strings.Contains(out, `href="43.html"`) {
			t.Fatalf("code span path not linked:\n%s", out)
		}
	})

	t.Run("link destination with directory part", func(t *testing.T) {
		out, _ := renderIssues(t, ix, "[8](closed/8.md) and [epic](21.md)\n")
		if !strings.Contains(out, `<a href="8.html">8</a>`) {
			t.Fatalf("dir dest not rewritten:\n%s", out)
		}
		if !strings.Contains(out, `<a href="21.html">epic</a>`) {
			t.Fatalf("epic label link not rewritten:\n%s", out)
		}
	})

	t.Run("unresolvable flows to warnings", func(t *testing.T) {
		out, warns := renderIssues(t, ix, "see #44 and [9](9.md)\n")
		if !strings.Contains(out, "9.md") {
			t.Fatalf("unresolvable references must stay intact:\n%s", out)
		}
		if len(warns) != 2 {
			t.Fatalf("expected 2 warnings, got %v", warns)
		}
		for _, w := range warns {
			if w.Source != "9.md" || (w.Ref != "#44" && w.Ref != "9.md") {
				t.Fatalf("bad warning: %+v", w)
			}
		}
	})

	t.Run("prose issue phrase", func(t *testing.T) {
		out, _ := renderIssues(t, ix, "we address issue 8 here (not issue 44)\n")
		if !strings.Contains(out, `href="8.html"`) {
			t.Fatalf("issue phrase not linkified:\n%s", out)
		}
	})

	t.Run("full path in prose", func(t *testing.T) {
		out, _ := renderIssues(t, ix, "see docs/issues/closed/21.md fully\n")
		if !strings.Contains(out, `href="21.html"`) {
			t.Fatalf("path not linkified:\n%s", out)
		}
	})
}
