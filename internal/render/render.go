// Package render turns book2skill domain values into the markdown and JSON
// artifacts the pipeline writes: SKILL.md, BOOK_OVERVIEW.md, and INDEX.md. All
// functions are pure — they take values and return strings — so they are
// golden-file testable without any I/O.
package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

const descWrapWidth = 76

// Skill renders s as a SKILL.md document whose six RIA++ segments use the
// canonical headings required by book2skill.ParseSegments (## R, ## I, ## A1,
// ## A2, ## E, ## B).
func Skill(s *book2skill.Skill) string {
	var b strings.Builder
	renderFrontmatter(&b, s)
	fprintf(&b, "\n# %s\n\n", s.Title)
	renderReading(&b, s.Reading)
	fprintf(&b, "## I — Interpretation\n\n%s\n\n", s.Interpretation)
	renderApplication(&b, s.Application)
	renderTrigger(&b, s.Trigger)
	renderExecution(&b, s.Execution)
	renderBoundary(&b, &s.Boundary)
	renderRelated(&b, s.Related)
	fprintf(&b, "## Provenance\n\n- **Source:** %s\n", s.Provenance)
	return single(&b)
}

// BookOverview renders o as BOOK_OVERVIEW.md.
func BookOverview(o *book2skill.BookOverview) string {
	var b strings.Builder
	fprintf(&b, "# %s — Book Overview\n\n", o.Title)
	fprintf(&b, "- **Author:** %s\n- **Year:** %s\n- **Genre:** %s\n\n",
		o.Author, o.Year, o.Structure.Genre)
	fprintf(&b, "## One-Sentence Summary\n\n%s\n\n", o.Structure.OneSentenceSummary)
	renderList(&b, "Skeleton", o.Structure.Skeleton)
	fprintf(&b, "**Argument relationship:** %s\n\n", o.Structure.ArgumentRelationship)
	fprintf(&b, "**Core problem:** %s\n\n", o.Structure.CoreProblem)
	renderKeyTerms(&b, o.Interpretation.KeyTerms)
	renderList(&b, "Core Propositions", o.Interpretation.CorePropositions)
	renderCritique(&b, &o.Critique)
	renderList(&b, "Skillable Topics", o.Applicability.SkillableTopics)
	return single(&b)
}

// Index renders INDEX.md for a distilled book: its skills, a Mermaid
// relationship graph, and a dependency-ordered learning path.
func Index(o *book2skill.BookOverview, skills []book2skill.Skill) string {
	var b strings.Builder
	fprintf(&b, "# %s — Skill Index\n\n", o.Title)
	fprintf(&b, "%d skills distilled from *%s* by %s.\n\n",
		len(skills), o.Title, o.Author)

	fprintf(&b, "## Skills\n\n")
	for i := range skills {
		fprintf(&b, "- [`%s`](./%s/SKILL.md) — %s\n",
			skills[i].Slug, skills[i].Slug, skills[i].Title)
	}
	fprintf(&b, "\n")

	renderGraph(&b, skills)
	renderLearningOrder(&b, skills)
	return single(&b)
}

// single returns the builder's contents with trailing blank lines collapsed to a
// single terminating newline, matching what a markdown formatter emits (so
// generated documents are formatter fixed points).
func single(b *strings.Builder) string {
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// LearningOrder returns skill slugs ordered so that every skill appears after
// the skills it depends on. Cycles and edges to unknown skills are ignored; ties
// break alphabetically for deterministic output.
func LearningOrder(skills []book2skill.Skill) []string {
	known := make(map[string]bool, len(skills))
	for i := range skills {
		known[skills[i].Slug] = true
	}
	indegree := make(map[string]int, len(skills))
	deps := make(map[string][]string, len(skills))
	for i := range skills {
		indegree[skills[i].Slug] = 0
	}
	for i := range skills {
		for _, rel := range skills[i].Related {
			if rel.Kind != book2skill.DependsOn || !known[rel.To] || rel.To == skills[i].Slug {
				continue
			}
			deps[rel.To] = append(deps[rel.To], skills[i].Slug)
			indegree[skills[i].Slug]++
		}
	}
	return kahnSort(indegree, deps)
}

func renderFrontmatter(b *strings.Builder, s *book2skill.Skill) {
	fprintf(b, "---\nname: %s\ndescription: >-\n", s.Slug)
	for _, line := range wrapText(s.Description, descWrapWidth) {
		fprintf(b, "  %s\n", line)
	}
	if len(s.Tags) > 0 {
		fprintf(b, "tags: [%s]\n", strings.Join(s.Tags, ", "))
	}
	fprintf(b, "---\n")
}

func renderReading(b *strings.Builder, r book2skill.Reading) {
	fprintf(b, "## R — Original Text (Reading)\n\n")
	for _, line := range strings.Split(r.Quote, "\n") {
		fprintf(b, "> %s\n", line)
	}
	if r.Attribution != "" {
		fprintf(b, ">\n> — %s\n", r.Attribution)
	}
	fprintf(b, "\n")
}

func renderApplication(b *strings.Builder, cases []book2skill.ApplicationCase) {
	fprintf(b, "## A1 — Past Application (From the Book)\n\n")
	for i := range cases {
		c := cases[i]
		fprintf(b, "### %s\n\n", c.Name)
		fprintf(b, "- **Problem:** %s\n", c.Problem)
		fprintf(b, "- **Methodology:** %s\n", c.MethodologyUse)
		fprintf(b, "- **Conclusion:** %s\n", c.Conclusion)
		fprintf(b, "- **Result:** %s\n\n", c.Result)
	}
}

func renderTrigger(b *strings.Builder, t book2skill.Trigger) {
	fprintf(b, "## A2 — Trigger (When to Invoke)\n\n")
	for i, s := range t.Scenarios {
		fprintf(b, "%d. %s\n", i+1, s)
	}
	fprintf(b, "\n### Language Signals\n\n")
	for _, sig := range t.LanguageSignals {
		fprintf(b, "- %q\n", sig)
	}
	if len(t.AdjacentDistinctions) > 0 {
		fprintf(b, "\n### Distinct from Adjacent Skills\n\n")
		for _, d := range t.AdjacentDistinctions {
			fprintf(b, "- `%s` — %s\n", d.Skill, d.Difference)
		}
	}
	fprintf(b, "\n")
}

func renderExecution(b *strings.Builder, steps []book2skill.Step) {
	fprintf(b, "## E — Execution\n\n")
	for i := range steps {
		st := steps[i]
		fprintf(b, "%d. **%s**\n", i+1, st.Text)
		fprintf(b, "   - Completion: %s\n", st.CompletionCriterion)
		if st.StopCondition != "" {
			fprintf(b, "   - Stop: %s\n", st.StopCondition)
		}
	}
	fprintf(b, "\n")
}

func renderBoundary(b *strings.Builder, bd *book2skill.Boundary) {
	fprintf(b, "## B — Boundary\n\n")
	renderSubList(b, "Do Not Use When", bd.AntiScenarios)
	renderSubList(b, "Failure Patterns the Author Warns About", bd.AuthorWarnedFailures)
	renderSubList(b, "Author Blind Spots and Era Limitations", bd.AuthorBlindSpots)
	renderSubList(b, "Easily Confused With", bd.ConfusableNeighbors)
}

func renderRelated(b *strings.Builder, rels []book2skill.Relationship) {
	if len(rels) == 0 {
		return
	}
	fprintf(b, "## %s\n\n", book2skill.RelatedSkillsHeading)
	for _, r := range rels {
		fprintf(b, "- %s: `%s` — %s\n", r.Kind, r.To, r.Rationale)
	}
	fprintf(b, "\n")
}

func renderCritique(b *strings.Builder, c *book2skill.Critique) {
	renderList(b, "Era Limitations", c.EraLimitations)
	renderList(b, "Author Blind Spots", c.AuthorBlindSpots)
	renderList(b, "Unproven Assumptions", c.UnprovenAssumptions)
	fprintf(b, "**Strongest objection:** %s\n\n", c.StrongestObjection)
}

func renderKeyTerms(b *strings.Builder, terms []book2skill.KeyTerm) {
	fprintf(b, "## Key Terms\n\n")
	for _, t := range terms {
		fprintf(b, "- **%s:** %s (differs from common usage: %s)\n",
			t.Term, t.AuthorDefinition, t.DiffersFromCommon)
	}
	fprintf(b, "\n")
}

func renderGraph(b *strings.Builder, skills []book2skill.Skill) {
	fprintf(b, "## Relationship Graph\n\n```mermaid\ngraph LR\n")
	for i := range skills {
		for _, rel := range skills[i].Related {
			fprintf(b, "    %s -->|%s| %s\n", skills[i].Slug, rel.Kind, rel.To)
		}
	}
	fprintf(b, "```\n\n")
}

func renderLearningOrder(b *strings.Builder, skills []book2skill.Skill) {
	fprintf(b, "## Recommended Learning Order\n\n")
	for i, slug := range LearningOrder(skills) {
		fprintf(b, "%d. `%s`\n", i+1, slug)
	}
	fprintf(b, "\n")
}

func renderList(b *strings.Builder, heading string, items []string) {
	fprintf(b, "## %s\n\n", heading)
	for _, item := range items {
		fprintf(b, "- %s\n", item)
	}
	fprintf(b, "\n")
}

func renderSubList(b *strings.Builder, heading string, items []string) {
	fprintf(b, "### %s\n\n", heading)
	for _, item := range items {
		fprintf(b, "- %s\n", item)
	}
	fprintf(b, "\n")
}

// kahnSort topologically orders the nodes in indegree using Kahn's algorithm,
// breaking ties alphabetically so output is deterministic.
func kahnSort(indegree map[string]int, deps map[string][]string) []string {
	var ready []string
	for node, deg := range indegree {
		if deg == 0 {
			ready = append(ready, node)
		}
	}
	sort.Strings(ready)

	order := make([]string, 0, len(indegree))
	for len(ready) > 0 {
		node := ready[0]
		ready = ready[1:]
		order = append(order, node)
		var unblocked []string
		for _, dependent := range deps[node] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				unblocked = append(unblocked, dependent)
			}
		}
		sort.Strings(unblocked)
		ready = append(ready, unblocked...)
	}
	return order
}

// wrapText greedily wraps s into lines no longer than width runes, splitting on
// spaces. A single word longer than width becomes its own line.
func wrapText(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) > width {
			lines = append(lines, current)
			current = word
			continue
		}
		current += " " + word
	}
	return append(lines, current)
}

func fprintf(b *strings.Builder, format string, args ...any) {
	_, _ = fmt.Fprintf(b, format, args...)
}
