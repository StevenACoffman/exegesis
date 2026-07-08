// Package store is the filesystem adapter for book2skill: it loads book text
// and both writes and reads back the books/<slug>/ output tree. Text chunking is
// pure and lives here alongside the I/O it feeds.
package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

const (
	// SkillFile is the per-skill document within a skill directory.
	SkillFile = "SKILL.md"
	// OverviewFile is the Stage-0 overview at a book tree's root.
	OverviewFile = "BOOK_OVERVIEW.md"
	// IndexFile is the Zettelkasten index at a book tree's root.
	IndexFile = "INDEX.md"

	dirPerm  = 0o755
	filePerm = 0o644

	paragraphSep = "\n\n"
)

// Writer writes files beneath a root directory, creating parents as needed.
type Writer struct {
	root string
}

// LoadText reads a plain-text or markdown book file. Binary formats (PDF, EPUB)
// are rejected with EINVALID: extract them to text first.
func LoadText(path string) (string, error) {
	const op = "store.LoadText"
	switch strings.ToLower(filepath.Ext(path)) {
	case "", ".txt", ".md", ".markdown", ".text":
	default:
		return "", &book2skill.Error{
			Code:    book2skill.EINVALID,
			Message: "unsupported book format " + filepath.Ext(path) + "; provide plain text",
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", &book2skill.Error{Op: op, Err: err}
	}
	return string(data), nil
}

// GatherSkills reads every immediate subdirectory of dir that contains a
// SKILL.md into a book2skill.Skill carrying its slug (the directory name),
// title, and relationships. os.ReadDir returns entries sorted by name, so the
// result is deterministic. Directories without a SKILL.md are skipped.
func GatherSkills(dir string) ([]book2skill.Skill, error) {
	const op = "store.GatherSkills"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, &book2skill.Error{Op: op, Err: err}
	}
	var skills []book2skill.Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		slug := e.Name()
		data, err := os.ReadFile(filepath.Join(dir, slug, SkillFile))
		if err != nil {
			continue // not a skill directory
		}
		md := string(data)
		skills = append(skills, book2skill.Skill{
			Slug:    slug,
			Title:   book2skill.ParseTitle(md),
			Related: book2skill.ParseRelated(slug, md),
		})
	}
	return skills, nil
}

// ReadOverview reads and parses dir/BOOK_OVERVIEW.md into the Stage-0 quality-gate
// fields. ok is false (with a nil error) when the file is absent.
func ReadOverview(dir string) (overview *book2skill.BookOverview, ok bool, err error) {
	data, err := os.ReadFile(filepath.Join(dir, OverviewFile))
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, false, nil
	case err != nil:
		return nil, false, &book2skill.Error{Op: "store.ReadOverview", Err: err}
	default:
		o := book2skill.ParseBookOverview(string(data))
		return &o, true, nil
	}
}

// Chunk splits text into chunks of at most maxRunes code points, preferring
// paragraph boundaries. A paragraph longer than maxRunes is hard-split. A
// non-positive maxRunes returns text unchunked.
func Chunk(text string, maxRunes int) []string {
	if maxRunes <= 0 || utf8.RuneCountInString(text) <= maxRunes {
		return []string{text}
	}

	var chunks []string
	var cur strings.Builder
	curRunes := 0
	flush := func() {
		if cur.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(cur.String()))
			cur.Reset()
			curRunes = 0
		}
	}

	for _, para := range strings.Split(text, paragraphSep) {
		paraRunes := utf8.RuneCountInString(para)
		switch {
		case paraRunes > maxRunes:
			flush()
			chunks = append(chunks, splitRunes(para, maxRunes)...)
		case curRunes > 0 && curRunes+paraRunes > maxRunes:
			flush()
			cur.WriteString(para)
			curRunes = paraRunes
		default:
			if cur.Len() > 0 {
				cur.WriteString(paragraphSep)
				curRunes += len([]rune(paragraphSep))
			}
			cur.WriteString(para)
			curRunes += paraRunes
		}
	}
	flush()
	return chunks
}

// NewWriter returns a Writer rooted at root.
func NewWriter(root string) *Writer {
	return &Writer{root: root}
}

// WriteFile writes data to relPath beneath the writer's root, creating any
// missing parent directories.
func (w *Writer) WriteFile(relPath string, data []byte) error {
	const op = "store.Writer.WriteFile"
	full := filepath.Join(w.root, relPath)
	if err := os.MkdirAll(filepath.Dir(full), dirPerm); err != nil {
		return &book2skill.Error{Op: op, Err: err}
	}
	if err := os.WriteFile(full, data, filePerm); err != nil {
		return &book2skill.Error{Op: op, Err: err}
	}
	return nil
}

// splitRunes hard-splits s into pieces of at most size runes.
func splitRunes(s string, size int) []string {
	runes := []rune(s)
	out := make([]string, 0, len(runes)/size+1)
	for i := 0; i < len(runes); i += size {
		end := min(i+size, len(runes))
		out = append(out, string(runes[i:end]))
	}
	return out
}
