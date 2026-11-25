package natural

import (
	"slices"
	"strings"
	"testing"
)

func TestCompare(t *testing.T) {
	for _, test := range []struct {
		a, b string
		want int
	}{
		{"1.md", "2.md", -1},
		{"2.md", "13.md", -1},
		{"13.md", "2.md", 1},
		{"9.md", "10.md", -1},
		{"1.md", "1.md", 0},
		{"01.md", "1.md", 0},
		{"a2.md", "a10.md", -1},
		{"a10.md", "a2.md", 1},
		{"12345678901234567890123.md", "12345678901234567890124.md", -1},
		{"abc.md", "abd.md", -1},
		{"1.markdown", "1.md", -1},
	} {
		t.Run(test.a+"_"+test.b, func(t *testing.T) {
			if got := Compare(test.a, test.b); got != test.want {
				t.Fatalf("Compare(%q, %q):\ngot %d\nexp %d", test.a, test.b, got, test.want)
			}
		})
	}
}

func TestSort(t *testing.T) {
	files := []string{"10.md", "2.md", "1.md", "13.md", "readme.md", "a10.md", "a2.md"}
	slices.SortStableFunc(files, Compare)

	want := "1.md 2.md 10.md 13.md a2.md a10.md readme.md"
	if got := strings.Join(files, " "); got != want {
		t.Fatalf("unexpected sort:\ngot %s\nexp %s", got, want)
	}
}
