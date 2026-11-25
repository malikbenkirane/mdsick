package main

import (
	"os"
	"strings"
	"testing"

	"github.com/malikbenkirane/mdsick/internal/issues"
)

func TestWriteIssuesIndex(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	})

	open := []issues.Row{
		{Number: 21, Base: "21.md", Side: "open", Head: "First"},
		{Number: 3, Base: "3.md", Side: "open"},
	}
	closed := []issues.Row{{Number: 10, Base: "10.md", Side: "closed", Head: "Archived"}}
	warns := []issues.Warning{{Source: "3.md", Ref: "#44"}}

	if err := writeIssuesIndex("Repo & Issues", open, closed, warns); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatal(err)
	}
	h := string(got)

	for _, want := range []string{
		"<title>Repo &amp; Issues</title>",
		"<h1>Repo &amp; Issues</h1>",
		"Open Issues", "Closed Issues",
		`<a href="21.html">21.md</a> <span class="file-title">First</span>`,
		`<a href="10.html">10.md</a> <span class="file-title">Archived</span>`,
		"<h2>Warnings</h2>", "3.md: #44",
	} {
		if !strings.Contains(h, want) {
			t.Fatalf("missing %q in:\n%s", want, h)
		}
	}
}

func TestWriteIssuesIndexEmptySection(t *testing.T) {
	wd, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	})

	if err := writeIssuesIndex("Solo", []issues.Row{{Number: 7, Base: "7.md", Side: "open"}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatal(err)
	}
	h := string(got)
	if !strings.Contains(h, "No closed issues") {
		t.Fatalf("missing empty-section disclaimer:\n%s", h)
	}
	if strings.Contains(h, "No open issues") {
		t.Fatalf("open section should list entries:\n%s", h)
	}
}
