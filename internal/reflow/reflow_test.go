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
