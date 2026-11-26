package reflow_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/malikbenkirane/mdsick/internal/reflow"
)

// errReader fails on the first Read, exercising Reflow's error path.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read boom") }

func TestReflowReadsAndForwardsSource(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:    "reader error is returned verbatim",
			input:   "anything",
			wantErr: "read boom",
		},
		{
			name:  "empty input passes through unchanged",
			input: "",
			want:  "",
		},
		{
			name:  "single-line paragraph passes through",
			input: "some paragraph\n",
			want:  "some paragraph\n",
		},
		{
			name:  "two blank lines between top-level blocks collapse to one",
			input: "a\n\n\n\nb\n",
			want:  "a\n\nb\n",
		},
		{
			name:  "exactly one blank line between blocks stays byte-identical",
			input: "a\n\nb\n",
			want:  "a\n\nb\n",
		},
		{
			name:  "adjacent top-level lines without a blank line stay untouched",
			input: "```go\nx\n```\nnext para\n",
			want:  "```go\nx\n```\nnext para\n",
		},
		{
			name:  "single blank line after last block collapses the output trailing newline",
			input: "a\n\n",
			want:  "a\n",
		},
		{
			name:  "blank-only input yields a POSIX newline",
			input: "  \n\n",
			want:  "\n",
		},
		{
			name:  "missing final newline is guaranteed",
			input: "a",
			want:  "a\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r io.Reader
			if tt.wantErr != "" {
				r = errReader{}
			} else {
				r = strings.NewReader(tt.input)
			}
			got, err := reflow.Reflow(r)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				if got != nil {
					t.Fatalf("out = %q, want nil", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("out = %q, want %q", got, tt.want)
			}
		})
	}
}
