package ast_test

import (
	"testing"

	reflowast "github.com/malikbenkirane/mdsick/internal/reflow/ast"
)

func dump(t *testing.T, src string) string {
	t.Helper()
	return reflowast.Dump([]byte(src))
}

func TestParagraphContinuousLinesJoin(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "paragraph spanning two lines stays one prose block",
			src:  "hello\nworld\n",
			want: `` +
				"Document\n" +
				"  Paragraph\n" +
				"    Text \"hello\" (soft)\n" +
				"    Text \"world\"\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dump(t, tt.src); got != tt.want {
				t.Errorf("got:\n%v\nwant:\n%v", got, tt.want)
			}
		})
	}
}

func TestParagraphHardBreakFlag(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "two trailing spaces mark a hard break, without newline in emphasis output",
			src:  "hello  \nworld\n",
			want: `` +
				"Document\n" +
				"  Paragraph\n" +
				"    Text \"hello\" (hard)\n" +
				"    Text \"world\"\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dump(t, tt.src); got != tt.want {
				t.Errorf("got:\n%v\nwant:\n%v", got, tt.want)
			}
		})
	}
}

func TestEmphasisWrapSpansBreak(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "emphasis node holds both halves of a wrapped run",
			src:  "*em\nbody*\n",
			want: `` +
				"Document\n" +
				"  Paragraph\n" +
				"    Emphasis\n" +
				"      Text \"em\" (soft)\n" +
				"      Text \"body\"\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dump(t, tt.src); got != tt.want {
				t.Errorf("got:\n%v\nwant:\n%v", got, tt.want)
			}
		})
	}
}

func TestBlockquoteParagraphsNestAndWrap(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "lazy continuation joins the quoted paragraph",
			src:  "> quoted\nlazy\n",
			want: `` +
				"Document\n" +
				"  Blockquote\n" +
				"    Paragraph\n" +
				"      Text \"quoted\" (soft)\n" +
				"      Text \"lazy\"\n",
		},
		{
			name: "a deeper-opened quote line is filled without its own marker too",
			src:  ">> deep\n> plain\n",
			want: `` +
				"Document\n" +
				"  Blockquote\n" +
				"    Blockquote\n" +
				"      Paragraph\n" +
				"        Text \"deep\" (soft)\n" +
				"        Text \"plain\"\n",
		},
		{
			name: "hard break inside a quote is flagged on the Text",
			src:  "> line one\n> line two  \n> after\n",
			want: `` +
				"Document\n" +
				"  Blockquote\n" +
				"    Paragraph\n" +
				"      Text \"line one\" (soft)\n" +
				"      Text \"line two\" (hard)\n" +
				"      Text \"after\"\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dump(t, tt.src); got != tt.want {
				t.Errorf("got:\n%v\nwant:\n%v", got, tt.want)
			}
		})
	}
}
