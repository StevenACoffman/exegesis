# Plan — richer merge-index graph, verify --merge A2-sharpness, merge-status --link

Three enhancements to the merge tooling. Split (advice §1, §2, §5): pure model +
graph rendering + set logic in `book2skill`/`render`; the book-tree assembly
(which currently lives in `cmd/mergeindex`) extracted to a shared
`internal/mergetree` adapter so `cmd/mergeindex` and `cmd/verify` reuse it rather
than importing each other; thin command shells.

______________________________________________________________________

## M1 — Cross-book graph edges (+ extract `internal/mergetree`)

**Extract assembly (advice §4 reuse, §1 no cmd→cmd imports).** Move
`assemble`/`readSourceBook`/`recordParents` out of `cmd/mergeindex` into
`internal/mergetree`:

```go
// Assemble builds the merge-index model for a merged tree from the source books'
// merge-status ledgers.
func mergetree.Assemble(tree string, sourceDirs []string) (*book2skill.MergeIndex, error)
```

`cmd/mergeindex` becomes a thin shell over it (behaviour unchanged; its e2e test
still passes).

**Enrich the model with relationship edges** so the graph shows more than
`superseded-by`:

```go
type MergeSourceBook struct { …; Edges []Relationship } // intra-book edges (both ends in this book)
type MergeRecord     struct { …; Edges []Relationship } // the merged skill's own ## Related Skills
```

Assembly populates them: `store.GatherSkills` already returns each skill's
`Related` (via `ParseRelated`); for a source book, keep edges whose `To` is also a
skill in that book; for a merged skill, keep all its `Related` edges.

**Render** (`render.renderMergeGraph`, split into helpers to stay under
funlen/cyclop):

- Per-source `subgraph`: nodes (superseded ones tagged) **plus** intra-book edges
  `from -->|kind| to`.
- Merged nodes + `superseded-by` edges (unchanged).
- Merged-skill edges: resolve each edge's `To` against a global slug→node index
  (all source skills + merged skills). Draw resolvable edges from the merged node
  (`-.->` for `contrasts-with`, `-->` otherwise, matching the template); skip
  unresolvable bare slugs. Ambiguous slugs (same slug in two books) resolve to the
  first and are documented.

Still an mdformat+rumdl fixed point (edges live inside the ```mermaid fence, which
the formatters leave alone; `--check` already tolerates table/heading cosmetics).

Tests: `render.MergeIndex` shows an intra-book edge and a resolved merged edge;
`mergetree.Assemble` populates edges; existing merge-index e2e stays green.

## M2 — `verify --merge --source` A2-sharpness (advisory)

Add `--source <bookA>,<bookB>` to `verify`. In `--merge` mode with sources, add an
**A2-sharpness** advisory gate: reuse `mergetree.Assemble` to map each merged skill
to its source skills (via the ledgers), read the merged and source A2 bodies, and
run `book2skill.A2Sharpness`. A merged skill with `< MinSharpSignals` unique
signals is reported (per-skill) — advisory (a `gateOutcome` that passes but prints
a `note`), escalated to a failure only under the existing `--strict`. Without
`--source`, the gate is skipped (no mapping available), so current behaviour is
unchanged.

This keeps the per-skill A2 mapping correct (each merged skill → its own sources)
while surfacing the signal at the tree level the user asked for.

Tests via `cmd.Run`: a sharp merged tree passes clean; a dull one prints the note
(and fails under `--strict`).

## M3 — `merge-status append --link`

Add a `--link` boolean to `merge-status append`. When set and the state is
`merged` or `partial`, after appending the ledger entry also append a
`- superseded-by: <into>` bullet to the same skill's `## Related Skills` via
`book2skill.AppendRelated` (idempotent) — collapsing the two Phase-3 annotations
into one call. `--link` with a non-superseding state is a no-op link (reported).

Tests: `append --link --state merged --into X` writes both the ledger entry and
the `superseded-by` bullet; re-running is idempotent on the bullet.

## M4 — Verify, wire, reformat, commit

Full `go build` + `go test -race` + `golangci-lint` (0 issues). Wire the new
flags into merge SKILL.md (Phase 3 `merge-status append --link`; the graph note;
`verify --merge --source`); reformat touched markdown with mdformat+rumdl; run a
combined e2e; confirm formatter idempotency; commit. Then suggest next steps.

______________________________________________________________________

## Go-advice refinements applied

1. **§1/§4 — extract shared assembly.** `internal/mergetree.Assemble` is owned by
   one package and reused by `cmd/mergeindex` and `cmd/verify`; commands never
   import each other. The model stays pure `book2skill`; yaml stays in `mergedoc`.
2. **§5 functional core / imperative shell.** Edge resolution and A2 set logic are
   pure functions of the model/bodies; commands read files and print. The node
   index for edge resolution is built inside the pure renderer from the model.
3. **§2/§4 return values, not errors / no hidden control flow.** `A2Sharpness`
   returns the unique signals; `verify` decides note-vs-fail. `AppendRelated`
   returns `(string, changed)`. `--link` on a non-superseding state is a
   documented no-op, not an error (define-out).
4. **§4 model constraints in types; reuse.** Edges reuse the existing
   `Relationship`/`RelationshipKind`; the graph reuses `nodeID`; the ledger reuse
   goes through `mergedoc`/`mergetree`.
5. **Formatter tolerance / fixed points.** New graph content is inside the mermaid
   fence (untouched by the formatters); `merge-index --check` stays padding- and
   heading-case-tolerant; no new byte-fragile comparisons.
6. **§9/§10 tests.** Black-box packages, `t.TempDir`, stdlib only, through the real
   `cmd.Run`; the merge-index renderer is exercised with the real renderer.
7. **Lint-clean by construction.** `renderMergeGraph` split into `renderSubgraphs`
   / `renderMergedNodes` / `renderMergedEdges` for funlen/cyclop; const→type→func
   order; no globals; slices/pointers not large-by-value params; no builtin
   shadowing.

## Milestone checkpoints

Each milestone ends green: `go build` + `go test -race` (touched packages) +
`golangci-lint run` = 0 issues, improved per go-advice before proceeding.
