package markdown

import "testing"

func TestDocTitle(t *testing.T) {
	for _, test := range []struct {
		name string
		doc  string
		want string
	}{
		{name: "leading h1", doc: "# Hello World\n\nbody\n", want: "Hello World"},
		{name: "inline markup", doc: "# *Bold*, `code` and [link](http://x)\n", want: "Bold, code and link"},
		{name: "entity", doc: "# Q &amp; A\n", want: "Q & A"},
		{name: "escaped punctuation", doc: "# A \\* B\n", want: "A * B"},
		{name: "h2 first", doc: "## Not a title\n", want: ""},
		{name: "paragraph first", doc: "intro\n\n# Later\n", want: ""},
		{name: "empty", doc: "", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := DocTitle([]byte(test.doc)); got != test.want {
				t.Fatalf("unexpected title:\ngot %q\nexp %q", got, test.want)
			}
		})
	}
}
