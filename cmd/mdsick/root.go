package main

import (
	"bytes"
	_ "embed"
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

	"github.com/malikbenkirane/mdsick/internal/natural"
	"github.com/malikbenkirane/mdsick/internal/transformers"
	"github.com/malikbenkirane/mdsick/style"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/toc"
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

	md := newMarkdown()

	tmp, err := tempWorkspace()
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

	if err := render(dst, title, md, source, doc); err != nil {
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

	tmp, err := tempWorkspace()
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

		md := newMarkdown()
		doc := md.Parser().Parse(text.NewReader(source))

		if err := render(base+".html", base, md, source, doc); err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		rendered = append(rendered, indexedFile{name: file, title: docTitle(source)})
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

var titleMarkdown = goldmark.New()

func docTitle(source []byte) string {
	doc := titleMarkdown.Parser().Parse(text.NewReader(source))
	heading, ok := doc.FirstChild().(*ast.Heading)
	if !ok || heading.Level != 1 {
		return ""
	}

	var title bytes.Buffer
	if err := ast.Walk(heading, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindText {
			title.Write(n.(*ast.Text).Value(source))
		}
		return ast.WalkContinue, nil
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}

	resolved := util.ResolveNumericReferences(title.Bytes())
	resolved = util.ResolveEntityNames(resolved)
	resolved = util.UnescapePunctuations(resolved)
	return strings.TrimSpace(string(resolved))
}

func tempWorkspace() (string, error) {
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}

	fmt.Fprintln(os.Stderr, tmp)

	if err := copyStyle(tmp); err != nil {
		return "", err
	}

	if err := os.Chdir(tmp); err != nil {
		return "", err
	}

	return tmp, nil
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

func newMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.PrioritizedValue{
				Value: transformers.CodeLiteralRenderer{},
			}),
		),
		goldmark.WithParserOptions(
			parser.WithASTTransformers(util.PrioritizedValue{
				Value: &transformers.Mermaid{},
			}),
			parser.WithAutoHeadingID(),
		),
		goldmark.WithExtensions(&toc.Extender{}),
		goldmark.WithExtensions(extension.Linkify),
		goldmark.WithExtensions(extension.Strikethrough),
		goldmark.WithExtensions(extension.Table),
	)
}

func render(to, title string, md goldmark.Markdown, source []byte, doc ast.Node) (err error) {
	f, err := os.Create(to)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	_, err = f.WriteString(fmt.Sprintf(`<!doctype html>
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
		</style>
	</head>
	<body class="markdown-body">`, styleRef, paddingHorizontal, paddingVertical, title))
	if err != nil {
		return err
	}

	_, err = f.WriteString(`
	</body>
</html>`)
	if err != nil {
		return err
	}

	if err := md.Renderer().Render(f, source, doc); err != nil {
		return err
	}

	_, err = f.WriteString("</body></html>")
	if err != nil {
		return err
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
`, styleRef, paddingHorizontal, paddingVertical, html.EscapeString(title))
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

const (
	styleRef          = "github-markdown.css"
	paddingHorizontal = "6.47rem"
	paddingVertical   = "4rem"
)

func copyStyle(to string) (err error) {
	dst, err := os.Create(filepath.Join(to, styleRef))
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, dst.Close())
	}()

	src := strings.NewReader(style.Github)

	if _, err = io.Copy(dst, src); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
