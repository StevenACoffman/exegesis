# Plan — migrate merge-skills mechanics into exegesis

Move the deterministic mechanics of `merge-skills` behind `exegesis`, leaving only
convergence/synthesis judgment to the agent. Scope = my recommended order items
1–5 (prereqs, `tests --merge`, `merge-status`, `verify --merge`, `quotecheck`).
The cross-book `merge-index` renderer (item 6) is deferred (documented next step).

Split (advice §1, §2, §5): pure domain types + validation + text parsing in
`book2skill` (no third-party imports); YAML/markdown surgery in a new
`internal/mergedoc` adapter (imports `book2skill` + `gopkg.in/yaml.v3`); thin
imperative-shell commands in `cmd/*`; reuse `store`, `render`, `skilllint`,
`verify`'s gate structure.

______________________________________________________________________

## M1 — Prerequisites

1. **`SupersededBy` relationship kind** — `book2skill.RelationshipKind`:
   add `SupersededBy = "superseded-by"` and the `Valid()` arm. `render.renderRelated`
   and `ParseRelated` already handle arbitrary kinds; `LearningOrder` only follows
   `DependsOn`, so a superseded-by edge correctly does not affect learning order.
2. **Fix `merge-skills/templates/test-prompts.json.template`** to the bare darwin
   array with the 4th type (mirror the book2skill fix; it's `.json`, untouched by
   the formatters).
3. **Wire `exegesis lint` into `merge-skills/SKILL.md`** — replace the `uvx
   skillcheck` gate and drop the generics false-positive note (mirror book2skill).

## M2 — Merge test-set validation (`book2skill`, pure)

`testprompt.go`:

- `PreferMergedOverSource TestType = "prefer_merged_over_source"` + `Valid()` arm.
- Merge thresholds: reuse `MinShouldTrigger` (3), `MinShouldNotTrigger` (2); add
  `MinMergedEdgeCase = 2`, `MinPreferMerged = 2` (the "identify ≥2 scenarios or
  auto-dissolve" rule).
- `ValidateMergedTestSet(cases []TestCase) []string` — returns all failing reasons
  (mirrors `ValidateTestSet`); enforces the four minimums. Runtime pass-rate stays
  with darwin.
- `TemplateMergedTestCases() []TestCase` — 3 trigger + 2 decoy + 2 edge + 2
  prefer-merged placeholders (passes the gate).

Tests: table for pass + each failing category; `TemplateMergedTestCases` passes.

## M3 — `tests --merge` (`cmd/tests`)

- `--merge` flag. When set: `ValidateMergedTestSet` instead of `ValidateTestSet`;
  `--scaffold --merge` writes `TemplateMergedTestCases`; report includes the
  prefer-merged count.
- Keep `exec` thin; branch the validator/template by `cfg.Merge`.

Tests (via `cmd.Run`): scaffold --merge then validate passes; a 3-category set
fails under `--merge` (missing prefer-merged).

## M4 — Merge-status ledger (domain + adapter)

**`book2skill` (pure, no yaml import — struct tags are inert):**

```go
type MergeState string   // no-candidate|surface-resemblance|complementary|rejected|partial|merged
type MergeReason string  // source-text-unavailable|source-verification-failed|v1..v4 codes
type MergeStatusEntry struct {
    Run string `yaml:"run"`; State MergeState `yaml:"state"`
    Pair, Into string `yaml:"...,omitempty"`; Reason MergeReason `yaml:"reason,omitempty"`
    Excluded string `yaml:"excluded,omitempty"`
}
func (MergeState) Valid() bool; func (MergeReason) Valid() bool
func (e *MergeStatusEntry) Validate() []string   // vocab + per-state required-field rules
```

Per-state field rules (from SKILL.md): `pair` required for surface-resemblance /
complementary / rejected; `into` required for merged / partial; `reason` required
(and valid) for rejected; `excluded` required for partial; `run` always required.

**`internal/mergedoc` (imports book2skill + yaml.v3):**

```go
const StatusHeading = "Merge Status"
func Parse(md string) ([]book2skill.MergeStatusEntry, error)          // read the ## Merge Status yaml fence
func Append(md string, e book2skill.MergeStatusEntry) (string, error) // append-only; create section+fence if absent
```

`Append` is append-only by construction (parse existing → append → re-marshal the
whole list → splice the fence in place, or append the section at EOF if absent);
the "remove/overwrite an entry" failure mode is defined out of existence. Locating
the section is case-insensitive (title-cased `## Merge Status`).

Tests: parse a sample block; append creates the section when absent and preserves
prior entries when present; round-trip Parse(Append(...)); `Validate` table.

## M5 — `exegesis merge-status` (`cmd/mergestatus`)

- `merge-status append <skill-dir> --run <slug> --state <state> [--pair --into
  --reason --excluded]` — build entry, `Validate()`, then `mergedoc.Append` and
  write. Refuses invalid entries (leaf `EINVALID`).
- `merge-status check <tree>` — walk skills (`store.GatherSkills`), `mergedoc.Parse`
  each, `Validate` every entry; report problems; `ExitError(1)` on any.
- Register in `cmd/cmd.go`. Tests via `cmd.Run`: append then check passes; an
  invalid `--state` is rejected; check flags a hand-written bad block.

## M6 — `exegesis quotecheck` (`book2skill` pure + `cmd/quotecheck`)

**`book2skill`:**

- `RSegmentQuotes(body string) []string` — extract the R segment
  (`ParseSegments`), collect blockquote (`>`-prefixed) lines, group blank-line-
  separated blocks into quotes, strip markers and normalize whitespace. Pure.
- `QuoteFound(quote, source string) bool` — whitespace-normalized substring match.
  Pure.

**`cmd/quotecheck <skill-dir> --source <a.txt> [--source <b.txt>]`:** read the
skill body + each source text; for every R quote, flag if found in no source.
Verbatim/normalized presence is mechanical; paraphrase distance stays the agent's
judgment (documented). TXT only. `ExitError(1)` if any quote is unlocated.

Tests: `RSegmentQuotes` extraction (single + dual citation); `QuoteFound`
whitespace tolerance; command pass/fail via `cmd.Run`.

## M7 — `verify --merge` (`cmd/verify`)

Add a `--merge` flag selecting a merged-tree gate set:

- **overview** → skip the `book2skill` BOOK_OVERVIEW gate; instead check
  `MERGE_OVERVIEW.md` presence (its content is judgment).
- **lint** → unchanged (per-skill spec/quality/redlines).
- **tests** → use `ValidateMergedTestSet`.
- **merge-status** → new gate: `mergedoc.Parse` + `Validate` every block.
- **index** → skip (cross-book INDEX is `merge-index`, deferred).

Reuse the `gateOutcome` structure; keep gate helpers small. Tests via `cmd.Run`:
a consistent merged tree passes; a bad merge-status block or undersized merged
test-set fails.

## M8 — Wire merge SKILL.md + reformat

Replace hand-narrated mechanics in `merge-skills/SKILL.md` (+ methodology) with
CLI calls: Phase 4 → `exegesis tests --merge --scaffold` / `exegesis tests
--merge`; Phase 1.5 → `exegesis quotecheck`; Source Skill Annotation → `exegesis
merge-status append`; a closing `exegesis verify --merge books/merged/<slug>/`;
Quality-Red-Lines note. Then run `mdformat --wrap keep --number` + `rumdl fmt`
(exegesis configs) over merge-skills.

## M9 — Verification

`go build ./...` clean · `go test -race ./...` green · `golangci-lint run ./...`
0 issues (no rules relaxed) · a merge-tree e2e (`tests --merge`, `merge-status
append`+`check`, `quotecheck`, `verify --merge`) · formatters idempotent on the
edited docs. Then suggest next steps (merge-index; commit).

______________________________________________________________________

## Go-advice refinements applied

1. **§1/§2 layering — keep `book2skill` third-party-free.** YAML lives only in the
   new `internal/mergedoc` adapter; `book2skill` holds the merge-status *types* +
   `Validate()` with inert `yaml:` struct tags (same as its existing `json:` tags,
   which use only stdlib `encoding/json`). No cross-subpackage imports; the
   dispatcher stays the composition root.
2. **§2/§4 return-values-not-errors.** `Validate`, `ValidateMergedTestSet`,
   `RSegmentQuotes` return values/`[]string`; only the command shells turn a
   non-empty result into an `Error` + exit code. Core stays pure and side-effect-free.
3. **§4 model constraints in types.** `MergeState`/`MergeReason`/`TestType`/
   `RelationshipKind` are string enums with `Valid()`; `MergeStatusEntry.Validate`
   encodes the per-state field rules once, so no caller re-checks them.
4. **§3 define errors out of existence.** `mergedoc.Append` is append-only —
   there is no "overwrite/remove entry" path to fail. Leaf failures are
   `Error{Code:EINVALID}`; wrapping is `Error{Op,Err}`; `fmt.Errorf("…: %w")` only
   for the external `yaml` error at the adapter boundary.
5. **§5 functional core / imperative shell.** Parsing (`RSegmentQuotes`,
   `mergedoc.Parse`), validation, and matching are pure; commands are flat
   read→compute→write shells. No globals, no `time.Now` — output is a deterministic
   function of inputs.
6. **§4 reuse, don't duplicate.** `verify --merge` reuses `gateOutcome`;
   `merge-status check` and `verify` reuse `store.GatherSkills`; quote extraction
   reuses `ParseSegments`; section location reuses the case-insensitive scan.
7. **Formatter tolerance.** All new heading/section matching is case-insensitive
   (title-cased `## Merge Status`, `## Related Skills`, `### Language Signals`);
   generated JSON/markdown stays a formatter fixed point where compared.
8. **§9/§10 tests.** Black-box packages, `t.TempDir`, stdlib only, exercised
   through the real `cmd.Run` dispatcher; no test-only seams.

## Milestone checkpoints

Each milestone ends green: `go build` + `go test -race` (touched packages) +
`golangci-lint run` = 0 issues, improving per go-advice before moving on.
