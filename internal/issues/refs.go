package issues

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// NewTransformer builds a per-document transformer bound to one issue
// file: warnings carry `source` as their origin.
func NewTransformer(index *Index, source string, warnings *[]Warning) *Transformer {
	return &Transformer{index: index, source: source, warnings: warnings}
}

// Transformer rewrites issue references in one document. One instance
// per file; instantiate fresh for every document (it holds per-file
// state: source filename for warnings and the collected mutations).
type Transformer struct {
	index    *Index
	source   string
	warnings *[]Warning
	edits    []edit
}

type edit struct {
	parent ast.Node
	old    ast.Node
	new    []ast.Node
}

var (
	hashRe  = regexp.MustCompile(`(^|[^\w])#(\d+)\b`)
	issueRe = regexp.MustCompile(`(?i)(\bissue[ \t]+(?:closed[ \t]+)?)(\d+)(\b)`)
	pathRe  = regexp.MustCompile(`\b(?:docs/issues(?:/closed)?/)?(\d+)\.(?:md)\b`)
)

func (t *Transformer) warn(source, ref string) {
	*t.warnings = append(*t.warnings, Warning{Source: source, Ref: ref})
}

// splitAndLink walks re.FindAllSubmatchIndex over val and builds a node
// sequence where each reference match becomes
// [leading-String, link(match), trailing-String ...] fragments.
// `hash` marks the `#NN` tier: the link label starts one byte before
// the number (at the '#'), leaving any boundary char in the leading
// fragment.
func (t *Transformer) splitAndLink(val []byte, re *regexp.Regexp, numGroup int, hash bool) []ast.Node {
	var out []ast.Node
	last := 0
	locs := re.FindAllSubmatchIndex(val, -1)
	if locs == nil {
		return nil
	}
	for _, loc := range locs {
		start, stop := loc[2*numGroup], loc[2*numGroup+1]
		num, _ := strconv.Atoi(string(val[start:stop]))
		href, ok := t.index.Resolve(num)
		if !ok {
			if hash {
				t.warn(t.source, string(val[start-1:stop]))
			} else {
				t.warn(t.source, string(val[start:stop]))
			}
			continue
		}
		labelStart := loc[0]
		if hash {
			labelStart = start - 1
		}
		if labelStart < last {
			continue
		}
		if labelStart > last {
			out = append(out, str(val[last:labelStart]))
		}
		label := val[labelStart:loc[1]]
		ln := ast.NewLink()
		ln.Destination = []byte(href)
		ln.AppendChild(ln, ast.NewString(label))
		out = append(out, ln)
		last = loc[1]
	}
	if last < len(val) {
		out = append(out, str(val[last:]))
	}
	return out
}

func str(b []byte) ast.Node { return ast.NewString(b) }

func (t *Transformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.FencedCodeBlock:
			return ast.WalkSkipChildren, nil
		case *ast.CodeSpan:
			if nodes := t.splitAndLink(codeSpanText(n, reader.Source()), pathRe, 1, false); nodes != nil {
				t.edits = append(t.edits, edit{parent: n.Parent(), old: n, new: nodes})
			}
			return ast.WalkSkipChildren, nil
		case *ast.Text:
			if nodes := t.rewriteText(n, reader); nodes != nil {
				parent := n.Parent()
				t.edits = append(t.edits, edit{parent: parent, old: n, new: nodes})
			}
		case *ast.Link:
			t.rewriteDest(n, n.Destination)
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	for _, e := range t.edits {
		parent := e.parent
		parent.ReplaceChild(parent, e.old, e.new[0])
		prev := e.new[0]
		for _, extra := range e.new[1:] {
			parent.InsertAfter(parent, prev, extra)
			prev = extra
		}
	}
}

func (t *Transformer) rewriteText(n *ast.Text, reader text.Reader) []ast.Node {
	val := n.Segment.Value(reader.Source())
	nodes := t.splitAndLink(val, hashRe, 2, true)
	if nodes == nil {
		nodes = t.splitAndLink(val, pathRe, 1, false)
	}
	if nodes == nil {
		nodes = t.splitAndLink(val, issueRe, 2, false)
	}
	return nodes
}

// codeSpanText concatenates a code span's children into one value.
func codeSpanText(n *ast.CodeSpan, src []byte) []byte {
	var b []byte
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b = append(b, t.Segment.Value(src)...)
		}
	}
	return b
}

func (t *Transformer) rewriteDest(link *ast.Link, dst []byte) {
	loc := pathRe.FindSubmatch(dst)
	if loc == nil {
		return
	}
	num := atoi(loc[1])
	href, ok := t.index.Resolve(num)
	if !ok {
		t.warn(t.source, string(dst))
		return
	}
	link.Destination = []byte(href)
}

func atoi(b []byte) int { n, _ := strconv.Atoi(string(b)); return n }
