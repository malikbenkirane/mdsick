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
		{
			name:  "two-line paragraph joins at the soft break with one space",
			input: "hello\nworld\n",
			want:  "hello world\n",
		},
		{
			name:  "three lines join with one space each; last line keeps trailing space",
			input: "a\nb\nc  \n",
			want:  "a b c  \n",
		},
		{
			name:  "leading indent of the first line and per-line indents are trimmed",
			input: "  indent\n  cont\n",
			want:  "indent cont\n",
		},
		{
			name:  "wrapped inline construct joins naturally, Focus inside Emphasis",
			input: "some *em\nbody* text\n",
			want:  "some *em body* text\n",
		},
		{
			name:  "two paragraphs separated by one blank line stay separate",
			input: "first para\nwrapped\n\nsecond\n",
			want:  "first para wrapped\n\nsecond\n",
		},
		{
			name:  "wrapped lines inside loose and tight list items refill; markers and indents preserved byte-exactly",
			input: "- some list item\n  paragraph\n  - some sublist item\n    to reflow\n  - another item\n- one more item\n",
			want:  "- some list item paragraph\n  - some sublist item to reflow\n  - another item\n- one more item\n",
		},
		{
			name:  "ordered markers survive byte-exactly, no renumbering",
			input: "1. one\n   wrapped continuation\n10. ten stays ten\n",
			want:  "1. one wrapped continuation\n10. ten stays ten\n",
		},
		{
			name:  "list marker lookalike text inside an item is not treated as a marker",
			input: "- a**bold* start\n  continues here\n",
			want:  "- a**bold* start continues here\n",
		},
		{
			name:  "a wrapped GFM table row is immune, byte-exactly",
			input: "| a | b |\n| - | - |\n| one wrapped over\n| lines | |\n",
			want:  "| a | b |\n| - | - |\n| one wrapped over\n| lines | |\n",
		},
		{
			name:  "quoted paragraph refills underneath one re-emitted marker run",
			input: "> wrapping line one\n> wrapping line two\n",
			want:  "> wrapping line one wrapping line two\n",
		},
		{
			name:  "lazy continuation without its own marker joins the quote",
			input: "> quoted content\nlazy tail here\n",
			want:  "> quoted content lazy tail here\n",
		},
		{
			name:  "deeper-opened quote with lazy line refills into the inner paragraph",
			input: ">> deep quote\n> plain continuation\n",
			want:  ">> deep quote plain continuation\n",
		},
		{
			name:  "two quote paragraphs separated by a blank quote line stay separate",
			input: "> first para\n> wrapped\n>\n> second\n",
			want:  "> first para wrapped\n>\n> second\n",
		},
		{
			name:  "nested quote marker atoms strip and stack from the outermost",
			input: "> outer block quote\n>> inner nested\n",
			want:  "> outer block quote\n>> inner nested\n",
		},
		{
			name:  "reflowing an already-refilled quote is a no-op (idempotence)",
			input: ">> deep quote plain continuation\n",
			want:  ">> deep quote plain continuation\n",
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
