package main

import (
	"reflect"
	"testing"

	"github.com/malikbenkirane/mdsick/internal/issues"
)

func TestFoldWarnings(t *testing.T) {
	for _, test := range []struct {
		name string
		in   []issues.Warning
		want []fileWarnings
	}{
		{
			name: "one row per file, natural file order, sorted refs",
			in: []issues.Warning{
				{Source: "9.md", Ref: "#36"},
				{Source: "43.md", Ref: "#9"},
				{Source: "9.md", Ref: "#8"},
				{Source: "13.md", Ref: "#21"},
				{Source: "13.md", Ref: "#2"},
			},
			want: []fileWarnings{
				{Source: "9.md", Refs: []string{"#8", "#36"}},
				{Source: "13.md", Refs: []string{"#2", "#21"}},
				{Source: "43.md", Refs: []string{"#9"}},
			},
		},
		{
			name: "same broken ref in different spellings folds to #N",
			in: []issues.Warning{
				{Source: "9.md", Ref: "#8"},
				{Source: "9.md", Ref: "8.md"},
				{Source: "9.md", Ref: "docs/issues/closed/8.md"},
				{Source: "9.md", Ref: "issue 8"},
				{Source: "9.md", Ref: "8"},
			},
			want: []fileWarnings{
				{Source: "9.md", Refs: []string{"#8"}},
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
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("foldWarnings:\ngot %v\nexp %v", got, test.want)
			}
		})
	}
}
