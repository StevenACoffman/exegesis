# Plan — `verify` Command, `index` Relationship Warning, and a Schema-Backed BOOK_OVERVIEW.md

Goal: make the whole "Quality Red Line (Output will be stopped if violated)"
surface a single deterministic gate, and extend it to the hand-authored
`BOOK_OVERVIEW.md` by turning that file into a schema (fixed headings) the CLI
can parse reliably — the condition we established for gating Stage 0 without
false results.

Three deliverables:

1. **`exegesis verify <book-dir>`** — one command that walks a `books/<slug>/`
   tree and runs every *mechanical* red line: the Stage-0 overview gate, per-skill
   `lint` (spec/quality/redlines), per-skill `tests` structural gate, and
   `INDEX.md` staleness. Mostly orchestration over pieces we already built.
2. **`index` relationship-count warning** — surface `methodology/05`'s heuristic
   (~0.8–1.5 relationships per skill; too few → over-independent, too many →
   artificial) as a non-fatal warning.
3. **Schema-backed `BOOK_OVERVIEW.md`** — rewrite the template so its
   gate-relevant sections use the exact level-2 headings `render.BookOverview`
   emits, and add a pure parser that inverts them. One format for both the
   machine (`distill`) and hand-authored paths.

Split (advice §5): pure core in `book2skill` (parse + gate + heuristic); the FS
read of a book tree in `store` (the existing filesystem adapter); thin
imperative shells in `cmd/index` and `cmd/verify`.

______________________________________________________________________

## Part a — Schema + Parser (`internal/book2skill`)

`render.BookOverview` already emits these level-2 headings, and the Stage-0 gate
(`BookOverview.QualityGate`, already in code) needs only four groups. The parser
recovers exactly those (documented as gate-scoped, not a full round-trip):

- `# <Title> — Book Overview` + `- **Author:** <author>` → via existing `ParseOverviewHeader`
- `## One-sentence summary` → `Structure.OneSentenceSummary`
- `## Skeleton` (list) → `Structure.Skeleton`
- `## Key terms` (list) → `Interpretation.KeyTerms` (count-accurate; `Term` parsed from `- **Term:** …`)
- `## Era limitations` / `## Author blind spots` / `## Unproven assumptions` (lists) → `Critique.*`

New pure functions:

```go
// ParseBookOverview recovers the Stage-0 quality-gate fields from a rendered
// BOOK_OVERVIEW.md. Inverse of render.BookOverview for those sections.
func ParseBookOverview(md string) BookOverview

// RelationshipCountAdvice returns a heuristic warning when the relationship
// count is out of band for the skill count (methodology/05), or "" when fine or
// when there are too few skills to judge.
func RelationshipCountAdvice(skills []Skill) string
```

Helpers: reuse the existing unexported `sectionBody` (skillmd.go, same package);
add unexported `listItems(body string) []string` (lines beginning `- `).

Heuristic band: for `n := len(skills)`, `r := total relationships`; skip if
`n < 4`; `low = ceil(0.8n)`, `high = floor(1.5n)`; warn below `low`
("skills may be too independent — recheck unit selection") or above `high`
("relationships may be artificial"). For n=10 this is [8, 15], matching the doc.

Tests (black-box): `ParseBookOverview` recovers counts/summary; a **round-trip**
`render.BookOverview(sample) → ParseBookOverview → QualityGate` passes (anti-drift
lock, same pattern as the SKILL.md round-trip); deficient overview fails the
gate; `RelationshipCountAdvice` table (in-band, too-few, too-many, n\<4).

## Part B — Book-Tree Reading (`internal/store`)

`store` is the book2skill filesystem adapter. Extend it from writing the output
tree to also reading it (updates its doc comment). Move `gatherSkills` here so
`cmd/index` and `cmd/verify` share it (advice §4 — kill Repetition; commands must
not import each other, §1):

```go
const (
	SkillFile    = "SKILL.md"
	OverviewFile = "BOOK_OVERVIEW.md"
	IndexFile    = "INDEX.md"
)

// GatherSkills reads each immediate subdir of dir containing a SKILL.md into a
// slug/title/relationships triple (sorted by name).
func GatherSkills(dir string) ([]book2skill.Skill, error)

// ReadOverview reads and parses dir/BOOK_OVERVIEW.md; ok=false if absent.
func ReadOverview(dir string) (o *book2skill.BookOverview, ok bool, err error)
```

`cmd/index` refactors to use these (its end-to-end tests must stay green).

## Part C — `index` Relationship Warning (`cmd/index`)

After gathering skills, `if msg := book2skill.RelationshipCountAdvice(skills); msg != ""`
print `"warning: " + msg` to `cfg.Stderr`. Non-fatal; never changes the exit code.

## Part D — `exegesis verify <book-dir>` (`cmd/verify`)

Aggregates four gates and reports each as a section, exiting 1 if any errors
(and, under `--strict`, if any warnings):

1. **Overview** — `store.ReadOverview` → `QualityGate()`; missing file or
   nonempty problems ⇒ fail.
2. **Skills (lint)** — `skilllint.Run(dir, {spec, quality, redlines})`; reuse
   `skilllint.Result.Counts()`/`ExitCode(strict)` and `WriteText`.
3. **Tests** — for each gathered skill, read `test-prompts.json`,
   `DecodeTestPrompts`, `ValidateTestSet`; a missing file or nonempty problems ⇒ fail.
4. **INDEX.md** — render expected via `render.Index(overview, skills)` and compare
   to the on-disk file; differ or absent ⇒ stale (fail).

Also surface `RelationshipCountAdvice` as a warning. Flags: `--strict`
(warnings→failure), `--format text|json`. Leaf failures are
`book2skill.Error{Code: EINVALID}`; the aggregate non-zero result returns
`root.ExitError(1)` after printing (mirrors `lint`/`tests`).

Keep `exec` a thin dispatcher; one helper per gate so each stays under funlen/cyclop.

Tests (black-box, via `cmd.Run`): build a self-consistent valid tree with the
real renderers (`render.BookOverview` passing the gate, one lint-clean skill via
`render.Skill` + a `TemplateTestCases` `test-prompts.json`, a current `INDEX.md`)
→ exit 0; then mutate one artifact (drop `test-prompts.json`, or make `INDEX.md`
stale) → `ExitError(1)`. Using the real renderers also exercises the render↔parse
inverse at the command level.

## Part E — Enforce the Schema (Skill Docs)

- Rewrite `templates/BOOK_OVERVIEW.md.template` to mirror `render.BookOverview`'s
  exact structure and level-2 headings (framework→`## Skeleton` list; key-terms
  table→`## Key terms` list; critique→three level-2 lists; H1 →
  `# {{Title}} — Book Overview`). Keep authoring guidance as HTML comments / prose
  (ignored by the parser). Update the trailing quality-gate checklist to point at
  `exegesis verify`.
- `SKILL.md` Stage 0: after writing `BOOK_OVERVIEW.md`, run
  `exegesis verify books/<slug>/` (it also re-checks everything at the end); note
  the enforced headings. Add a Quality-Red-Line note that `exegesis verify`
  runs the whole mechanical surface in one shot.
- `methodology/01-stage0-adler.md`: note the gate is enforced by `exegesis verify`
  and requires the canonical headings.

______________________________________________________________________

## Go-Advice Refinements Applied

1. **§4 Repetition / §1 layering** — `GatherSkills` moves to `store` so `index`
   and `verify` share it instead of duplicating a tree walk or importing each
   other; the dispatcher stays the only composition root.
2. **§2/§4 return-values-not-errors** — `QualityGate`, `ValidateTestSet`, and
   `RelationshipCountAdvice` return `[]string`/`string`; only the shell turns a
   nonempty result into an error + exit code. Core stays pure.
3. **§5 functional core / imperative shell** — all parsing/gating/heuristics are
   pure functions of their inputs; `verify` is a flat read→gate→report shell. No
   `time.Now`, no globals — output is a deterministic function of the tree.
4. **§4 kill Information Leakage** — one canonical BOOK_OVERVIEW.md format shared
   by `render` (writer) and `ParseBookOverview` (reader), locked by a round-trip
   test — the same discipline that makes gating the hand-authored file
   deterministic (schema, not free-form recovery).
5. **§3 domain errors at the boundary** — leaf failures `Error{Code:EINVALID}`;
   wrapping `Error{Op,Err}`; `fmt.Errorf("…: %w")` only where wrapcheck requires.
6. **§9/§10** — reuse the real renderers in tests (no hand-built fixtures that
   could drift from production output); black-box packages; `t.TempDir`; test
   through the real dispatcher.
7. **Lint-clean by construction** — helpers kept under funlen(60/40)/cyclop(10);
   const→var→type→func ordering; no globals; domain values by slice/pointer.

## Milestones (Each Ends Green: `go build` + `go test -race` + `golangci-lint` 0 Issues)

- **M1** Part A — parser + heuristic + tests.
- **M2** Part B — `store` tree-reading; refactor `cmd/index`; index tests green.
- **M3** Part C — `index` warning.
- **M4** Part D — `verify` command + tests + register.
- **M5** Part E — template rewrite + docs wiring.
- **M6** full-suite verification + end-to-end smoke test; suggest next steps.
