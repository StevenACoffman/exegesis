// Package indexgen regenerates a skill tree's INDEX.md from every skill's
// `## Related skills` section — the shared core behind `exegesis index` and
// distill's Stage 3, so the two cannot drift. Rendering is pure
// (internal/related); this package does the skill discovery and reads.
package indexgen

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/exegesis/internal/related"
	"github.com/StevenACoffman/skillet/naming"
	"github.com/StevenACoffman/skillet/skill"
)

// fileName is the generated index file.
const fileName = "INDEX.md"

// Path returns tree's INDEX.md path.
func Path(tree string) string {
	return filepath.Join(tree, fileName)
}

// Generate returns the regenerated INDEX.md content for tree: the skill list,
// the Mermaid relationship graph, and a dependency-ordered learning path built
// from every skill's `## Related skills` section, with any hand-added tail below
// the generated block preserved. title/author override the header derived from
// BOOK_OVERVIEW.md (empty means derive).
func Generate(tree, title, author string) (string, error) {
	nodes, err := CollectNodes(tree)
	if err != nil {
		return "", err
	}
	existing := readFile(Path(tree))
	return related.Render(header(tree, title, author), nodes, related.Split(existing)), nil
}

// CollectNodes loads every skill under tree into a related.Node carrying its slug,
// title, description, and parsed related-skill edges. It is the one tree walk
// behind `index`, `verify`, `relate`, and `link`, so every command keys skills by
// the same slug and a graph check reports exactly what INDEX.md would drop.
//
// A skill whose SKILL.md cannot be loaded is an error, not a gap in the graph:
// silently omitting it would make an edge to it look dangling.
func CollectNodes(tree string) ([]related.Node, error) {
	dirs, err := skill.Discover(tree)
	if err != nil {
		return nil, fmt.Errorf("discover skills: %w", err)
	}
	nodes := make([]related.Node, 0, len(dirs))
	for _, dir := range dirs {
		s, loadErr := skill.Load(dir)
		if loadErr != nil {
			return nil, fmt.Errorf("load %s: %w", dir, loadErr)
		}
		slug := skill.Slug(filepath.Base(dir))
		nodes = append(nodes, related.Node{
			Slug:        slug,
			Title:       naming.Title(slug),
			Description: s.Description,
			Edges:       related.ParseSection(s.Body),
		})
	}
	return nodes, nil
}

// header resolves the INDEX.md heading: the title/author overrides, falling back
// to BOOK_OVERVIEW.md's first heading for the title.
func header(tree, title, author string) related.Header {
	if title == "" {
		if t, err := naming.TitleFromFile(filepath.Join(tree, "BOOK_OVERVIEW.md")); err == nil {
			title = t
		}
	}
	return related.Header{Title: title, Author: author}
}

// readFile returns the file's contents, or "" when it does not exist.
func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}
