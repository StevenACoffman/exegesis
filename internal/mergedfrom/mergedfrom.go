// Package mergedfrom reads a merged skill's declaration of the skills it was merged
// from: `metadata.merged-from`, a space-separated list of source skill names.
//
//	metadata:
//	  merged-from: repository-port-placement repository-update-closure
//
// The declaration does two jobs, and one key does both so they cannot disagree. Its
// presence marks the skill as merged, which selects the merged test-prompts composition
// for that skill alone, so a plugin can hold merged and ordinary skills side by side.
// Its value names the sources, so a related-skill edge that still names a source after
// the source was removed can be presumed to mean the skill that subsumed it.
//
// **Under `metadata`, not top-level**, for the reason skillet's `metadata.lineage` is:
// speclint warns on any top-level key outside the agentskills.io set, and the spec's
// own rule is that everything else belongs under `metadata`. **A string, not a YAML
// list**, because the spec types every `metadata` value as a string and speclint
// rejects anything else; a skill name holds no spaces, so splitting on whitespace is
// unambiguous. The `## Provenance` section remains the place for each source's book,
// author and note; this is its machine-readable index, cheap enough to sit in the
// frontmatter every client already parses.
//
// It lives in exegesis rather than skillet for the reason internal/testcomp does: the
// merged composition is exegesis's policy, and the declaration earns promotion to
// skillet only on a consumer outside exegesis.
//
// Everything here is pure; callers do the file I/O.
package mergedfrom

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/StevenACoffman/skillet/related"
	"github.com/StevenACoffman/skillet/speclint"
)

// Key is the declaration's name inside `metadata`.
const Key = "merged-from"

// Path is the dotted path a diagnostic names, so a reader is sent to the nested key
// rather than to a top-level one that would be rejected.
const Path = "metadata." + Key

// Declaration is what a skill's frontmatter says about its merge.
type Declaration struct {
	// Declared reports whether `metadata.merged-from` is present at all, whatever its
	// value. Presence alone selects the merged composition: an author who wrote the key
	// meant the skill to be gated as merged, and a malformed value is reported by
	// Problems rather than silently demoting the skill to the standard mix.
	Declared bool
	// Sources are the names the value lists, in written order, empty when the value is
	// not a string or names nothing.
	Sources []string
}

// Parse reads the declaration from a raw frontmatter block. A block that does not parse
// declares nothing; speclint already reports the parse failure as itself.
func Parse(frontmatter string) Declaration {
	var fields map[string]any
	if err := yaml.Unmarshal([]byte(frontmatter), &fields); err != nil {
		return Declaration{}
	}
	var raw any
	var present bool
	switch m := fields["metadata"].(type) {
	case map[string]any:
		raw, present = m[Key]
	case map[any]any:
		raw, present = m[Key]
	}
	if !present {
		return Declaration{}
	}
	s, _ := raw.(string) // a non-string value is speclint's error to report, not ours
	d := Declaration{Declared: true}
	if sources := strings.Fields(s); len(sources) > 0 {
		d.Sources = sources
	}
	return d
}

// Problems reports what is wrong with d for the skill named slug: an empty list, a name
// that is not a valid skill name, a name listed twice, or the skill naming itself.
//
// Each one is an error rather than a warning because the value is read by tools, not
// people: a typo'd source redirects nothing, and the edge it should have explained keeps
// reading as dangling with no hint why.
func (d Declaration) Problems(slug string) []string {
	if !d.Declared {
		return nil
	}
	if len(d.Sources) == 0 {
		return []string{fmt.Sprintf(
			"%s names no source skills; list the skills this one was merged from, "+
				"separated by spaces", Path)}
	}
	var out []string
	seen := make(map[string]bool, len(d.Sources))
	for _, src := range d.Sources {
		switch {
		case !speclint.ValidSkillName(src):
			out = append(out, fmt.Sprintf("%s: %q is not a valid skill name", Path, src))
		case src == slug:
			out = append(out, fmt.Sprintf("%s: names the skill itself", Path))
		case seen[src]:
			out = append(out, fmt.Sprintf("%s: %q is listed twice", Path, src))
		}
		seen[src] = true
	}
	return out
}

// Subsumers maps each declared source to the merged skills that name it, each list
// sorted. It is built from every skill in a tree, keyed by the merged skill's slug.
//
// A source can map to more than one skill -- a partial merge splits a source across
// two merged skills -- and Resolve refuses to guess between them.
func Subsumers(decls map[string]Declaration) map[string][]string {
	out := make(map[string][]string)
	for merged, d := range decls {
		for _, src := range d.Sources {
			if src != merged && !slices.Contains(out[src], merged) {
				out[src] = append(out[src], merged)
			}
		}
	}
	for src := range out {
		sort.Strings(out[src])
	}
	return out
}

// Resolve returns the one skill that subsumed target, when exactly one merged skill
// declares it. It reports false for a target nothing declares, and for one two or more
// skills declare, where any single answer would be a guess.
func Resolve(subsumers map[string][]string, target string) (string, bool) {
	if s := subsumers[target]; len(s) == 1 {
		return s[0], true
	}
	return "", false
}

// Redirect returns nodes with every edge to an absent, subsumed skill pointed at the
// skill that subsumed it, so INDEX.md draws the edge it would otherwise drop.
//
// Only an edge whose target is not in the tree is touched: a source still present is
// still a skill, and an edge to it means it. An edge that would point a skill at itself
// is dropped -- a merged skill naming its own source is the merge, not a relation -- and
// one that would duplicate an edge the skill already has is dropped too. The input is
// not mutated.
func Redirect(nodes []related.Node, subsumers map[string][]string) []related.Node {
	known := make(map[string]bool, len(nodes))
	for i := range nodes {
		known[nodes[i].Slug] = true
	}
	out := make([]related.Node, len(nodes))
	for i := range nodes {
		out[i] = nodes[i]
		out[i].Edges = redirectEdges(nodes[i].Slug, nodes[i].Edges, known, subsumers)
	}
	return out
}

// redirectEdges returns one skill's edges with each absent, subsumed target replaced by
// the skill that subsumed it, dropping the edges Redirect describes.
func redirectEdges(
	slug string,
	edges []related.Edge,
	known map[string]bool,
	subsumers map[string][]string,
) []related.Edge {
	out := make([]related.Edge, 0, len(edges))
	have := make(map[related.Edge]bool, len(edges))
	for _, e := range edges {
		if to, ok := Resolve(subsumers, e.Target); ok && !known[e.Target] {
			if to == slug {
				continue
			}
			e.Target = to
		}
		key := related.Edge{Kind: e.Kind, Target: e.Target}
		if !have[key] {
			have[key] = true
			out = append(out, e)
		}
	}
	return out
}
