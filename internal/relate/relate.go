// Package relate is the pure model behind `exegesis relate`: it parses a relations edge
// table (JSON) into validated, slug-normalized edges grouped by source skill and sorted
// for deterministic output. Parse is pure — bytes in, values out — so the command owns
// the file read and the per-skill writes.
package relate

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/StevenACoffman/exegesis/internal/related"
	"github.com/StevenACoffman/skillet/skill"
)

// Table is the top-level edge-table input.
type Table struct {
	Edges []Row `json:"edges"`
}

// Row is one edge in the table: an edge of Kind from the From skill to the To skill.
type Row struct {
	From      string `json:"from"`
	Kind      string `json:"kind"`
	To        string `json:"to"`
	Rationale string `json:"rationale"`
}

// Group is the edges to write into one source skill's `## Related skills` section.
type Group struct {
	Slug  string
	Edges []related.Edge
}

// Parse unmarshals an edge table and returns its edges grouped by source skill, sorted
// by source slug so the output is deterministic. It validates each row (known kind,
// non-empty from/to/rationale) and normalizes both endpoints to slugs; a bad row is an
// error, never a silent drop.
func Parse(data []byte) ([]Group, error) {
	var table Table
	if err := json.Unmarshal(data, &table); err != nil {
		return nil, fmt.Errorf("parse edge table: %w", err)
	}
	bySlug := map[string][]related.Edge{}
	order := []string{}
	for i := range table.Edges {
		row := &table.Edges[i]
		kind := related.Kind(row.Kind)
		if !kind.Valid() {
			return nil, fmt.Errorf("edge %d: unknown kind %q", i+1, row.Kind)
		}
		if row.From == "" || row.To == "" || row.Rationale == "" {
			return nil, fmt.Errorf("edge %d: from, to, and rationale are required", i+1)
		}
		from, to := skill.Slug(row.From), skill.Slug(row.To)
		if _, seen := bySlug[from]; !seen {
			order = append(order, from)
		}
		bySlug[from] = append(
			bySlug[from],
			related.Edge{Kind: kind, Target: to, Rationale: row.Rationale},
		)
	}
	slices.Sort(order)
	groups := make([]Group, 0, len(order))
	for _, slug := range order {
		groups = append(groups, Group{Slug: slug, Edges: bySlug[slug]})
	}
	return groups, nil
}
