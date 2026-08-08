// Package mergemigrate moves a merged skill's provenance out of frontmatter and into
// the body, which is where the spec that produced these skills says it belongs.
//
// The tree at books/merged/all-books-v1 carries `id`, `title`, `type`, `source_skills`
// and `related_skills` as frontmatter keys, and no `name` at all, so every skill in it
// fails `exegesis lint` on five disallowed keys and a name/folder mismatch. merge-skills'
// own methodology and template both say not to emit those keys and to capture the sources
// in a body `## Provenance` section; the tree predates that and contradicts it.
//
// The move is lossless, which was measured before it was written rather than assumed:
// `id` equals the folder in 27 of 27 skills, and every `supersedes` relation names
// exactly the skills already listed in `source_skills`, so both can be dropped rather
// than restated. `title` is the exception — 10 of 27 bodies have no `# ` heading at all,
// so for those the frontmatter title is the only human title in the file and becomes the
// heading instead of being deleted.
//
// Everything here is pure. The command reads and writes the files.
package mergemigrate

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/StevenACoffman/exegesis/internal/related"
	"github.com/StevenACoffman/skillet/frontmatter"
)

// Heading is the body section the provenance moves into, in the form this package
// writes. Title case for the reason every other heading here is: `rumdl` MD063 rewrites
// a lowercase one, and a tool and a formatter fighting over the same line is a defect
// that outlives both.
const Heading = "## Provenance"

// Source is one skill a merged skill was built from.
type Source struct {
	Slug   string `yaml:"slug"`
	Book   string `yaml:"book,omitempty"`
	Author string `yaml:"author,omitempty"`
	Note   string `yaml:"note,omitempty"`
}

// Provenance is the composition a merged skill records: what it is, and what it was
// merged from. It is rendered into the body as prose plus a fenced `yaml` block — the
// shape `## Merge Status` already uses, and for the same reason. `metadata` is the
// spec's home for client-specific frontmatter, but it holds string values only, and a
// composition is a list of records.
type Provenance struct {
	Type    string   `yaml:"type"`
	Sources []Source `yaml:"source_skills"`
}

// header is the frontmatter this migration reads. Only the keys it moves are declared:
// everything else is copied through as text and never round-tripped through YAML.
type header struct {
	ID      string   `yaml:"id"`
	Title   string   `yaml:"title"`
	Type    string   `yaml:"type"`
	Sources []Source `yaml:"source_skills"`
	Related []struct {
		Slug     string `yaml:"slug"`
		Relation string `yaml:"relation"`
		Note     string `yaml:"note"`
	} `yaml:"related_skills"`
}

// movedKey reports whether a frontmatter key is one this migration removes, because
// its content moves into the body.
//
// The set is exactly what merge-skills names as "unknown fields": everything else in a
// merged skill's frontmatter — `description`, `tags`, and any spec key a skill later
// grows — is copied through untouched, line for line, so a description's own wrapping
// and quoting survive a migration that has no business reformatting it.
func movedKey(k string) bool {
	switch k {
	case "id", "title", "type", "source_skills", "related_skills":
		return true
	default:
		return false
	}
}

// Migrate returns raw rewritten into the decided provenance model, and whether it
// changed. A skill that carries none of the moved keys is returned untouched, so the
// migration is idempotent and safe to run over a tree that is already migrated.
//
// Requires: folder is the skill's directory name.
// Ensures:  the output frontmatter holds no moved key and a `name` equal to folder;
//
//	every source in `source_skills` appears in the body; running Migrate on the
//	output reports no change; it is pure.
func Migrate(raw, folder string) (string, bool, error) {
	block, body := frontmatter.Split(raw)
	if block == "" {
		return raw, false, nil
	}
	var h header
	if err := yaml.Unmarshal([]byte(block), &h); err != nil {
		return raw, false, fmt.Errorf("frontmatter: %w", err)
	}
	kept := keepKeys(block)
	if len(kept) == len(strings.Split(strings.TrimRight(block, "\n"), "\n")) && h.Title == "" {
		return raw, false, nil // nothing to move
	}
	out := "---\n" + strings.Join(append([]string{"name: " + folder}, kept...), "\n") + "\n---\n" +
		migrateBody(body, &h)
	return out, out != raw, nil
}

// Render returns the `## Provenance` section for p: the prose bullets a reader wants
// and the fenced `yaml` block a generator reads, one following the other so neither
// audience is served by parsing the other's form.
func Render(p Provenance) string {
	var b strings.Builder
	b.WriteString(Heading + "\n\n- **Type:** " + typeLabel(p.Type) + "\n- **Merged from:**\n")
	for _, s := range p.Sources {
		fmt.Fprintf(&b, "  - `%s`%s\n", s.Slug, attribution(s))
	}
	block, err := yaml.Marshal(p)
	if err != nil {
		// Marshalling a struct of strings cannot fail; if it somehow does, the prose
		// above still carries every fact, so the section is written without the block
		// rather than the whole migration failing.
		return b.String()
	}
	b.WriteString("\n```yaml\n" + string(block) + "```\n")
	return b.String()
}

// keepKeys returns the frontmatter lines that survive: every line except those
// belonging to a moved key. A key's value continues until the next top-level key, so
// nested list items and mappings travel with the key that owns them.
func keepKeys(block string) []string {
	var out []string
	moved := false
	for _, line := range strings.Split(strings.TrimRight(block, "\n"), "\n") {
		if key, isTop := topLevelKey(line); isTop {
			moved = movedKey(key)
			if key == "name" {
				continue // re-emitted from the folder, so never duplicated
			}
		}
		if !moved {
			out = append(out, line)
		}
	}
	return out
}

// topLevelKey returns the key a line declares, when the line is a top-level mapping
// entry rather than part of a previous key's value.
func topLevelKey(line string) (string, bool) {
	if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '-' || line[0] == '#' {
		return "", false
	}
	key, _, found := strings.Cut(line, ":")
	if !found {
		return "", false
	}
	return strings.TrimSpace(key), true
}

// migrateBody returns the body with the title restored as a heading when it has none,
// the surviving relation recorded as a related-skill edge, and the Provenance section
// inserted.
func migrateBody(body string, h *header) string {
	out := withTitle(body, h.Title)
	for _, r := range h.Related {
		// `supersedes` is dropped, not migrated: it names exactly the skills already
		// listed in source_skills — measured, in all 27 skills — so writing it again
		// would be one fact in two places, and the Provenance block is the one that
		// also carries the book and the author.
		kind := related.Kind(r.Relation)
		if !kind.Valid() {
			continue
		}
		out, _ = related.Upsert(out, related.Edge{Kind: kind, Target: r.Slug, Rationale: r.Note})
	}
	return insertProvenance(out, Provenance{Type: h.Type, Sources: withNotes(h)})
}

// withNotes returns the sources with each one's supersession note attached.
//
// The note is the only thing a `supersedes` relation carries that `source_skills` does
// not — it says what the merged skill adds that its source lacked — and it is prose
// somebody wrote. Dropping the relation without first taking the note off it loses 55
// sentences across the tree, which a word-multiset comparison against the pre-migration
// files caught and which nothing else would have.
//
// The join is by slug, which is sound because every `supersedes` relation names a skill
// already in `source_skills`, in all 27 skills. A relation that names something else is
// left alone here; migrateBody records it as an edge if its kind is one exegesis knows.
func withNotes(h *header) []Source {
	notes := make(map[string]string, len(h.Related))
	for _, r := range h.Related {
		if r.Relation == "supersedes" && r.Note != "" {
			notes[r.Slug] = r.Note
		}
	}
	out := make([]Source, 0, len(h.Sources))
	for _, src := range h.Sources {
		if src.Note == "" {
			src.Note = notes[src.Slug]
		}
		out = append(out, src)
	}
	return out
}

// withTitle returns body with title as its `# ` heading when it has none.
//
// This is the only key whose removal would have lost content: 10 of the 27 merged
// skills have no heading in the body at all, so their frontmatter title is the file's
// only human-readable name.
func withTitle(body, title string) string {
	if title == "" || hasTitle(body) {
		return body
	}
	return "\n# " + title + "\n" + strings.TrimLeft(body, "\n")
}

// hasTitle reports whether body already opens a `# ` heading outside a code fence.
func hasTitle(body string) bool {
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"), strings.HasPrefix(trimmed, "~~~"):
			inFence = !inFence
		case !inFence && strings.HasPrefix(trimmed, "# "):
			return true
		}
	}
	return false
}

// insertProvenance returns body with the Provenance section placed before the audit
// section when there is one, and at the end otherwise — the order merge-skills'
// template uses. An existing Provenance section is replaced, which is what makes a
// second migration a no-op rather than a second section.
func insertProvenance(body string, p Provenance) string {
	if len(p.Sources) == 0 && p.Type == "" {
		return body
	}
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	at := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.EqualFold(trimmed, Heading) || strings.HasPrefix(trimmed, "## Audit") {
			at = i
			break
		}
	}
	head := strings.TrimRight(strings.Join(lines[:at], "\n"), "\n")
	tail := strings.Join(lines[dropSection(lines, at):], "\n")
	out := head + "\n\n" + Render(p)
	if tail != "" {
		out += "\n" + tail
	}
	return strings.TrimRight(out, "\n") + "\n"
}

// dropSection returns the index one past a Provenance section starting at at, or at
// itself when that is not what starts there. Replacing the section rather than
// appending beside it is what keeps a second run from writing a second copy.
func dropSection(lines []string, at int) int {
	if at >= len(lines) || !strings.EqualFold(strings.TrimSpace(lines[at]), Heading) {
		return at
	}
	for i := at + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			return i
		}
	}
	return len(lines)
}

// typeLabel renders the merged-skill type for a human, defaulting rather than leaving
// the bullet empty on a skill whose frontmatter omitted it.
func typeLabel(t string) string {
	if t == "" {
		return "merged skill"
	}
	return strings.ReplaceAll(t, "-", " ")
}

// attribution renders a source's book and author, omitting what the file did not say.
func attribution(s Source) string {
	switch {
	case s.Book != "" && s.Author != "":
		return fmt.Sprintf(" — *%s* by %s", s.Book, s.Author)
	case s.Book != "":
		return fmt.Sprintf(" — *%s*", s.Book)
	case s.Author != "":
		return " — by " + s.Author
	default:
		return ""
	}
}
