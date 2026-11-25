package issues

import "fmt"

// Index is the flat set of resolvable issue numbers, regardless of
// which side an issue currently lives on.
type Index struct {
	numbers map[int]bool
}

func NewIndex(open, closed []Row) *Index {
	ix := &Index{numbers: make(map[int]bool)}
	for _, rows := range [][]Row{open, closed} {
		for _, row := range rows {
			ix.numbers[row.Number] = true
		}
	}
	return ix
}

func (ix *Index) Resolve(n int) (href string, ok bool) {
	if !ix.numbers[n] {
		return "", false
	}
	return fmt.Sprintf("%d.html", n), true
}

// Warning is one unresolvable reference, raised on the index page.
type Warning struct {
	Source string // the issue file the reference came from, e.g. "43.md"
	Ref    string // the raw unresolvable reference, e.g. "#44"
}

func (w Warning) String() string {
	return fmt.Sprintf("%s: %s — unresolvable", w.Source, w.Ref)
}
