package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestHasMermaid(t *testing.T) {
	for _, test := range []struct {
		name string
		doc  string
		want bool
	}{
		{name: "fence", doc: "x\n\n```mermaid\ngraph TD\n```\n", want: true},
		{name: "tilde fence does not count", doc: "~~~mermaid\ngraph TD\n~~~\n", want: false},
		{name: "no mermaid", doc: "# hi\n\nbody\n", want: false},
		{name: "inline mention", doc: "use ```mermaid fences\n", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := hasMermaid([]byte(test.doc)); got != test.want {
				t.Fatalf("hasMermaid:\ngot %v\nexp %v", got, test.want)
			}
		})
	}
}

func TestGateBehaviors(t *testing.T) {
	dir := t.TempDir()

	t.Run("missing mmdc errors", func(t *testing.T) {
		t.Setenv("PATH", dir)
		err := checkMmdc([]byte("```mermaid\ngraph TD\n```\n"))
		if err == nil {
			t.Fatal("expected error with mmdc missing")
		}
		if !strings.Contains(err.Error(), "mmdc") {
			t.Fatalf("error must mention mmdc:\n%v", err)
		}
	})

	t.Run("shim present passes", func(t *testing.T) {
		shim := filepath.Join(dir, "mmdc")
		if err := os.WriteFile(shim, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)
		if err := checkMmdc([]byte("```mermaid\ngraph TD\n```\n")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("no mermaid skips the gate", func(t *testing.T) {
		t.Setenv("PATH", dir)
		if err := checkMmdc([]byte("# hi\n\nbody\n")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestNaturalCompare(t *testing.T) {
	for _, test := range []struct {
		a, b string
		want int
	}{
		{a: "1.md", b: "2.md", want: -1},
		{a: "2.md", b: "13.md", want: -1},
		{a: "13.md", b: "2.md", want: 1},
		{a: "9.md", b: "10.md", want: -1},
		{a: "1.md", b: "1.md", want: 0},
		{a: "01.md", b: "1.md", want: 0},
		{a: "a2.md", b: "a10.md", want: -1},
		{a: "a10.md", b: "a2.md", want: 1},
		{a: "12345678901234567890123.md", b: "12345678901234567890124.md", want: -1},
		{a: "abc.md", b: "abd.md", want: -1},
		{a: "1.markdown", b: "1.md", want: -1},
	} {
		t.Run(test.a+"_"+test.b, func(t *testing.T) {
			if got := naturalCompare(test.a, test.b); got != test.want {
				t.Fatalf("naturalCompare(%q, %q):\ngot %d\nexp %d", test.a, test.b, got, test.want)
			}
		})
	}
}

func TestNaturalSort(t *testing.T) {
	files := []string{"10.md", "2.md", "1.md", "13.md", "readme.md", "a10.md", "a2.md"}
	slices.SortStableFunc(files, naturalCompare)

	want := "1.md 2.md 10.md 13.md a2.md a10.md readme.md"
	if got := strings.Join(files, " "); got != want {
		t.Fatalf("unexpected sort:\ngot %s\nexp %s", got, want)
	}
}

func TestDocTitle(t *testing.T) {
	for _, test := range []struct {
		name string
		doc  string
		want string
	}{
		{name: "leading h1", doc: "# Hello World\n\nbody\n", want: "Hello World"},
		{name: "inline markup", doc: "# *Bold*, `code` and [link](http://x)\n", want: "Bold, code and link"},
		{name: "entity", doc: "# Q &amp; A\n", want: "Q & A"},
		{name: "escaped punctuation", doc: "# A \\* B\n", want: "A * B"},
		{name: "h2 first", doc: "## Not a title\n", want: ""},
		{name: "paragraph first", doc: "intro\n\n# Later\n", want: ""},
		{name: "empty", doc: "", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := docTitle([]byte(test.doc)); got != test.want {
				t.Fatalf("unexpected title:\ngot %q\nexp %q", got, test.want)
			}
		})
	}
}

func TestWriteIndex(t *testing.T) {
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

	files := []indexedFile{
		{name: "1.md", title: "First & <Best>"},
		{name: "13.md"},
		{name: "my file.md", title: "Spaced"},
	}
	if err := writeIndex("My & Index", files); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatal(err)
	}

	want := `<!doctype html>
<html>
	<head>
		<title>My &amp; Index</title>
		<link rel="stylesheet" href="github-markdown.css">
		<style>
			body {
				padding-left: 6.47rem;
				padding-right: 6.47rem;
				padding-top: 4rem;
				padding-bottom: 4rem;
			}
			.file-title {
				color: #656d76;
				font-weight: normal;
				margin-left: 0.5rem;
			}
		</style>
	</head>
	<body class="markdown-body">
		<h1>My &amp; Index</h1>
		<ul>
			<li><a href="1.html">1.md</a> <span class="file-title">First &amp; &lt;Best&gt;</span></li>
			<li><a href="13.html">13.md</a></li>
			<li><a href="my%20file.html">my file.md</a> <span class="file-title">Spaced</span></li>
		</ul>
	</body>
</html>`
	if string(got) != want {
		t.Fatalf("unexpected index:\ngot:\n%s\nexp:\n%s", got, want)
	}
}
