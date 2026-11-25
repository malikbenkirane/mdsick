package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/malikbenkirane/mdsick/internal/issues"
	"github.com/malikbenkirane/mdsick/internal/markdown"
	"github.com/malikbenkirane/mdsick/internal/workspace"
	"github.com/spf13/cobra"
	"github.com/yuin/goldmark/text"
)

var vcs string

// build renders every issue page flat in the workspace (NN.md ->
// NN.html regardless of open/closed side), writes the landing page,
// and hands the workspace directory plus page path to `show`.
// rootPath is the repo root; cwd may have changed to the workspace by
// the time show runs, so sources are read via their absolute paths.
func build(rootPath string, titleArgs []string, show func(page string) error) (string, error) {
	original, err := os.Getwd()
	if err != nil {
		return "", err
	}

	open, closed, err := issues.List(rootPath)
	if err != nil {
		return "", err
	}

	title := defaultTitle(rootPath)
	if len(titleArgs) > 0 {
		title = strings.Join(titleArgs, " ")
	}

	tmp, err := workspace.TempWorkspace()
	if err != nil {
		return "", err
	}

	ix := issues.NewIndex(open, closed)
	var warns []issues.Warning

	for _, rows := range [][]issues.Row{open, closed} {
		for _, row := range rows {
			srcPath := filepath.Join(original, "docs", "issues", row.Base)
			if row.Side == "closed" {
				srcPath = filepath.Join(original, "docs", "issues", "closed", row.Base)
			}
			source, err := os.ReadFile(srcPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			md := markdown.NewIssues(ix, row.Base, &warns)
			doc := md.Parser().Parse(text.NewReader(source))
			name := fmt.Sprintf("%d.html", row.Number)
			if err := workspace.RenderFile(name, row.Base, md, source, doc); err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			setHead(open, closed, row.Number, markdown.DocTitle(source))
		}
	}

	page := filepath.Join(tmp, "index.html")
	if err := os.Chdir(tmp); err != nil {
		return tmp, err
	}
	if err := writeIssuesIndex(title, open, closed, warns); err != nil {
		return tmp, err
	}
	if show != nil {
		if err := show(page); err != nil {
			return tmp, err
		}
	}
	return tmp, nil
}

func setHead(open, closed []issues.Row, number int, head string) {
	for i := range open {
		if open[i].Number == number {
			open[i].Head = head
		}
	}
	for i := range closed {
		if closed[i].Number == number {
			closed[i].Head = head
		}
	}
}

// defaultTitle: path/to/repo -> "to/repo Issues" (last two components).
func defaultTitle(rootPath string) string {
	return filepath.Base(filepath.Dir(rootPath)) + "/" + filepath.Base(rootPath) + " Issues"
}

func main() {
	cmd := &cobra.Command{
		Use:  "mdsick-issues [Title]",
		Long: "Render docs/issues and docs/issues/closed into one GitHub-styled landing page with flat per-issue pages.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootPath, err := issues.FindRepo(vcs)
			if err != nil {
				return err
			}
			if _, err := os.Stat(filepath.Join(rootPath, "docs", "issues")); err != nil {
				return fmt.Errorf("issues: docs/issues missing under %s", rootPath)
			}
			_, err = build(rootPath, args, func(page string) error {
				return exec.Command("open", page).Run()
			})
			return err
		},
	}
	cmd.Flags().StringVar(&vcs, "vcs", "auto", "repo marker preference: auto (jj first, then git), jj, git")
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
