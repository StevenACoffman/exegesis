// Package mergeindexgen regenerates a merged tree's INDEX.md — the cross-book provenance
// index that was hand-maintained until now. It reads each merged skill's `## Provenance`
// body section (written by internal/mergemigrate), the optional source-verification
// headers, and the rejected-pair files, so every verdict is written once instead of kept
// by hand. Distinct from indexgen, which builds a book tree's skill list and relationship
// graph and knows nothing about merge provenance. Rendering is deterministic (no date
// stamp), so `merge-index --check` can tell stale from current.
package mergeindexgen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/StevenACoffman/exegesis/internal/mergemigrate"
	"github.com/StevenACoffman/skillet/frontmatter"
	"github.com/StevenACoffman/skillet/skill"
)

const fileName = "INDEX.md"

// entry is one merged skill's provenance row.
type entry struct {
	slug string
	prov mergemigrate.Provenance
}

// verification is one source-verification header (source-verification/<pair>-{r,a1}.md).
type verification struct {
	Pair    string `yaml:"pair"`
	Check   string `yaml:"check"`
	Sources []struct {
		Book   string `yaml:"book"`
		Skill  string `yaml:"skill"`
		Status string `yaml:"status"`
	} `yaml:"sources"`
}

// Path returns tree's INDEX.md path.
func Path(tree string) string {
	return filepath.Join(tree, fileName)
}

// Generate returns the regenerated INDEX.md for a merged tree: the header counts, the
// cross-book provenance table (with fan-in sources marked), the source-verification
// summary, and the rejected pairs. Sections whose files are not on disk render an honest
// empty state rather than a blank table.
func Generate(tree string) (string, error) {
	entries, err := collect(tree)
	if err != nil {
		return "", err
	}
	fanIn := fanInSlugs(entries)
	var b strings.Builder
	b.WriteString("# Merged Skills — Cross-Book Provenance Index\n\n")
	fmt.Fprintf(&b, "- **Total merged skills:** %d\n", len(entries))
	rejected := readRejected(tree)
	fmt.Fprintf(&b, "- **Rejected pairs:** %d\n", len(rejected))
	fmt.Fprintf(&b, "- **Fan-in sources** (feed ≥ 2 merged skills): %d\n", len(fanIn))
	b.WriteString("\n")
	writeProvenanceTable(&b, entries, fanIn)
	writeVerification(&b, readVerifications(tree))
	writeRejected(&b, rejected)
	return b.String(), nil
}

// collect discovers every merged skill under tree and parses its Provenance block. A
// skill whose SKILL.md cannot be loaded is an error, not a gap: dropping it would hide a
// merged skill from the index. A skill with no Provenance block is kept with empty
// sources so it is visibly missing one rather than silently absent.
func collect(tree string) ([]entry, error) {
	dirs, err := skill.Discover(tree)
	if err != nil {
		return nil, fmt.Errorf("discover skills: %w", err)
	}
	entries := make([]entry, 0, len(dirs))
	for _, dir := range dirs {
		s, loadErr := skill.Load(dir)
		if loadErr != nil {
			return nil, fmt.Errorf("load %s: %w", dir, loadErr)
		}
		prov, _ := parseProvenance(s.Body)
		entries = append(entries, entry{slug: filepath.Base(dir), prov: prov})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].slug < entries[j].slug })
	return entries, nil
}

// parseProvenance extracts the fenced yaml from a body's `## Provenance` section. The raw
// body is scanned rather than markdown.Parse'd because markdown blanks code fences, and
// the yaml block is exactly what mergemigrate.Render writes, so mergemigrate's own type
// reads it back. ok is false when the section or its block is absent.
func parseProvenance(body string) (mergemigrate.Provenance, bool) {
	lines := strings.Split(body, "\n")
	i := 0
	for ; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == mergemigrate.Heading {
			break
		}
	}
	for i++; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "## ") {
			return mergemigrate.Provenance{}, false // next section, no block
		}
		if strings.HasPrefix(trimmed, "```yaml") {
			break
		}
	}
	var buf []string
	for i++; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			var p mergemigrate.Provenance
			if err := yaml.Unmarshal([]byte(strings.Join(buf, "\n")), &p); err != nil {
				return mergemigrate.Provenance{}, false
			}
			return p, true
		}
		buf = append(buf, lines[i])
	}
	return mergemigrate.Provenance{}, false
}

// fanInSlugs returns the source slugs that feed two or more merged skills.
func fanInSlugs(entries []entry) map[string]bool {
	count := map[string]int{}
	for i := range entries {
		for _, s := range entries[i].prov.Sources {
			count[s.Slug]++
		}
	}
	fanIn := map[string]bool{}
	for slug, n := range count {
		if n >= 2 {
			fanIn[slug] = true
		}
	}
	return fanIn
}

func writeProvenanceTable(b *strings.Builder, entries []entry, fanIn map[string]bool) {
	b.WriteString("## Cross-Book Provenance Table\n\n| Merged Skill | Sources |\n| --- | --- |\n")
	for i := range entries {
		srcs := make([]string, 0, len(entries[i].prov.Sources))
		for _, s := range entries[i].prov.Sources {
			cell := "`" + s.Slug + "`"
			if fanIn[s.Slug] {
				cell += " ★"
			}
			srcs = append(srcs, cell)
		}
		fmt.Fprintf(b, "| [%s](%s/SKILL.md) | %s |\n", entries[i].slug, entries[i].slug,
			strings.Join(srcs, ", "))
	}
	b.WriteString("\n")
}

// readVerifications reads the source-verification/*.md headers, sorted by file name. An
// absent directory is not an error: no merge run has written them yet.
func readVerifications(tree string) []verification {
	dir := filepath.Join(tree, "source-verification")
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil
	}
	sort.Strings(files)
	var out []verification
	for _, f := range files {
		raw, readErr := os.ReadFile(f)
		if readErr != nil {
			continue
		}
		block, _ := frontmatter.Split(string(raw))
		var v verification
		if yaml.Unmarshal([]byte(block), &v) == nil && v.Pair != "" {
			out = append(out, v)
		}
	}
	return out
}

func writeVerification(b *strings.Builder, vs []verification) {
	b.WriteString("## Source Verification Summary\n\n")
	if len(vs) == 0 {
		b.WriteString("_No source-verification records on disk yet._\n\n")
		return
	}
	b.WriteString("| Pair | Check | Source | Status |\n| --- | --- | --- | --- |\n")
	for _, v := range vs {
		for _, s := range v.Sources {
			fmt.Fprintf(b, "| %s | %s | %s/%s | %s |\n", v.Pair, v.Check, s.Book, s.Skill, s.Status)
		}
	}
	b.WriteString("\n")
}

// readRejected returns the rejected-pair files as (file, label) pairs, sorted by file
// name; label is the file's first `# ` heading.
func readRejected(tree string) [][2]string {
	files, err := filepath.Glob(filepath.Join(tree, "rejected", "pair-*.md"))
	if err != nil {
		return nil
	}
	sort.Strings(files)
	out := make([][2]string, 0, len(files))
	for _, f := range files {
		out = append(out, [2]string{filepath.Join("rejected", filepath.Base(f)), rejectedLabel(f)})
	}
	return out
}

// rejectedLabel returns the file's first `# ` heading text, or "" when it has none.
func rejectedLabel(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "# "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func writeRejected(b *strings.Builder, rejected [][2]string) {
	b.WriteString("## Rejected Pairs\n\n")
	if len(rejected) == 0 {
		b.WriteString("_None._\n")
		return
	}
	for _, r := range rejected {
		name := strings.TrimSuffix(filepath.Base(r[0]), ".md")
		if r[1] == "" {
			fmt.Fprintf(b, "- [%s](%s)\n", name, r[0])
			continue
		}
		fmt.Fprintf(b, "- [%s](%s) — %s\n", name, r[0], r[1])
	}
}
