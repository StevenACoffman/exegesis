# Plan — merge-index, A2-sharpness, and Phase 3 source-link automation

Three more merge-skills mechanics move behind `exegesis`. Split (advice §1, §2,
§5): pure core in `book2skill` (parsing, models, set logic, markdown edits with no
third-party imports); the pure `render` package for domain→markdown; data
assembly from `store` + `mergedoc` adapters; thin `cmd/*` shells.

______________________________________________________________________

## M1 — Phase 3 source-link automation

A general Zettelkasten link inserter — appends a relationship bullet to a skill's
`## Related Skills` section. Reusable by book2skill Phase 3 (depends-on /
contrasts-with / composes-with) and merge-skills Phase 3 (superseded-by).

**`book2skill` (pure, no new imports):**

```go
// AppendRelated returns md with a "- <kind>: `<to>` — <rationale>" bullet added
// to the ## Related Skills section (created at EOF if absent). It is idempotent:
// an identical (kind,to) bullet is not duplicated. changed reports whether md
// was modified.
func AppendRelated(md string, rel Relationship) (out string, changed bool)
```

Reuses the case-insensitive section scan (shared with `sectionBody`/`ParseRelated`)
to locate `## Related Skills`; renders the bullet with the same shape
`renderRelated` emits (one source of truth via `RelatedSkillsHeading`). Idempotency
uses `ParseRelated` to detect an existing (kind,to) edge.

**`cmd/link`:** `exegesis link --kind <kind> --to <slug> [--rationale <text>] <skill-dir>`
— validate `kind` via `RelationshipKind.Valid()`, read SKILL.md, `AppendRelated`,
write back; report "linked"/"already present" (idempotent, never errors on a repeat).

**Tests:** `AppendRelated` creates the section, is idempotent, preserves body;
command via `cmd.Run`. **Wire:** merge SKILL.md Phase 3 (`superseded-by`) and
book2skill Phase 3 (the three kinds).

## M2 — A2-sharpness check

**`book2skill` (pure):**

```go
// LanguageSignals returns the signals listed under the "### Language Signals"
// subsection of a SKILL.md body's A2 segment, with surrounding quotes stripped.
func LanguageSignals(body string) []string

// A2Sharpness returns the merged skill's language signals that appear in none of
// the source skills' signals (whitespace/case-normalized). len ≥ 2 satisfies the
// structural half of merge Red Line #3; semantic distinctness stays judgment.
func A2Sharpness(mergedBody string, sourceBodies []string) []string
```

`LanguageSignals` extracts the A2 segment (`ParseSegments`) then scans its
`### Language Signals` subsection for `- "…"` bullets (case-insensitive heading;
formatter-tolerant). `A2Sharpness` is a normalized set difference.

**`cmd/a2check`:** `exegesis a2check --source <srcA>,<srcB> <merged-skill-dir>` —
reads the merged skill body + each source skill body, computes `A2Sharpness`,
reports the count and the unique signals. Advisory by default (exit 0, prints
`WARN` when < 2 — semantic distinctness is the agent's call); `--strict` exits 1
when < 2 for CI. It is a **per-skill** command (merged skill + its specific source
skills), not a `verify --merge` tree gate, because the merged↔source mapping is
per-skill — a cleaner fit than a tree-level warning.

**Tests:** `LanguageSignals` extraction; `A2Sharpness` (sharp vs dull); command
advisory + `--strict` via `cmd.Run`. **Wire:** merge Red Line #3.

## M3 — Cross-book merge-index

`render.Index` is single-book and flat; the merge index is cross-book (per-source
subgraphs, `superseded-by` edges, provenance). New renderer + command. It builds
only the **deterministically-derivable** sections from the merge-status ledgers +
tree reads; the judgment/audit sections (Source-verification summary, free-text
Notes) are left to the agent, and this is documented in the output.

**`book2skill` (pure model, like `BookOverview`):**

```go
type MergeIndex struct {
    RunSlug string
    Sources []MergeSourceBook   // one per --source book
    Merges  []MergeRecord       // one per merged skill (with its parents)
}
type MergeSourceBook struct { Slug, Title, Author string; Skills []string; Superseded map[string]bool }
type MergeRecord struct { Slug, Title string; Parents []MergeParent }
type MergeParent struct { BookSlug, SkillSlug string; State MergeState }
```

**`render` (pure):** `func MergeIndex(*book2skill.MergeIndex) string` emitting, with
title-cased headings and a single trailing newline (a formatter fixed point):

1. `## Source Books` table (Book, Author, Slug, Skills scanned).
2. `## Provenance` table (Merged skill, Sources, Merge type = convergence, Status = active).
3. `## Cross-Book Skill Graph` — one `subgraph` per source book (skills; superseded
   ones tagged `:::superseded`), merged nodes tagged `:::merged`, `superseded-by`
   edges, and the `classDef`s. Node IDs are sanitized (hyphens→underscores) so the
   Mermaid parse is unambiguous, with the slug as the visible label.
4. `## Superseded Source Skills` table (Source skill, Superseded by, Run).

Split into per-section helpers to stay under funlen/cyclop.

**`cmd/mergeindex`:** `exegesis merge-index --source <bookA>,<bookB> <merged-tree>`
assembles the model (merged skills via `store.GatherSkills`; each source book via
`store.ReadOverview` + `store.GatherSkills`; parents/superseded by parsing each
source skill's ledger with `mergedoc.Parse` and keeping entries whose `run`
matches the tree slug and whose `into` is set), then writes
`<merged-tree>/INDEX.md`. `--check` compares without writing (exit 1 if stale),
mirroring `index`.

**Tests:** `render.MergeIndex` golden shape + a formatter fixed-point check;
command e2e via `cmd.Run` (assemble a merged tree + a source book with a ledger →
INDEX.md has the provenance row + superseded edge; `--check` passes then flags
staleness).

## M4 — Verify, wire, reformat

Full `go build` + `go test -race` + `golangci-lint` (0 issues). Wire the three
commands into merge (+ book2skill) SKILL.md/methodology; reformat all touched
markdown with mdformat+rumdl; run a combined e2e (`link`, `a2check`, `merge-index`)
and confirm formatter idempotency. Then suggest next steps.

______________________________________________________________________

## Go-advice refinements applied

1. **§1/§2 layering.** `AppendRelated`, `LanguageSignals`, `A2Sharpness`, and the
   `MergeIndex` model are pure `book2skill` (stdlib only); yaml stays in
   `mergedoc`; `render` stays domain→string; assembly (file/tree/ledger reads) is
   the command shell over `store`+`mergedoc`. No cross-subpackage imports.
2. **§4 reuse, don't duplicate.** `AppendRelated` shares the section scan and
   bullet shape with `renderRelated`/`ParseRelated` (one `RelatedSkillsHeading`);
   `merge-index` reuses `store.GatherSkills`/`ReadOverview` and `mergedoc.Parse`;
   the graph reuses the existing mermaid-edge idiom.
3. **§3 define errors out of existence.** `AppendRelated` is idempotent (a repeat
   is a no-op, not an error); `link` never fails on a repeat. Leaf failures are
   `Error{Code:EINVALID}`; wrapping `Error{Op,Err}`; `fmt.Errorf("…: %w")` only at
   adapter boundaries.
4. **§2/§4 return values, not errors.** `A2Sharpness` returns the unique signals
   (`[]string`); the shell decides warn/strict. `AppendRelated` returns
   `(string, bool)`. Pure core, no side effects.
5. **§5 functional core / imperative shell.** All parsing/set-logic/rendering is a
   deterministic function of inputs; commands are flat read→compute→write. No
   globals, no `time.Now` (the index uses the run slug, not a date, keeping it a
   fixed point).
6. **Formatter tolerance & fixed points.** All new heading/subsection matching is
   case-insensitive (title-cased `## Related Skills`, `### Language Signals`);
   `render.MergeIndex` emits title-cased headings + single trailing newline and is
   verified an mdformat+rumdl fixed point (as `render.Index` is).
7. **§9/§10 tests.** Black-box packages, `t.TempDir`, stdlib only, through the real
   `cmd.Run`; `merge-index` uses the real renderer so generate→format→check holds.
8. **Lint-clean by construction.** funlen/cyclop via per-section helpers;
   const→type→func order (`decorder`); domain values by slice/pointer (no
   large-by-value params); no builtin shadowing.

## Milestone checkpoints

Each milestone ends green: `go build` + `go test -race` (touched packages) +
`golangci-lint run` = 0 issues, improved per go-advice before proceeding.
