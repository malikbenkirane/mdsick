package main

import (
	"errors"
	"fmt"
	"html"
	"net/url"
	"os"
	"strings"

	"github.com/malikbenkirane/mdsick/internal/issues"
	"github.com/malikbenkirane/mdsick/internal/workspace"
)

// writeIssuesIndex renders the single landing page: two fixed sections
// (Open Issues, Closed Issues), reverse-natural-ordered entries, then a
// plain markdown-heading warnings block. Styling stays inside the
// markdown-body shell used by every mdsick page.
func writeIssuesIndex(title string, open, closed []issues.Row, warns []issues.Warning) (err error) {
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
`, workspace.StyleRef, workspace.PaddingHorizontal, workspace.PaddingVertical, html.EscapeString(title))
	if err != nil {
		return err
	}

	if err := writeSection(f, "Open Issues", open); err != nil {
		return err
	}
	if err := writeSection(f, "Closed Issues", closed); err != nil {
		return err
	}

	if len(warns) > 0 {
		if _, err = fmt.Fprintf(f, "\t\t<h2>Warnings</h2>\n\t\t<pre>\n"); err != nil {
			return err
		}
		for _, w := range warns {
			if _, err = fmt.Fprintf(f, "\t\t\t%s\n", html.EscapeString(w.String())); err != nil {
				return err
			}
		}
		if _, err = f.WriteString("\t\t</pre>\n"); err != nil {
			return err
		}
	}

	_, err = fmt.Fprintf(f, "\t</body>\n</html>\n")
	return err
}

func writeSection(f *os.File, title string, rows []issues.Row) error {
	if _, err := fmt.Fprintf(f, "\t\t<h2>%s</h2>\n", html.EscapeString(title)); err != nil {
		return err
	}
	if len(rows) == 0 {
		_, err := fmt.Fprintf(f, "\t\t<p>No %s issues.</p>\n", strings.ToLower(title))
		return err
	}
	if _, err := f.WriteString("\t\t<ul>\n"); err != nil {
		return err
	}
	for _, row := range rows {
		href := fmt.Sprintf("%d.html", row.Number)
		name := html.EscapeString(row.Base)
		if _, err := fmt.Fprintf(f, "\t\t\t<li><a href=\"%s\">%s</a>", url.PathEscape(href), name); err != nil {
			return err
		}
		if row.Head != "" {
			if _, err := fmt.Fprintf(f, " <span class=\"file-title\">%s</span>", html.EscapeString(row.Head)); err != nil {
				return err
			}
		}
		if _, err := f.WriteString("</li>\n"); err != nil {
			return err
		}
	}
	_, err := f.WriteString("\t\t</ul>\n")
	return err
}
