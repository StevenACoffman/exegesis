// Package provenance answers which source skills a merged skill replaced, by reading
// the supersession each source skill declares about itself.
//
// A merge run records its result on the *source* side: each skill that was folded into
// a merged one gains a `superseded-by` edge naming it. Nothing on the merged side names
// its sources, so the map a caller wants -- merged skill to the skills it replaced -- is
// the inverse of what is written down, and building it is this package's whole job.
//
// The alternative would be a provenance field in skills-manifest.json. It is not worth
// having: the edges already are the audit trail, a derived copy can disagree with them,
// and a second record of one fact is the information leakage this repo keeps finding.
//
// Reading trees is Collect's; everything else here is pure.
package provenance

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/StevenACoffman/skillet/related"
	"github.com/StevenACoffman/skillet/skill"
)

// relationSupersededBy is the `relation:` value marking a frontmatter entry as a
// supersession. It is the same vocabulary word related.SupersededBy spells for the
// bullet form, and the two are asserted equal by a test rather than kept in step by
// hand.
const relationSupersededBy = "superseded-by"

// Source is one source skill that declared itself superseded.
type Source struct {
	Slug string // directory name, for reporting
	Dir  string // path, so a caller can read the skill's own text
}

// Index answers, for one merged skill, which source skills declare it replaced them.
//
// It is keyed by the merged skill's **slug alone**, not by the qualified target the
// bullet writes. A bullet says `merged/all-books-v1/x` while the tree being verified sits
// on disk at some path ending in that run's directory, and the two spellings need not
// agree -- requiring them to would make the answer depend on where the caller ran from.
// The last segment is the slug, which is what filepath.Base of a skill directory yields.
//
// The cost is that two merge runs producing the same slug would collide. That cannot
// happen inside the one tree a verify run reads, which is the only place this is used.
type Index map[string][]Source

// relatedSkillsFrontmatter is the slice of the frontmatter this package reads. Every
// other key is ignored, so an unrelated frontmatter change cannot break provenance.
//
// The list is []any rather than a slice of structs because **the corpus mixes two item
// shapes in one list**: a bare slug (`- golden-files-update-flag`) beside a mapping
// (`- slug: … relation: …`). Decoding into a struct slice fails on the first scalar and
// yields nothing for the whole block -- which is the fail-open direction, since a merged
// skill with no sources has every signal scored as new. Measured: that alone hid
// hashimoto/table-driven-named-cases, one of the 27.
type relatedSkillsFrontmatter struct {
	RelatedSkills []any `yaml:"related_skills"`
}

// Targets returns the merged skills that one source skill declares superseded it.
//
// A supersession is written two ways in this corpus and both count: a
// `- superseded-by: <target>` bullet in the `## Related skills` section, and a
// `related_skills:` frontmatter entry carrying `relation: superseded-by`.
//
// Reading only one spelling is not a smaller version of this function, it is a broken
// one. Measured over books/: 8 merged skills are named only by a bullet, 8 only by
// frontmatter, 11 by both. A bullet-only reader resolves 19 of 27 and reports *no
// sources* for the other 8 -- and a merged skill with no sources has every signal
// counted as new, which is the direction that passes. The gate would fail open on a
// third of the tree.
//
// Ensures: the result is sorted, holds no duplicates, and every element is a qualified
//
//	target; it is pure.
func Targets(frontmatter, body string) []string {
	seen := make(map[string]bool)
	for _, e := range related.ParseSection(body) {
		if e.Kind == related.SupersededBy {
			seen[e.Target] = true
		}
	}
	for _, t := range frontmatterTargets(frontmatter) {
		seen[t] = true
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		// Unqualified targets are dropped rather than resolved: a bare slug names a
		// skill in the source's own tree, which is a sibling relationship and not a
		// merge. related.Qualified is the same test the wire format uses.
		if related.Qualified(t) {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}

// Collect walks every tree and returns which source skills each merged skill replaced.
//
// Trees that hold no skills are returned in empty rather than raised as an error.
// skill.Discover reads immediate subdirectories only, so naming a book that nests its
// skills one level down (books/effective-go-recipes/**skills**/…) discovers nothing --
// and so does naming a book with none built yet, which books/fcis-go really is. Those
// two look identical from here and only the caller knows which it meant, so this reports
// the fact and declines to judge it. Saying nothing is the one option ruled out: it
// would resolve fewer sources than exist and every unresolved merged skill scores as
// sharper than it is.
//
// Requires: each tree is a directory whose immediate subdirectories are skills.
// Ensures:  every Source in idx names a directory that was read; a merged skill named by
//
//	no source is absent from idx rather than present and empty, so a caller can
//	tell "replaced nothing we can see" from "replaced nothing"; empty holds one
//	entry per tree that yielded no skills, in the order given.
func Collect(trees []string) (idx Index, empty []string, err error) {
	idx = make(Index)
	for _, tree := range trees {
		dirs, discoverErr := skill.Discover(tree)
		if discoverErr != nil {
			return nil, nil, fmt.Errorf("provenance: discover %s: %w", tree, discoverErr)
		}
		if len(dirs) == 0 {
			empty = append(empty, tree)
			continue
		}
		for _, dir := range dirs {
			s, loadErr := skill.Load(dir)
			if loadErr != nil {
				return nil, nil, fmt.Errorf("provenance: load %s: %w", dir, loadErr)
			}
			idx.add(s)
		}
	}
	idx.sortSources()
	return idx, empty, nil
}

// add records every supersession s declares, keyed by the merged skill's slug.
func (x Index) add(s *skill.Skill) {
	src := Source{Slug: path.Base(s.Dir), Dir: s.Dir}
	for _, target := range Targets(s.Frontmatter, s.Body) {
		slug := path.Base(target)
		// A skill declaring the same supersession in both spellings declares one
		// relationship, not two -- the (kind, target) dedup ParseSection already
		// applies within a document, extended across the two places it can be written.
		if !slices.Contains(x[slug], src) {
			x[slug] = append(x[slug], src)
		}
	}
}

// sortSources orders each entry by slug so a report reads the same on every run.
func (x Index) sortSources() {
	for slug := range x {
		sources := x[slug]
		sort.Slice(sources, func(i, j int) bool { return sources[i].Slug < sources[j].Slug })
	}
}

// frontmatterTargets returns the superseded-by targets in a `related_skills:` block.
//
// A block that does not parse yields no targets rather than an error, for the reason
// skill.FrontmatterErr exists: one malformed source skill must not stop a walk over
// every other skill in the tree.
func frontmatterTargets(frontmatter string) []string {
	if !strings.Contains(frontmatter, "related_skills:") {
		// Not an optimization -- most skills have no such block, and unmarshalling a
		// frontmatter that cannot contain the key would report parse failures this
		// function has no opinion about.
		return nil
	}
	var fm relatedSkillsFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return nil
	}
	out := make([]string, 0, len(fm.RelatedSkills))
	for _, item := range fm.RelatedSkills {
		if slug, ok := supersededSlug(item); ok {
			out = append(out, slug)
		}
	}
	return out
}

// supersededSlug returns the slug of one `related_skills:` item when that item declares
// a supersession.
//
// A bare-scalar item states a relationship with no kind, which is not a supersession --
// so it is skipped rather than treated as one. Anything that is not a mapping is skipped
// for the same reason: this reads the shape it understands and stays silent about the
// rest, because the alternative is inventing an edge nobody wrote.
func supersededSlug(item any) (string, bool) {
	fields, ok := item.(map[string]any)
	if !ok {
		return "", false
	}
	relation, _ := fields["relation"].(string)
	if strings.TrimSpace(relation) != relationSupersededBy {
		return "", false
	}
	slug, _ := fields["slug"].(string)
	slug = strings.TrimSpace(slug)
	return slug, slug != ""
}
