package main

import (
	"slices"

	"github.com/malikbenkirane/mdsick/internal/issues"
	"github.com/malikbenkirane/mdsick/internal/natural"
)

// foldWarnings deduplicates exact (source, ref) pairs — each *mention*
// of one broken ref produces its own warning — and orders rows
// deterministically: by file (natural), then by ref.
func foldWarnings(warns []issues.Warning) []issues.Warning {
	seen := make(map[issues.Warning]bool)
	var rows []issues.Warning
	for _, w := range warns {
		if seen[w] {
			continue
		}
		seen[w] = true
		rows = append(rows, w)
	}
	slices.SortStableFunc(rows, func(a, b issues.Warning) int {
		if c := natural.Compare(a.Source, b.Source); c != 0 {
			return c
		}
		return natural.Compare(a.Ref, b.Ref)
	})
	return rows
}
