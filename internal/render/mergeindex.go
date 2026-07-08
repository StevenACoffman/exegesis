package render

import (
	"strconv"
	"strings"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

// MergeIndex renders the cross-book INDEX.md for a merged-skills run: the source
// books, the provenance table, a cross-book relationship graph, and the
// superseded source skills. Tables use single-space cells; a markdown formatter
// pads them, and merge-index --check compares padding-normalized. Headings are
// title-cased and the output ends in a single newline.
func MergeIndex(mi *book2skill.MergeIndex) string {
	var b strings.Builder
	fprintf(&b, "# Merged Skills Index — %s\n\n", mi.RunSlug)
	renderSourceBooks(&b, mi.Sources)
	renderProvenance(&b, mi.Merges)
	renderMergeGraph(&b, mi)
	renderSuperseded(&b, mi)
	return single(&b)
}

func renderSourceBooks(b *strings.Builder, sources []book2skill.MergeSourceBook) {
	fprintf(b, "## Source Books\n\n")
	rows := make([][]string, 0, len(sources))
	for i := range sources {
		s := sources[i]
		rows = append(rows, []string{
			"*" + s.Title + "*", s.Author, "`" + s.Slug + "`", strconv.Itoa(len(s.Skills)),
		})
	}
	writeTable(b, []string{"Book", "Author", "Slug", "Skills Scanned"}, rows)
}

func renderProvenance(b *strings.Builder, merges []book2skill.MergeRecord) {
	fprintf(b, "## Provenance\n\n")
	rows := make([][]string, 0, len(merges))
	for i := range merges {
		m := merges[i]
		sources := make([]string, 0, len(m.Parents))
		for _, p := range m.Parents {
			sources = append(sources, "`"+p.BookSlug+"/"+p.SkillSlug+"`")
		}
		rows = append(rows, []string{
			"[`" + m.Slug + "`](./" + m.Slug + "/SKILL.md)",
			strings.Join(sources, ", "), "convergence", "active",
		})
	}
	writeTable(b, []string{"Merged Skill", "Sources", "Merge Type", "Status"}, rows)
}

func renderMergeGraph(b *strings.Builder, mi *book2skill.MergeIndex) {
	fprintf(b, "## Cross-Book Skill Graph\n\n```mermaid\ngraph LR\n")
	for i := range mi.Sources {
		s := mi.Sources[i]
		fprintf(b, "    subgraph %q\n", s.Title)
		for _, skill := range s.Skills {
			class := ""
			if s.Superseded[skill] {
				class = ":::superseded"
			}
			fprintf(b, "        %s[%q]%s\n", nodeID("s", s.Slug, skill), skill, class)
		}
		fprintf(b, "    end\n")
	}
	for i := range mi.Merges {
		m := mi.Merges[i]
		fprintf(b, "    %s[%q]:::merged\n", nodeID("m", m.Slug), m.Slug)
		for _, p := range m.Parents {
			fprintf(b, "    %s -->|superseded-by| %s\n",
				nodeID("s", p.BookSlug, p.SkillSlug), nodeID("m", m.Slug))
		}
	}
	fprintf(b, "    classDef merged fill:#c8e6c9,stroke:#388e3c\n")
	fprintf(b, "    classDef superseded fill:#ffe0b2,stroke:#e65100,stroke-dasharray:4 4\n")
	fprintf(b, "```\n\n")
}

func renderSuperseded(b *strings.Builder, mi *book2skill.MergeIndex) {
	fprintf(b, "## Superseded Source Skills\n\n")
	var rows [][]string
	for i := range mi.Merges {
		m := mi.Merges[i]
		for _, p := range m.Parents {
			rows = append(rows, []string{
				"`" + p.BookSlug + "/" + p.SkillSlug + "`", "`" + m.Slug + "`", mi.RunSlug,
			})
		}
	}
	writeTable(b, []string{"Source Skill", "Superseded By", "Run"}, rows)
}

// writeTable emits a GitHub table with single-space cells (a formatter aligns it).
func writeTable(b *strings.Builder, headers []string, rows [][]string) {
	fprintf(b, "| %s |\n", strings.Join(headers, " | "))
	dashes := make([]string, len(headers))
	for i := range dashes {
		dashes[i] = "---"
	}
	fprintf(b, "| %s |\n", strings.Join(dashes, " | "))
	for _, row := range rows {
		fprintf(b, "| %s |\n", strings.Join(row, " | "))
	}
	fprintf(b, "\n")
}

// nodeID builds a Mermaid-safe node identifier from a prefix and slug parts.
func nodeID(prefix string, parts ...string) string {
	safe := make([]string, len(parts))
	for i, p := range parts {
		safe[i] = strings.NewReplacer("-", "_", ".", "_", "/", "_", " ", "_").Replace(p)
	}
	return prefix + "_" + strings.Join(safe, "_")
}
