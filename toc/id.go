package toc

import (
	"strings"
	"unicode"
)

func Id(title string) string {
	var s strings.Builder
	s.WriteRune('#')
	for _, r := range title {
		switch r {
		case '(', ')', '@':
			continue
		case ' ', '-':
			s.WriteRune('-')
		default:
			r = unicode.ToLower(r)
			if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
				continue
			}
			s.WriteRune(r)
		}
	}
	return s.String()
}
