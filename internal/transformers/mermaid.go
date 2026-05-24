package transformers

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"sync"

	"github.com/malikbenkirane/mdsick/internal/mermaid"
	"github.com/schollz/progressbar/v3"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type Mermaid struct {
	event chan event
	ready chan struct{}
	bar   *progressbar.ProgressBar
}

type event struct {
	change *change
	err    error
}

type change struct {
	oldChild ast.Node
	newChild ast.Node
	parent   ast.Node
}

func randHex(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (m *Mermaid) render(code []byte, prepare change) {
	var svg bytes.Buffer
	if err := mermaid.Compile(bytes.NewBuffer(code), &svg); err != nil {
		m.event <- event{err: err}
		return
	}

	name, err := randHex(32)
	if err != nil {
		m.event <- event{err: err}
		return
	}

	f, err := os.Create(name + ".svg")
	if err != nil {
		m.event <- event{err: err}
		return
	}

	_, err = f.Write(svg.Bytes())
	if err != nil {
		m.event <- event{err: err}
		return
	}

	var svg64 bytes.Buffer
	{
		enc := base64.NewEncoder(base64.StdEncoding, &svg64)
		_, err := enc.Write(svg.Bytes())
		if err != nil {
			m.event <- event{err: err}
			return
		}
	}
	ln := ast.NewLink()
	ln.Destination = []byte(f.Name())
	prepare.newChild = ast.NewImage(ln)
	prepare.newChild.SetAttributeString("style", "max-height: 500px; width: auto;")
	m.event <- event{change: &prepare}
}

func (m *Mermaid) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {

	var wg sync.WaitGroup

	var changes []*change
	var mu sync.Mutex

	m.event = make(chan event, 1)
	m.ready = make(chan struct{})
	defer close(m.event)
	defer close(m.ready)

	var count, done int

	wg.Go(func() {
		_, ok := <-m.ready
		if !ok {
			return
		}
		for {
			event, ok := <-m.event
			if !ok {
				fmt.Fprintln(os.Stderr, "mermaid event chan closed")
				return
			}

			if event.err != nil {
				fmt.Fprintln(os.Stderr, event.err)
				continue
			}

			mu.Lock()
			changes = append(changes, event.change)
			_ = m.bar.Add(1)
			done++
			if done == count {
				mu.Unlock()
				m.bar.Describe("mermaiding done")
				return
			}
			mu.Unlock()
		}
	})

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
			count++
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	if count > 0 {
		m.bar = progressbar.New(count)
		m.bar.Describe("mermaiding...")
		m.ready <- struct{}{}
		wg.Wait()
	}

	for _, change := range changes {
		change.parent.ReplaceChild(change.parent, change.oldChild, change.newChild)
	}

}
