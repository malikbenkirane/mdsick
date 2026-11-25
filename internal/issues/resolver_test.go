package issues

import "testing"

func TestNewIndex(t *testing.T) {
	open := []Row{{Number: 21}, {Number: 3}}
	closed := []Row{{Number: 10}}
	ix := NewIndex(open, closed)

	if href, ok := ix.Resolve(21); !ok || href != "21.html" {
		t.Fatalf("Resolve(21) = %q,%v; want 21.html,true", href, ok)
	}
	if _, ok := ix.Resolve(10); !ok {
		t.Fatal("closed issue must resolve")
	}
	if href, ok := ix.Resolve(44); ok || href != "" {
		t.Fatalf("Resolve(44) = %q,%v; want \"\",false", href, ok)
	}
}
