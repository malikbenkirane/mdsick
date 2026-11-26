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

func TestListQuoteItemVsPlainTightItem(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "quoted block inside a list item nests under the ListItem",
			src:  "- > quoted in list\n- plain item\n",
			want: `` +
				"Document\n" +
				"  List\n" +
				"    ListItem\n" +
				"      Blockquote\n" +
				"        Paragraph\n" +
				"          Text \"quoted in list\"\n" +
				"    ListItem\n" +
				"      TextBlock\n" +
				"        Text \"plain item\"\n",
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

func TestNestedSublistInMixedList(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "item text and wrapped continuation land in one TextBlock",
			src:  "- some list item\n  paragraph\n  - some sublist item\n    to reflow\n  - another item\n- one more item\n",
			want: `` +
				"Document\n" +
				"  List\n" +
				"    ListItem\n" +
				"      TextBlock\n" +
				"        Text \"some list item\" (soft)\n" +
				"        Text \"paragraph\"\n" +
				"      List\n" +
				"        ListItem\n" +
				"          TextBlock\n" +
				"            Text \"some sublist item\" (soft)\n" +
				"            Text \"to reflow\"\n" +
				"        ListItem\n" +
				"          TextBlock\n" +
				"            Text \"another item\"\n" +
				"    ListItem\n" +
				"      TextBlock\n" +
				"        Text \"one more item\"\n",
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

func TestTightVsLooseListItems(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "blank line between items gives each a Paragraph",
			src:  "- mine\n\n- two\n",
			want: `` +
				"Document\n" +
				"  List\n" +
				"    ListItem\n" +
				"      Paragraph\n" +
				"        Text \"mine\"\n" +
				"    ListItem\n" +
				"      Paragraph\n" +
				"        Text \"two\"\n",
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

func TestSetextHeadingHoldsWrappedLines(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "setext underline consumes every preceding nonblank line",
			src:  "Heading\nsecond\n===\n",
			want: `` +
				"Document\n" +
				"  Heading level=1\n" +
				"    Text \"Heading\" (soft)\n" +
				"    Text \"second\"\n",
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

func TestTableHeaderAndBodyRows(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "table parses to TableHeader plus one TableRow",
			src:  "a | b\n--- | ---\n1 | 2\n",
			want: `` +
				"Document\n" +
				"  Table\n" +
				"    TableHeader\n" +
				"      TableCell\n" +
				"        Text \"a\"\n" +
				"      TableCell\n" +
				"        Text \"b\"\n" +
				"    TableRow\n" +
				"      TableCell\n" +
				"        Text \"1\"\n" +
				"      TableCell\n" +
				"        Text \"2\"\n",
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

func TestImmuneBlocksWrapAndUnwrap(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "one fenced code block built by three source lines",
			src:  "```go\nx\n```\n",
			want: `` +
				"Document\n" +
				"  FencedCodeBlock language=\"go\"\n",
		},
		{
			name: "one indented code block holding two lines",
			src:  "    indented code\n    more\n",
			want: `` +
				"Document\n" +
				"  CodeBlock\n",
		},
		{
			name: "one html block built by three source lines",
			src:  "<div>\nraw html\n</div>\n",
			want: `` +
				"Document\n" +
				"  HTMLBlock\n",
		},
		{
			name: "thematic break built by one line",
			src:  "---\n",
			want: `` +
				"Document\n" +
				"  ThematicBreak\n",
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
