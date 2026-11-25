package main

import (
	"testing"

	"github.com/malikbenkirane/mdsick/internal/issues"
)

func TestFoldWarnings(t *testing.T) {
	for _, test := range []struct {
		name string
		in   []issues.Warning
		want []issues.Warning
	}{
		{
			name: "duplicates fold, natural file order, ref order",
			in: []issues.Warning{
				{Source: "9.md", Ref: "36"},
				{Source: "43.md", Ref: "#9"},
				{Source: "9.md", Ref: "#8"},
				{Source: "9.md", Ref: "36"},
				{Source: "13.md", Ref: "#21"},
				{Source: "13.md", Ref: "#2"},
			},
			want: []issues.Warning{
				{Source: "9.md", Ref: "#8"},
				{Source: "9.md", Ref: "36"},
				{Source: "13.md", Ref: "#2"},
				{Source: "13.md", Ref: "#21"},
				{Source: "43.md", Ref: "#9"},
			},
		},
		{
			name: "empty stays empty",
			in:   nil,
			want: nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := foldWarnings(test.in)
			if len(got) != len(test.want) {
				t.Fatalf("foldWarnings:\ngot %v\nexp %v", got, test.want)
			}
			for i := range test.want {
				if got[i] != test.want[i] {
					t.Fatalf("foldWarnings[%d]:\ngot %v\nexp %v", i, got[i], test.want[i])
				}
			}
		})
	}
}
