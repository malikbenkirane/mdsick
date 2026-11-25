package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/malikbenkirane/mdsick/internal/markdown"
	"github.com/malikbenkirane/mdsick/internal/natural"
	"github.com/malikbenkirane/mdsick/internal/workspace"
	"github.com/yuin/goldmark/text"
)

func run() error {
	if stdinIsTerminal() {
		return runDir()
	}

	var b bytes.Buffer
	_, err := io.Copy(&b, os.Stdin)
	if err != nil {
		return err
	}
	source := b.Bytes()

	if err := checkMmdc(source); err != nil {
		return err
	}

	md := markdown.New()

	tmp, err := workspace.TempWorkspace()
	if err != nil {
		return err
	}

	r := text.NewReader(source)
	doc := md.Parser().Parse(r)

	const dst = "rendered.html"

	flag.Parse()
	args := flag.Args()
	title := dst
	if len(args) > 0 {
		title = strings.Join(args, " ")
	}

	if err := workspace.RenderFile(dst, title, md, source, doc); err != nil {
		return err
	}

	return exec.Command("open", filepath.Join(tmp, dst)).Run()
}

func stdinIsTerminal() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

func runDir() error {
	flag.Parse()
	args := flag.Args()
	title := "Index"
	if len(args) > 0 {
		title = strings.Join(args, " ")
	}

	files, err := filepath.Glob("*.md")
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "mdsick: no markdown files in current directory")
		return nil
	}

	if err := checkMmdcBatch(); err != nil {
		return err
	}

	slices.SortStableFunc(files, natural.Compare)

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	tmp, err := workspace.TempWorkspace()
	if err != nil {
		return err
	}

	var rendered []indexedFile
	for _, file := range files {
		source, err := os.ReadFile(filepath.Join(wd, file))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		base := strings.TrimSuffix(file, filepath.Ext(file))

		md := markdown.New()
		doc := md.Parser().Parse(text.NewReader(source))

		if err := workspace.RenderFile(base+".html", base, md, source, doc); err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		rendered = append(rendered, indexedFile{name: file, title: markdown.DocTitle(source)})
	}

	if len(rendered) == 0 {
		return nil
	}

	if err := writeIndex(title, rendered); err != nil {
		return err
	}

	return exec.Command("open", filepath.Join(tmp, "index.html")).Run()
}

type indexedFile struct {
	name  string
	title string
}

func hasMermaid(source []byte) bool {
	return strings.Contains(string(source), "```mermaid")
}

func checkMmdc(source []byte) error {
	if !hasMermaid(source) {
		return nil
	}
	if _, err := exec.LookPath("mmdc"); err != nil {
		return fmt.Errorf("mdsick: mmdc not found on PATH; mermaid blocks require it")
	}
	return nil
}

func checkMmdcBatch() error {
	if _, err := exec.LookPath("mmdc"); err != nil {
		return fmt.Errorf("mdsick: mmdc not found on PATH; directory markdown may contain mermaid blocks that require it")
	}
	return nil
}

func writeIndex(title string, files []indexedFile) (err error) {
	f, err := os.Create("index.html")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	_, err = fmt.Fprintf(f, `<!doctype html>
<html>
	<head>
		<title>%[4]s</title>
		<link rel="stylesheet" href="%[1]s">
		<style>
			body {
				padding-left: %[2]s;
				padding-right: %[2]s;
				padding-top: %[3]s;
				padding-bottom: %[3]s;
			}
			.file-title {
				color: #656d76;
				font-weight: normal;
				margin-left: 0.5rem;
			}
		</style>
	</head>
	<body class="markdown-body">
		<h1>%[4]s</h1>
		<ul>
`, workspace.StyleRef, workspace.PaddingHorizontal, workspace.PaddingVertical, html.EscapeString(title))
	if err != nil {
		return err
	}

	for _, file := range files {
		href := strings.TrimSuffix(file.name, filepath.Ext(file.name)) + ".html"
		_, err = fmt.Fprintf(f, "\t\t\t<li><a href=\"%s\">%s</a>", url.PathEscape(href), html.EscapeString(file.name))
		if err != nil {
			return err
		}
		if file.title != "" {
			_, err = fmt.Fprintf(f, " <span class=\"file-title\">%s</span>", html.EscapeString(file.title))
			if err != nil {
				return err
			}
		}
		_, err = f.WriteString("</li>\n")
		if err != nil {
			return err
		}
	}

	_, err = f.WriteString("\t\t</ul>\n\t</body>\n</html>")

return err
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
