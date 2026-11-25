package issues

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/malikbenkirane/mdsick/internal/natural"
)

// Row is one issue file on one side of the tree.
type Row struct {
	Number int    // parsed NN
	Base   string // "NN.md"
	Side   string // "open" or "closed"
	Head   string // first level-1 heading text, "" when absent
}

func List(root string) (open, closed []Row, err error) {
	issuesDir := filepath.Join(root, "docs", "issues")
	closedDir := filepath.Join(issuesDir, "closed")
	if !exists(issuesDir) || !exists(closedDir) {
		return nil, nil, fmt.Errorf("issues: docs/issues and docs/issues/closed must exist under the repo root")
	}

	openNames, err := listNames(issuesDir, "open")
	if err != nil {
		return nil, nil, err
	}
	closedNames, err := listNames(closedDir, "closed")
	if err != nil {
		return nil, nil, err
	}

	seen := make(map[int]bool)
	for _, row := range append(slices.Clone(openNames), closedNames...) {
		if seen[row.Number] {
			return nil, nil, fmt.Errorf("issues: duplicate issue number %d (present on both sides of docs/issues): %s", row.Number, row.Base)
		}
		seen[row.Number] = true
	}

	return openNames, closedNames, nil
}

func listNames(dir, side string) ([]Row, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var rows []Row
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") || e.IsDir() {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".md")
		n, convErr := strconv.Atoi(base)
		if convErr != nil || !isDigitsOnly(base) {
			return nil, fmt.Errorf("issues: unexpected issue filename %q in %s (want NN.md)", e.Name(), side)
		}
		rows = append(rows, Row{Number: n, Base: e.Name(), Side: side})
	}
	slices.SortStableFunc(rows, func(a, b Row) int {
		return natural.Compare(b.Base, a.Base)
	})
	return rows, nil
}

func isDigitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
