package related

import "strings"

// Resolution states for looking a display title up in a tree.
const (
	// TitleUnknown is the zero value: no skill in the tree carries this heading.
	//
	// Zero on purpose. A caller that ignores the resolution must not be handed a slug
	// -- an empty target written into an edge is worse than an unresolved bullet,
	// because the bullet stays visibly broken and the edge does not.
	TitleUnknown Resolution = iota
	// TitleResolved means exactly one skill carries the heading.
	TitleResolved
	// TitleAmbiguous means more than one does, so no slug is returned. Refusing is the
	// point: picking either would be a coin flip recorded as fact.
	TitleAmbiguous
)

// Resolution is the outcome of a title lookup.
type Resolution int

// TitleRef is one bullet that names a skill by display title rather than by slug, with
// what that title resolves to in the tree.
type TitleRef struct {
	From       string // the skill whose section holds the bullet
	Title      string // the display title as written
	Slug       string // the resolved slug, empty unless Resolution is TitleResolved
	Resolution Resolution
}

// Titles maps a skill's H1 heading to the slugs carrying it.
//
// A slice rather than a single slug so a collision is detectable rather than
// last-write-wins. Measured on a 233-skill corpus there are 232 distinct headings, so
// at least one collision exists already and a map to one slug would silently pick.
type Titles map[string][]string

// NewTitles indexes a tree's skills by their H1 heading.
//
// **The H1, not the slug-derived Title.** Measured over 97 bullets naming a skill by
// display name: the H1 index resolves 53, a slug-derived index resolves 38, and both
// together resolve 54. The second index buys one bullet and doubles the surface on which
// two different strings can claim the same skill, which is the wrong trade when a wrong
// match is the failure being designed against.
func NewTitles(nodes []Node) Titles {
	out := make(Titles, len(nodes))
	for i := range nodes {
		if h := strings.TrimSpace(nodes[i].Heading); h != "" {
			out[h] = append(out[h], nodes[i].Slug)
		}
	}
	return out
}

// Resolve looks a display title up, exactly.
//
// **Exact match only: no slugification, no fuzzy distance, no prefix.** The asymmetry
// that decides this is that a wrong match is invisible and a missed one is not. An
// unresolved bullet stays visibly unreadable until someone fixes it; a bullet resolved to
// the wrong slug becomes a real edge that `index`, `verify` and any downstream gate count
// as authored. Slugifying "Competing Consumers vs. Dispatcher" produces something
// plausible, and plausible is exactly what makes the failure survive review.
func (t Titles) Resolve(title string) (string, Resolution) {
	switch slugs := t[strings.TrimSpace(title)]; len(slugs) {
	case 0:
		return "", TitleUnknown
	case 1:
		return slugs[0], TitleResolved
	default:
		return "", TitleAmbiguous
	}
}

// String names the resolution for a report.
func (r Resolution) String() string {
	switch r {
	case TitleResolved:
		return "resolved"
	case TitleAmbiguous:
		return "ambiguous"
	case TitleUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// Heading returns a body's H1 as written, or "" when it has none.
//
// Fenced regions are skipped, so a "# " line inside a shell transcript is not mistaken
// for the document's heading -- the same rule ParseSection follows, and for the same
// reason: a parser that reads inside fences finds text nobody wrote as markup.
func Heading(body string) string {
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if isFence(trimmed) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if after, ok := strings.CutPrefix(trimmed, "# "); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

// TitleRefs reports every bullet in nodes' sections that names a skill by display title.
//
// **It reports and does not rewrite, and that is a finding rather than caution.**
// Measured: resolving the title is *necessary but not sufficient*. A bullet reading
// "- **Lock-In Cost Optimization** — *depends-on* → why" is unreadable because of the
// italic kind and the arrow, not only the title -- substituting the correct slug leaves
// it just as unparseable, which was confirmed by feeding Normalize a bullet whose bold
// token was already a valid slug and watching it pass through untouched.
//
// So this cannot fix a bullet until the dialect tolerance for italic kinds and arrow
// separators lands in the parser. What it can do today is say which titles *would*
// resolve, which is the input to deciding whether that work is worth it.
//
// Pure: the caller collects the nodes.
func TitleRefs(nodes []Node) []TitleRef {
	titles := NewTitles(nodes)
	slugs := make(map[string]bool, len(nodes))
	for i := range nodes {
		slugs[nodes[i].Slug] = true
	}
	out := make([]TitleRef, 0)
	for i := range nodes {
		for _, tok := range boldLeads(nodes[i].Body) {
			if slugs[tok] || strings.Contains(tok, "_") {
				continue // already a slug, or a kind in the bold position
			}
			slug, res := resolveToken(titles, slugs, tok)
			out = append(out, TitleRef{
				From: nodes[i].Slug, Title: tok, Slug: slug, Resolution: res,
			})
		}
	}
	return out
}

// resolveToken resolves one display title, falling back to a slug the author wrote
// inside it.
//
// The fallback is checked second so an author's parenthetical can never override an
// exact heading match, and it requires the slug to name a skill in this tree: a
// cross-tree reference has no answer here, and inventing one would be the wrong-match
// failure by another route.
func resolveToken(titles Titles, slugs map[string]bool, tok string) (string, Resolution) {
	if slug, res := titles.Resolve(tok); res != TitleUnknown {
		return slug, res
	}
	if inline, ok := inlineSlug(tok); ok && slugs[inline] {
		return inline, TitleResolved
	}
	return "", TitleUnknown
}

// boldLeads returns the bold token opening each non-canonical bullet in a body's related
// section -- the position a display title occupies in the legacy dialects.
func boldLeads(body string) []string {
	lines := strings.Split(body, "\n")
	var out []string
	for _, sec := range findSections(lines) {
		for _, b := range sectionBullets(lines, sec.head, sec.end) {
			t := strings.TrimSpace(b.text)
			rest, ok := strings.CutPrefix(t, "- **")
			if !ok {
				continue
			}
			if end := strings.Index(rest, "**"); end > 0 {
				out = append(out, rest[:end])
			}
		}
	}
	return out
}

// inlineSlug returns a backticked token inside a display title, when there is exactly
// one. More than one is not a slug reference, it is prose that happens to use code
// spans, and picking either would be the guess this package refuses to make.
func inlineSlug(title string) (string, bool) {
	parts := strings.Split(title, "`")
	if len(parts) != 3 {
		return "", false
	}
	tok := strings.TrimSpace(parts[1])
	return tok, tok != ""
}
