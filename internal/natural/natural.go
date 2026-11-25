package natural

import (
	"cmp"
	"strings"
)

func Compare(a, b string) int {
	for len(a) > 0 && len(b) > 0 {
		if isDigit(a[0]) && isDigit(b[0]) {
			var an, bn string
			an, a = leadingDigits(a)
			bn, b = leadingDigits(b)
			if c := compareNumbers(an, bn); c != 0 {
				return c
			}
			continue
		}
		if c := cmp.Compare(a[0], b[0]); c != 0 {
			return c
		}
		a, b = a[1:], b[1:]
	}
	switch {
	case len(a) > 0:
		return 1
	case len(b) > 0:
		return -1
	}
	return 0
}

func isDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

func leadingDigits(s string) (digits, rest string) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i], s[i:]
}

func compareNumbers(a, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if c := cmp.Compare(len(a), len(b)); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}
