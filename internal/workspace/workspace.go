package workspace

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/malikbenkirane/mdsick/style"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
)

const (
	styleRef          = "github-markdown.css"
	paddingHorizontal = "6.47rem"
	paddingVertical   = "4rem"
)

// TempWorkspace creates a fresh temp dir, prints its path to stderr,
// copies the embedded github-style sheet into it, and chdirs into it.
// Both binaries must call this BEFORE parsing any document so that
// mermaid-generated sidecar files (svg) land in the workspace.
func TempWorkspace() (string, error) {
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}

	fmt.Fprintln(os.Stderr, tmp)

	if err := CopyStyle(tmp); err != nil {
		return "", err
	}

	if err := os.Chdir(tmp); err != nil {
		return "", err
	}

	return tmp, nil
}

func CopyStyle(to string) (err error) {
	dst, err := os.Create(filepath.Join(to, styleRef))
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, dst.Close())
	}()

	if _, err = io.Copy(dst, strings.NewReader(style.Github)); err != nil {
		return err
	}

	return nil
}

// RenderFile renders a parsed markdown document into `to` using the
// shared GitHub-styled shell. The double `</body></html>` writing is a
// long-standing rendering quirk; keep it.
func RenderFile(to, title string, md goldmark.Markdown, source []byte, doc ast.Node) (err error) {
	f, err := os.Create(to)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	if _, err = f.WriteString(fmt.Sprintf(`<!doctype html>
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
	<body class="markdown-body">`,
		styleRef, paddingHorizontal, paddingVertical, title,
	)); err != nil {
		return err
	}

	if _, err = f.WriteString(`
	</body>
</html>`); err != nil {
		return err
	}

	if err := md.Renderer().Render(f, source, doc); err != nil {
		return err
	}

	_, err = f.WriteString("</body></html>")
	return err
}
