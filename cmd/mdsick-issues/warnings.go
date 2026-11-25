package main

import (
	"slices"
	"strconv"

	"github.com/malikbenkirane/mdsick/internal/issues"
	"github.com/malikbenkirane/mdsick/internal/natural"
)

// fileWarnings is one warnings-table row: all distinct broken
// references of a single source file, sorted.
type fileWarnings struct {
	Source string
	Refs   []string
}

// foldWarningKey identifies a broken reference by its underlying issue
// number: the same broken ref reaches the collector under different
// spellings (#8, 8.md, closed/8.md, issue 8) depending on which rewrite
// tier matched, so refs fold on the number, not the spelling.
// Non-numeric refs fall back to the raw string as the key.
type foldWarningKey struct {
	source string
	key    string
}

// refNumber extracts the first digit run in a raw reference; ok=false
// means the ref carries no number at all (defensive; all warn call
// sites emit refs matched by number-based regexes).
func refNumber(ref string) (n int, ok bool) {
	start := -1
	for i := 0; i <= len(ref); i++ {
		digit := i < len(ref) && ref[i] >= '0' && ref[i] <= '9'
		if digit && start < 0 {
			start = i
		} else if !digit && start >= 0 {
			n, err := strconv.Atoi(ref[start:i])
			return n, err == nil
		}
	}
	return 0, false
}

// canonicalRef folds a raw warning ref to its canonical display form:
// `#N` for numeric refs (which folds the different tier spellings of
// one broken ref together), the raw text otherwise.
func canonicalRef(ref string) string {
	if n, ok := refNumber(ref); ok {
		return "#" + strconv.Itoa(n)
	}
	return ref
}

// foldWarnings groups warnings by source file — one row per file,
// deduplicated on the underlying broken reference (each *mention* and
// each *spelling* of one broken ref produces its own warning) — and
// orders rows by file (natural), refs within a file sorted.
func foldWarnings(warns []issues.Warning) []fileWarnings {
	seen := make(map[foldWarningKey]bool)
	var rows []fileWarnings
	idx := make(map[string]int)
	for _, w := range warns {
		ref := canonicalRef(w.Ref)
		key := foldWarningKey{source: w.Source, key: ref}
		if seen[key] {
			continue
		}
		seen[key] = true
		i, ok := idx[w.Source]
		if !ok {
			i = len(rows)
			idx[w.Source] = i
			rows = append(rows, fileWarnings{Source: w.Source})
		}
		rows[i].Refs = append(rows[i].Refs, ref)
	}
	for i := range rows {
		slices.SortFunc(rows[i].Refs, natural.Compare)
	}
	slices.SortStableFunc(rows, func(a, b fileWarnings) int {
		return natural.Compare(a.Source, b.Source)
	})
	return rows
}
