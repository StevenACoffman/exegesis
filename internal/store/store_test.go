package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/store"
)

func TestChunk(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		text     string
		maxRunes int
		wantMax  int // every chunk must be <= this many runes
		wantLen  int
	}{
		"fits in one":  {"short text", 100, 100, 1},
		"no limit":     {"anything at all", 0, 1 << 30, 1},
		"splits paras": {"aaaa\n\nbbbb\n\ncccc", 5, 5, 3},
		"hard split":   {strings.Repeat("x", 25), 10, 10, 3},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			chunks := store.Chunk(tc.text, tc.maxRunes)
			if len(chunks) != tc.wantLen {
				t.Fatalf("got %d chunks, want %d: %q", len(chunks), tc.wantLen, chunks)
			}
			for _, c := range chunks {
				if n := utf8.RuneCountInString(c); n > tc.wantMax {
					t.Errorf("chunk %q has %d runes, want <= %d", c, n, tc.wantMax)
				}
			}
		})
	}
}

func TestLoadTextRejectsBinaryFormats(t *testing.T) {
	t.Parallel()
	_, err := store.LoadText("book.pdf")
	if book2skill.ErrorCode(err) != book2skill.EINVALID {
		t.Errorf("ErrorCode = %q, want %q", book2skill.ErrorCode(err), book2skill.EINVALID)
	}
}

func TestLoadTextReadsPlainText(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "book.txt")
	if err := os.WriteFile(path, []byte("hello book"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	got, err := store.LoadText(path)
	if err != nil {
		t.Fatalf("LoadText: %v", err)
	}
	if got != "hello book" {
		t.Errorf("got %q, want %q", got, "hello book")
	}
}

func TestWriterCreatesParents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := store.NewWriter(dir)
	if err := w.WriteFile("nested/deep/SKILL.md", []byte("body")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "nested", "deep", "SKILL.md"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "body" {
		t.Errorf("got %q, want %q", got, "body")
	}
}
