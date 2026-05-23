package toc

import (
	"testing"
)

func TestTOC(t *testing.T) {
	for _, test := range []struct {
		name     string
		title    string
		expected string
	}{
		{
			title:    "Alpha Numeric 1 Entry",
			expected: "#alpha-numeric-1-entry",
		},
		{
			title:    "With Symbol @",
			expected: "#with-symbol-",
		},
		{
			title:    "With --dashes",
			expected: "#with---dashes",
		},
		{
			title:    "With < > More Symbols (There)",
			expected: "#with---more-symbols-there",
		},
		{title: "With    Repeated Spaces",
			expected: "#with----repeated-spaces",
		},
	} {
		if test.name == "" {
			test.name = test.expected
		}
		t.Run(test.name, func(t *testing.T) {
			got := Id(test.title)
			if got != test.expected {
				t.Fatalf("unexpected title:\ngot %q\nexp %q", got, test.expected)
			}
		})
	}
}
