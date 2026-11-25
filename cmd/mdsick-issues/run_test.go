package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func chdirToRoot(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatal(err)
		}
	})
}

func writeFixture(t *testing.T, root string, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureRepo(t *testing.T, marker string) string {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "issues", "closed"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, filepath.Join("docs", "issues", "21.md"), "# Alpha\n\nfixes #43 and [closed ten](closed/10.md)\n")
	writeFixture(t, root, filepath.Join("docs", "issues", "3.md"), "# Beta\n\nsee docs/issues/closed/10.md\n")
	writeFixture(t, root, filepath.Join("docs", "issues", "closed", "10.md"), "# Archived\n\nback-references #3 and `21.md`\n")
	writeFixture(t, root, filepath.Join("docs", "issues", "43.md"), "# Dangling\n\nsee #99\n")
	return root
}

// collectHrefs returns every href attribute value in the document, in
// order.
func collectHrefs(t *testing.T, body string) []string {
	t.Helper()
	var hrefs []string
	z := html.NewTokenizer(strings.NewReader(body))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return hrefs
		case html.StartTagToken:
			name, _ := z.TagName()
			if string(name) != "a" {
				continue
			}
			for {
				k, v, more := z.TagAttr()
				if string(k) == "href" {
					hrefs = append(hrefs, string(v))
				}
				if !more {
					break
				}
			}
		}
	}
}

func TestBuild(t *testing.T) {
	root := fixtureRepo(t, ".jj")
	chdirToRoot(t, root)

	show := func(path string) error {
		return nil // browser stub: record nothing, succeed
	}
	tmp, err := build(root, nil, show)
	if err != nil {
		t.Fatal(err)
	}

	mustRead := func(p string) string {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	idx := mustRead(filepath.Join(tmp, "index.html"))
	hrefs := collectHrefs(t, idx)
	if len(hrefs) != 5 {
		t.Fatalf("expected 5 anchors (21, 3, 10, open issue 43 and the warnings row #43), got %v", hrefs)
	}
	for _, g := range []string{"21.html", "3.html", "10.html", "43.html"} {
		found := false
		for _, h := range hrefs {
			if h == g {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing anchor %q in index hrefs %v", g, hrefs)
		}
	}

	page21 := mustRead(filepath.Join(tmp, "21.html"))
	if !strings.Contains(page21, `href="43.html"`) {
		t.Fatalf("hash ref to issue 43 missing:\n%s", page21)
	}
	if !strings.Contains(page21, `href="10.html"`) {
		t.Fatalf("dir-dest link to closed issue missing:\n%s", page21)
	}

	page10 := mustRead(filepath.Join(tmp, "10.html"))
	if !strings.Contains(page10, `href="21.html"`) {
		t.Fatalf("back-reference to open issue missing:\n%s", page10)
	}
	if !strings.Contains(page10, `href="3.html"`) {
		t.Fatalf("back-reference to issue 3 missing:\n%s", page10)
	}
}

func TestBuildWarningsSurfaceOnIndex(t *testing.T) {
	root := fixtureRepoClosedEmpty(t, ".jj")
	chdirToRoot(t, root)

	tmp, err := build(root, nil, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(tmp, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	h := string(body)
	if !strings.Contains(h, "<code>#99</code>") || !strings.Contains(h, `<a href="43.html">#43</a>`) {
		t.Fatalf("unresolvable warning missing from index:\n%s", h)
	}
	if !strings.Contains(h, "No closed issues") {
		t.Fatalf("empty-section disclaimer missing:\n%s", h)
	}
}

func fixtureRepoClosedEmpty(t *testing.T, marker string) string {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "issues", "closed"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, filepath.Join("docs", "issues", "21.md"), "# Alpha\n\nbody\n")
	writeFixture(t, root, filepath.Join("docs", "issues", "43.md"), "# Dangling\n\nsee #99\n")
	return root
}
