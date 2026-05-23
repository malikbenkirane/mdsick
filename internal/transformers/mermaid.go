package transformers

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"sync"

	"github.com/malikbenkirane/mdsick/internal/mermaid"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type Mermaid struct {
	err    chan error
	change chan change
}

type change struct {
	oldChild ast.Node
	newChild ast.Node
	parent   ast.Node
}

func (m *Mermaid) render(code []byte, prepare change) {
	var svg bytes.Buffer
	if err := mermaid.Compile(bytes.NewBuffer(code), &svg); err != nil {
		m.err <- err
	}
	var svg64 bytes.Buffer
	{
		enc := base64.NewEncoder(base64.StdEncoding, &svg64)
		_, err := enc.Write(svg.Bytes())
		if err != nil {
			m.err <- err
		}
	}
	ln := ast.NewLink()
	ln.Destination = bytes.Join([][]byte{[]byte("data:image/svg+xml;base64"), svg64.Bytes()},
		[]byte{','})
	prepare.newChild = ast.NewImage(ln)
	m.change <- prepare
}

func (m *Mermaid) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {

	var wg sync.WaitGroup

	var changes []change
	var count, left int
	var mu sync.Mutex

	m.change = make(chan change, 1)
	m.err = make(chan error, 1)
	defer close(m.change)
	defer close(m.err)

	go func() {
		for {
			select {
			case err, ok := <-m.err:
				if !ok {
					return
				}
				fmt.Fprintln(os.Stderr, err)
			case change, ok := <-m.change:
				if !ok {
					return
				}
				mu.Lock()
				changes = append(changes, change)
				left--
				mu.Unlock()
				fmt.Fprintln(os.Stderr, "mermaid:", left, "/", count)
			}
		}
	}()

	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.FencedCodeBlock:
			if string(n.Language(reader.Source())) != "mermaid" {
				return ast.WalkContinue, nil
			}
			wg.Go(func() {
				m.render(n.Lines().Value(reader.Source()),
					change{
						parent:   n.Parent(),
						oldChild: n,
					})
			})
			mu.Lock()
			count++
			left++
			mu.Unlock()
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	wg.Wait()

	for _, change := range changes {
		change.parent.ReplaceChild(change.parent, change.oldChild, change.newChild)
	}

}
