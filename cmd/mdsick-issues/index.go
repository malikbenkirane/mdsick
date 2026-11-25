package main

import (
	"errors"
	"fmt"
	"html"
	"net/url"
	"os"
	"strconv"
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
		if err := writeWarningsTable(f, foldWarnings(warns)); err != nil {
			return err
		}
	}

	_, err = fmt.Fprintf(f, "\t</body>\n</html>\n")
	return err
}

// issueNumberOf extracts the flat-page number from a warning's source
// file ("43.md" -> 43). The List contract guarantees all-digit bases;
// ok=false is a defensive fallback for direct callers.
func issueNumberOf(source string) (n int, ok bool) {
	base := strings.TrimSuffix(source, ".md")
	n, err := strconv.Atoi(base)
	if err != nil || base == "" {
		return 0, false
	}
	return n, true
}

func writeWarningsTable(f *os.File, rows []fileWarnings) (err error) {
	if _, err = fmt.Fprint(f, "\t\t<h2>Warnings</h2>\n\t\t<table>\n\t\t\t<thead>\n\t\t\t\t<tr><th>File</th><th>Ref</th></tr>\n\t\t\t</thead>\n\t\t\t<tbody>\n"); err != nil {
		return err
	}
	for _, row := range rows {
		cell := fmt.Sprintf(`<td>%s</td>`, html.EscapeString(row.Source))
		if n, ok := issueNumberOf(row.Source); ok {
			cell = fmt.Sprintf(`<td><a href="%d.html">#%d</a></td>`, n, n)
		}
		if _, err = fmt.Fprint(f, "\t\t\t\t<tr>"+cell+"<td>\n"); err != nil {
			return err
		}
		for _, ref := range row.Refs {
			if _, err = fmt.Fprintf(f, "\t\t\t\t\t<code>%s</code>\n", html.EscapeString(ref)); err != nil {
				return err
			}
		}
		if _, err = fmt.Fprint(f, "\t\t\t\t</td></tr>\n"); err != nil {
			return err
		}
	}
	_, err = fmt.Fprint(f, "\t\t\t</tbody>\n\t\t</table>\n")
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
