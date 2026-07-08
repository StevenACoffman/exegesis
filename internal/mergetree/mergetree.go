// Package mergetree assembles the cross-book merge-index model from a merged
// tree and its source books, reading the append-only merge-status ledgers on the
// source skills. It is the shared book-tree reader used by both the merge-index
// and verify commands, so neither command imports the other.
package mergetree

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/mergedoc"
	"github.com/StevenACoffman/exegesis/internal/store"
)

// Assemble builds the merge-index model for a merged tree from the source books'
// merge-status ledgers. The run slug is the merged tree's directory name; only
// ledger entries for that run with a merged/partial `into` count as parents.
func Assemble(tree string, sourceDirs []string) (*book2skill.MergeIndex, error) {
	runSlug := filepath.Base(filepath.Clean(tree))
	merged, err := store.GatherSkills(tree)
	if err != nil {
		return nil, fmt.Errorf("mergetree: %w", err)
	}
	parents := make(map[string][]book2skill.MergeParent)
	model := &book2skill.MergeIndex{RunSlug: runSlug}
	for _, srcDir := range sourceDirs {
		book, err := readSourceBook(srcDir, runSlug, parents)
		if err != nil {
			return nil, err
		}
		model.Sources = append(model.Sources, book)
	}
	for i := range merged {
		model.Merges = append(model.Merges, book2skill.MergeRecord{
			Slug:    merged[i].Slug,
			Title:   merged[i].Title,
			Parents: parents[merged[i].Slug],
			Edges:   merged[i].Related,
		})
	}
	return model, nil
}

// readSourceBook reads one source book: its header, skills, intra-book edges, and
// (per merged skill, into the shared parents map) the source skills whose ledger
// says they merged into it during runSlug.
func readSourceBook(
	srcDir, runSlug string, parents map[string][]book2skill.MergeParent,
) (book2skill.MergeSourceBook, error) {
	slug := filepath.Base(filepath.Clean(srcDir))
	book := book2skill.MergeSourceBook{Slug: slug, Title: slug, Superseded: map[string]bool{}}
	if o, ok, _ := store.ReadOverview(srcDir); ok {
		book.Title, book.Author = o.Title, o.Author
	}
	skills, err := store.GatherSkills(srcDir)
	if err != nil {
		return book, fmt.Errorf("mergetree: %w", err)
	}
	inBook := make(map[string]bool, len(skills))
	for i := range skills {
		inBook[skills[i].Slug] = true
	}
	for i := range skills {
		slugName := skills[i].Slug
		book.Skills = append(book.Skills, slugName)
		book.Edges = append(book.Edges, intraBookEdges(&skills[i], inBook)...)
		err := ledgerParents(srcDir, slug, slugName, runSlug, book.Superseded, parents)
		if err != nil {
			return book, err
		}
	}
	return book, nil
}

// intraBookEdges returns the skill's relationships whose target is another skill
// in the same book.
func intraBookEdges(sk *book2skill.Skill, inBook map[string]bool) []book2skill.Relationship {
	var edges []book2skill.Relationship
	for _, rel := range sk.Related {
		if inBook[rel.To] {
			edges = append(edges, rel)
		}
	}
	return edges
}

// ledgerParents parses one source skill's merge-status block and records, for
// this run, the merged skills it fed (marking the skill superseded).
func ledgerParents(
	srcDir, bookSlug, skillSlug, runSlug string,
	superseded map[string]bool,
	parents map[string][]book2skill.MergeParent,
) error {
	data, err := os.ReadFile(filepath.Join(srcDir, skillSlug, store.SkillFile))
	if err != nil {
		return nil //nolint:nilerr // a skill without a readable SKILL.md contributes no parents
	}
	entries, err := mergedoc.Parse(string(data))
	if err != nil {
		return fmt.Errorf("mergetree: %w", err)
	}
	for i := range entries {
		e := entries[i]
		if e.Run != runSlug || e.Into == "" {
			continue
		}
		if e.State != book2skill.StateMerged && e.State != book2skill.StatePartial {
			continue
		}
		superseded[skillSlug] = true
		parents[e.Into] = append(parents[e.Into], book2skill.MergeParent{
			BookSlug: bookSlug, SkillSlug: skillSlug, State: e.State,
		})
	}
	return nil
}
