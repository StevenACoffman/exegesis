package lint

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/skill"
)

// bundledFile is one text file shipped beside SKILL.md.
type bundledFile struct {
	path string // slash path relative to the skill directory
	text string
}

// CheckBundle returns the findings that need the skill's directory rather than its
// SKILL.md alone: body links that resolve to nothing (error), hidden characters in every
// bundled text file (error; SKILL.md's own are Check's), bundled files nothing references
// (warning), and paths into another skill's directory (warning).
//
// It is separate from Check so Check stays pure over a loaded skill. The directory comes
// in as an fs.FS rooted at the skill, so the commands pass os.DirFS(dir) and tests pass
// an in-memory tree.
//
// Requires: fsys is rooted at s.Dir.
// Ensures:  every diagnostic names a file or link inside the skill; it reads fsys only.
func CheckBundle(fsys fs.FS, s *skill.Skill) []finding.Diagnostic {
	var ds []finding.Diagnostic
	for _, dest := range skill.BrokenLinks(fsys, s.Body) {
		ds = append(
			ds,
			diagf("body: broken link '%s' — no such file or directory in the skill", dest),
		)
	}
	files, err := bundledFiles(fsys)
	if err != nil {
		return append(ds, diagf("bundle: not checked: %v", err))
	}
	for _, f := range files {
		ds = append(ds, hiddenFindings(f.path, f.text)...)
	}
	ds = append(ds, unreferencedFindings(s.Body, files)...)
	self := filepath.Base(s.Dir)
	all := append([]bundledFile{{path: skill.FileName, text: s.Body}}, files...)
	return append(ds, crossSkillFindings(self, all)...)
}

// bundledFiles returns every text file in the skill other than SKILL.md, in walk order.
//
// The hidden-character scan used to stop at SKILL.md, but a skill tells the agent to read
// its reference files, so a payload there is read with the same authority (book2skill's
// methodology files held three zero-width spaces nothing reported). A file is text when
// it is valid UTF-8 with no NUL byte; anything else is not something an agent reads as
// instructions. Dot-directories (.git, tool caches) are not part of the skill and are
// skipped.
func bundledFiles(fsys fs.FS) ([]bundledFile, error) {
	var files []bundledFile
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && p != "." && strings.HasPrefix(d.Name(), "."):
			return fs.SkipDir
		case !d.Type().IsRegular() || p == skill.FileName:
			return nil
		}
		text, isText, err := readText(fsys, p)
		if isText {
			files = append(files, bundledFile{path: p, text: text})
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("walk skill directory: %w", err)
	}
	return files, nil
}

// readText reads p and reports whether it is text: valid UTF-8 with no NUL byte.
func readText(fsys fs.FS, p string) (string, bool, error) {
	b, err := fs.ReadFile(fsys, p)
	if err != nil {
		return "", false, fmt.Errorf("read %s: %w", p, err)
	}
	return string(b), utf8.Valid(b) && !bytes.Contains(b, []byte{0}), nil
}
