// Package mergetree assembles the cross-book merge-index model from a merged
// tree and its source books, reading the append-only merge-status ledgers on the
// source skills. It is the shared book-tree reader used by both the merge-index
// and verify commands, so neither command imports the other.
package mergetree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/mergedoc"
	"github.com/StevenACoffman/exegesis/internal/store"
)

const (
	verificationDir = "source-verification"
	mergedDirName   = "merged"
)

// DiscoverSources returns the source book directories that contributed to a
// merged tree: the sibling directories of merged/ under the books root whose
// skills carry a merge-status entry for this run. It errors when tree is not
// under a books/merged/ layout, so a caller can fall back to explicit --source.
func DiscoverSources(tree string) ([]string, error) {
	clean := filepath.Clean(tree)
	mergedDir := filepath.Dir(clean)
	if filepath.Base(mergedDir) != mergedDirName {
		return nil, fmt.Errorf(
			"mergetree: cannot infer sources: %s is not under a books/%s/ root",
			tree,
			mergedDirName,
		)
	}
	booksRoot := filepath.Dir(mergedDir)
	runSlug := filepath.Base(clean)
	entries, err := os.ReadDir(booksRoot)
	if err != nil {
		return nil, fmt.Errorf("mergetree: %w", err)
	}
	var sources []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == mergedDirName {
			continue
		}
		dir := filepath.Join(booksRoot, e.Name())
		if bookHasRun(dir, runSlug) {
			sources = append(sources, dir)
		}
	}
	return sources, nil
}

// bookHasRun reports whether any skill under dir has a merge-status entry for
// runSlug (i.e. the book participated in that merge run).
func bookHasRun(dir, runSlug string) bool {
	skills, err := store.GatherSkills(dir)
	if err != nil {
		return false
	}
	for i := range skills {
		data, err := os.ReadFile(filepath.Join(dir, skills[i].Slug, store.SkillFile))
		if err != nil {
			continue
		}
		entries, err := mergedoc.Parse(string(data))
		if err != nil {
			continue
		}
		for j := range entries {
			if entries[j].Run == runSlug {
				return true
			}
		}
	}
	return false
}

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
	rejected := make(map[string]book2skill.MergeReason)
	model := &book2skill.MergeIndex{RunSlug: runSlug}
	for _, srcDir := range sourceDirs {
		book, err := readSourceBook(srcDir, runSlug, parents, rejected)
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
	rows, err := verificationRows(tree, rejected)
	if err != nil {
		return nil, err
	}
	model.Verification = rows
	return model, nil
}

// verificationRows reads <tree>/source-verification/*.md, groups the artifact
// headers by pair, and attaches each pair's V1–V4 outcome (a rejected reason, or
// "all pass"). Returns nil when the directory is absent.
func verificationRows(tree string, rejected map[string]book2skill.MergeReason) (
	[]book2skill.VerificationRow, error,
) {
	dir := filepath.Join(tree, verificationDir)
	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("mergetree: %w", err)
	}
	byPair := make(map[string]*book2skill.VerificationRow)
	var order []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if err := addVerification(filepath.Join(dir, e.Name()), byPair, &order); err != nil {
			return nil, err
		}
	}
	sort.Strings(order)
	rows := make([]book2skill.VerificationRow, 0, len(order))
	for _, pair := range order {
		row := byPair[pair]
		if reason, ok := rejected[pair]; ok {
			row.Validations = string(reason)
		} else {
			row.Validations = "all pass"
		}
		rows = append(rows, *row)
	}
	return rows, nil
}

// addVerification parses one artifact and merges its header into the pair's row.
func addVerification(
	path string,
	byPair map[string]*book2skill.VerificationRow,
	order *[]string,
) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil //nolint:nilerr // unreadable artifacts are skipped, not fatal
	}
	sv, ok, err := mergedoc.ParseVerification(string(data))
	if err != nil {
		return fmt.Errorf("mergetree: %w", err)
	}
	if !ok {
		return nil
	}
	row := byPair[sv.Pair]
	if row == nil {
		row = &book2skill.VerificationRow{Pair: sv.Pair}
		byPair[sv.Pair] = row
		*order = append(*order, sv.Pair)
	}
	switch sv.Check {
	case book2skill.CheckRQuoteAccuracy:
		row.R = sv.Sources
	case book2skill.CheckA1Attribution:
		row.A1 = sv.Sources
	}
	return nil
}

// readSourceBook reads one source book: its header, skills, intra-book edges, and
// (per merged skill, into the shared parents map) the source skills whose ledger
// says they merged into it during runSlug.
func readSourceBook(
	srcDir, runSlug string,
	parents map[string][]book2skill.MergeParent,
	rejected map[string]book2skill.MergeReason,
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
		err := ledgerEntries(srcDir, slug, slugName, runSlug, book.Superseded, parents, rejected)
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

// ledgerEntries parses one source skill's merge-status block and records, for
// this run, the merged skills it fed (marking it superseded) and any rejected
// pair's reason (for the verification summary's V1–V4 column).
func ledgerEntries(
	srcDir, bookSlug, skillSlug, runSlug string,
	superseded map[string]bool,
	parents map[string][]book2skill.MergeParent,
	rejected map[string]book2skill.MergeReason,
) error {
	data, err := os.ReadFile(filepath.Join(srcDir, skillSlug, store.SkillFile))
	if err != nil {
		return nil //nolint:nilerr // a skill without a readable SKILL.md contributes no entries
	}
	entries, err := mergedoc.Parse(string(data))
	if err != nil {
		return fmt.Errorf("mergetree: %w", err)
	}
	for i := range entries {
		e := entries[i]
		if e.Run != runSlug {
			continue
		}
		switch {
		case e.Into != "" && (e.State == book2skill.StateMerged || e.State == book2skill.StatePartial):
			superseded[skillSlug] = true
			parents[e.Into] = append(parents[e.Into], book2skill.MergeParent{
				BookSlug: bookSlug, SkillSlug: skillSlug, State: e.State,
			})
		case e.State == book2skill.StateRejected && e.Pair != "":
			rejected[e.Pair] = e.Reason
		}
	}
	return nil
}
