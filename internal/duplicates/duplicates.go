// Package duplicates finds a skill that exists in more than one tree, and reports
// whether the copies still agree.
//
// One skill kept in several places is the normal state of an installed corpus: a market
// repository is the source, and each runtime gets its own directory under a different
// dot-folder. The copies are made by copying, so nothing keeps them in step, and the
// drift is invisible -- every tree passes its own gates while answering a different
// question from the tree beside it.
//
// Duplication alone is therefore not a defect and this package does not call it one. The
// finding is *divergence*: two copies of one slug whose content hashes differ, which
// means at least one of them is stale and a reader cannot tell which.
//
// Scan reads trees; everything else here is pure.
package duplicates

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/StevenACoffman/skillet/skill"
)

// Copy is one skill as found in one tree.
type Copy struct {
	Tree string // the tree it was found in, as the caller named it
	Slug string // directory name
	Hash string // identity.Hash of SKILL.md, via skill.Hash
}

// Finding is one skill present in more than one tree.
type Finding struct {
	Slug   string
	Copies []Copy // sorted by tree
}

// Diverged reports whether the copies differ in content rather than merely existing in
// several places.
//
// It is a method over the hashes rather than a field set when the Finding is built: a
// stored bool would be a second record of what Copies already says, and the two can
// disagree after any edit to either. Same reason mergestatus.Entry.SupersededBy derives
// its edge instead of storing one.
//
// Ensures: false for a Finding whose copies all share one hash; it is pure.
func (f Finding) Diverged() bool {
	for _, c := range f.Copies {
		if c.Hash != f.Copies[0].Hash {
			return true
		}
	}
	return false
}

// Find returns the skills present in more than one tree, sorted by slug.
//
// A slug found twice in the *same* tree cannot happen -- a slug is a directory basename
// and one directory holds one -- so grouping by slug alone is enough, and the count of
// copies is the count of trees.
//
// Ensures: every Finding holds at least two copies, sorted by tree; the result is sorted
//
//	by slug; it is pure and does not mutate copies.
func Find(copies []Copy) []Finding {
	bySlug := make(map[string][]Copy)
	for _, c := range copies {
		bySlug[c.Slug] = append(bySlug[c.Slug], c)
	}
	out := make([]Finding, 0, len(bySlug))
	for slug, group := range bySlug {
		if len(group) < 2 {
			continue
		}
		sort.Slice(group, func(i, j int) bool { return group[i].Tree < group[j].Tree })
		out = append(out, Finding{Slug: slug, Copies: group})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// Scan reads every tree and returns one Copy per skill found in it.
//
// Trees holding no skills come back in empty rather than as an error, for the reason
// provenance.Collect gives: a path that missed a nesting level and a tree with nothing
// in it look identical from here. Reporting the fact matters more than usual for this
// command -- a tree silently contributing nothing cannot duplicate anything, so the
// answer would be "no duplicates" when the truth is "I did not look".
//
// Ensures: every Copy names a skill that was read; empty holds one entry per tree that
//
//	yielded none, in the order given.
func Scan(trees []string) (copies []Copy, empty []string, err error) {
	for _, tree := range trees {
		dirs, discoverErr := skill.Discover(tree)
		if discoverErr != nil {
			return nil, nil, fmt.Errorf("duplicates: discover %s: %w", tree, discoverErr)
		}
		if len(dirs) == 0 {
			empty = append(empty, tree)
			continue
		}
		for _, dir := range dirs {
			s, loadErr := skill.Load(dir)
			if loadErr != nil {
				return nil, nil, fmt.Errorf("duplicates: load %s: %w", dir, loadErr)
			}
			// skill.Hash is identity.Hash over the whole SKILL.md, the same value verify
			// writes into skills-manifest.json -- so a hash compared here and a hash
			// recorded there mean the same thing by construction, not by coincidence.
			copies = append(copies, Copy{
				Tree: tree,
				Slug: filepath.Base(dir),
				Hash: s.Hash(),
			})
		}
	}
	return copies, empty, nil
}
