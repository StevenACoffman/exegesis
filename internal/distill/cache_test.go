package distill_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/distill"
)

func TestCache(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c := distill.NewCache(dir)
	const id = "abc123"

	if c.Has(id) {
		t.Fatal("empty cache should not have the id")
	}
	if _, err := c.Read(id); err == nil {
		t.Error("Read of a missing id should error")
	}
	p := c.Path(id)
	if filepath.Dir(p) != dir {
		t.Errorf("Path dir = %s, want %s", filepath.Dir(p), dir)
	}

	if err := os.WriteFile(p, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	if !c.Has(id) {
		t.Error("cache should have the id after a response is written")
	}
	b, err := c.Read(id)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(b) != `{"ok":true}` {
		t.Errorf("Read = %s, want the written bytes", b)
	}
}
