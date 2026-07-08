# Plan — `index` and `tests` Commands

Two new `exegesis` subcommands surfacing deterministic book2skill mechanics that
SKILL.md currently narrates as manual agent work:

- **`index <book-dir>`** — regenerate `INDEX.md` for a `books/<slug>/` tree from
  the skills' `## Related skills` sections (skill list, Mermaid graph,
  dependency-ordered learning path).
- **`tests <skill-dir>`** — validate/normalize a skill's `test-prompts.json`
  against the structural Phase-4 gate, scaffold a template, and print the darwin
  handoff.

Guiding split (advice §5, §8): **pure core in `internal/book2skill` + `render`;
thin imperative shell in `cmd/*`.** Both commands read/parse files (shell),
transform text↔domain with pure functions (core), and render domain→markdown
with the existing pure `render` package. No new rendering logic — `render.Index`
already exists.

______________________________________________________________________

## Part a — `tests` Command

### A1. Domain Additions — `internal/book2skill/testprompt.go`

The structural gate currently lives **unexported and duplicated** in
`internal/pipeline/stages.go` (`validateTestSet`, `min*` consts). It is domain
knowledge about `TestCase`, so it belongs in the domain package next to
`EncodeTestPrompts`, following the established `BookOverview.QualityGate() []string`
idiom (advice §2, §4: pull the decision down to one owner; remove Information
Leakage / Repetition).

Add:

```go
// exported thresholds — single source of truth (D4 structural gate)
const (
	MinShouldTrigger    = 3
	MinShouldNotTrigger = 2
	MinEdgeCase         = 1
)

// CountByType tallies cases by TestType. Pure.
func CountByType(cases []TestCase) map[TestType]int

// ValidateTestSet returns the reasons cases fail the structural Phase-4 gate;
// an empty slice means the set passes. Mirrors BookOverview.QualityGate.
func ValidateTestSet(cases []TestCase) []string

// DecodeTestPrompts parses the darwin-shaped bare JSON array. Inverse of
// EncodeTestPrompts.
func DecodeTestPrompts(data []byte) ([]TestCase, error)

// TemplateTestCases returns a minimal scaffold that satisfies the gate
// (MinShouldTrigger + MinShouldNotTrigger + MinEdgeCase placeholder cases).
func TemplateTestCases() []TestCase
```

`ValidateTestSet` returns **all** failing reasons (not just the first) so the
command can report everything at once — better than the pipeline's
first-failure-only behaviour.

### A2. Pipeline Refactor — `internal/pipeline/stages.go`

Delete the local `min*` consts and reimplementation; delegate:

```go
func validateTestSet(cases []book2skill.TestCase) error {
	if problems := book2skill.ValidateTestSet(cases); len(problems) > 0 {
		return testSetError(problems[0])
	}
	return nil
}
```

Rewrite `testResultsDoc`'s manual counting to use `book2skill.CountByType`
(removes the third copy of the by-type tally). Behaviour unchanged; existing
pipeline test still passes.

### A3. Command — `cmd/tests/tests.go` (Shell)

`exegesis tests [FLAGS] <skill-dir>` operating on `<skill-dir>/test-prompts.json`:

| Flag                  | Effect                                                                                       |
| --------------------- | -------------------------------------------------------------------------------------------- |
| `--scaffold`          | if the file is absent, write `TemplateTestCases`; error if it already exists (don't clobber) |
| `--fix`               | rewrite the existing file in canonical `EncodeTestPrompts` form                              |
| `--format text\|json` | report format (default text)                                                                 |

Flow: read → `DecodeTestPrompts` → `ValidateTestSet` → report counts + pass/fail

- the `darwin evolve <skill-dir>/` handoff line. Gate failure → `root.ExitError(1)`
  (same convention as `lint`). Leaf errors are `book2skill.Error{Code: EINVALID}`.

______________________________________________________________________

## Part B — `index` Command

### B1. Domain Additions — Pure Parsers (Inverse of `render`)

`render` writes SKILL.md / BOOK_OVERVIEW.md; regenerating INDEX.md means reading
them back. `render.Index` needs only: `BookOverview.{Title,Author}` and each
`Skill.{Slug,Title,Related}`. So we parse just those — **no YAML dependency**:
the slug is the directory name (skilllint already enforces `name == dir`), the
title is the `#` heading, relationships come from `## Related skills`.

`internal/book2skill/skillmd.go` (already owns SKILL.md structural parsing —
`ParseSegments`, `SegmentTagFromHeading`):

```go
// ParseTitle returns the text of the first level-1 heading ("# ...") in md.
func ParseTitle(md string) string

// ParseRelated parses the "## Related skills" section into relationships whose
// From is set to fromSlug. It is the inverse of render.renderRelated; invalid
// kinds are skipped.
func ParseRelated(fromSlug, md string) []Relationship
```

`internal/book2skill/overview.go`:

```go
// ParseOverviewHeader extracts the title and author from a rendered
// BOOK_OVERVIEW.md ("# <Title> — Book Overview", "- **Author:** <author>").
func ParseOverviewHeader(md string) (title, author string)
```

### B2. Reduce Format Coupling (Advice §4 — Information Leakage)

`render.renderRelated` and `ParseRelated` share the `## Related skills` heading
and item shape. Move that heading text to one shared constant in `book2skill`
used by both writer and parser. The item shape (`- <kind>: \`<to>\` — <rationale>`) is matched by a tolerant regexp in `ParseRelated`, documented as the inverse of `renderRelated`. A **round-trip test** (`render.Skill\` → parse → equal Related)
locks the two ends together so drift is caught.

### B3. Command — `cmd/index/index.go` (Shell)

`exegesis index [FLAGS] <book-dir>`:

| Flag                   | Effect                                                                |
| ---------------------- | --------------------------------------------------------------------- |
| `--title` / `--author` | override the BOOK_OVERVIEW.md-derived header                          |
| `--check`              | compare against the existing `INDEX.md`; exit 1 if stale, don't write |

Flow: read `BOOK_OVERVIEW.md` (if present) + flag overrides → list immediate
subdirs containing `SKILL.md`, **sorted by name** (deterministic) → for each,
`ParseTitle` + `ParseRelated(slug, …)` → build `[]book2skill.Skill` →
`render.Index` → write `<book-dir>/INDEX.md` (or compare under `--check`).
Fallbacks: title = dir base name, author = "" when no overview and no flags.

______________________________________________________________________

## Part C — Wiring & Tests

- Register both in `cmd/cmd.go` (`index.New(r)`, `tests.New(r)`) — order = help
  output order.
- **Core tests (`book2skill`, black-box):** `ValidateTestSet` table (pass + each
  failure), `DecodeTestPrompts` round-trip with `Encode`, `TemplateTestCases`
  passes the gate, `ParseTitle`, `ParseRelated`, `ParseOverviewHeader`.
- **Round-trip test (`render`):** `render.Skill(skill)` parsed back recovers
  Title + Related — the anti-drift lock for B2.
- **Command tests (`cmd`, black-box, via `cmd.Run`):** one happy path + one
  failure per command over a `t.TempDir()` tree with injected buffers — this is
  the integration I'd fear breaking (advice §5: test what you fear; §10: stdlib
  only, `t.TempDir`, no network).

______________________________________________________________________

## Go-Advice Refinements Applied to This Plan

1. **§2/§4 — one owner for the gate.** The Phase-4 thresholds move from
   `pipeline` (where they were duplicated three ways) to `book2skill` as the
   single source; `pipeline` and the new command both consume it. Removes
   Repetition + Information Leakage.
2. **§4 — return-values-not-errors for validation.** `ValidateTestSet` /
   `QualityGate` return `[]string` problems; only the shell turns a nonempty
   result into an `Error` + exit code. Keeps the core pure and side-effect-free.
3. **§5 — functional core / imperative shell.** All parsing/validation/rendering
   is pure (string↔value); the commands are flat read→transform→write shells.
   No `time.Now`, no globals, no hidden state — the output is a deterministic
   function of the tree, which also makes it golden-testable.
4. **§4 — kill Information Leakage between render and parse** via a shared
   heading constant + a round-trip test, rather than letting the SKILL.md format
   decision live independently in two modules.
5. **§1 — no YAML in the domain package.** Recover the slug from the directory
   name (contract already enforced by `lint`) instead of adding a frontmatter
   parser to `book2skill`; keeps the root domain package free of third-party
   imports.
6. **§3 — domain errors at the boundary.** Leaf failures are
   `book2skill.Error{Code: EINVALID}`; wrapping uses `Error{Op, Err}`; only
   `fmt.Errorf("…: %w", …)` where `wrapcheck` requires it.
7. **§10 — black-box tests, stdlib only, `t.TempDir`.** No test-only seams; the
   commands are tested through the real `cmd.Run` dispatcher, which also
   verifies registration.
8. **Lint-clean by construction.** Keep every `exec`/helper under
   funlen (60/40) and cyclop (10) by extracting helpers; consts→vars→types→funcs
   ordering (`decorder`); no globals (`gochecknoglobals`); domain structs passed
   by slice/pointer, never large-by-value (`gocritic hugeParam`).

## Milestones (Each Ends At: `go build` + `go test -race` + `golangci-lint` = 0 Issues)

- **M1** A1 domain additions + tests.
- **M2** A2 pipeline refactor (pipeline test still green).
- **M3** A3 `tests` command + command test + register.
- **M4** B1/B2 parsers + shared constant + round-trip test.
- **M5** B3 `index` command + command test + register.
- **M6** full-suite verification; suggest next steps.
