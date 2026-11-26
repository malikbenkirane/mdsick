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
