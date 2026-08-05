# exegesis — TODO

`exegesis` is the deterministic pipeline/gate CLI behind the **book2skill** skill:
it distills a book into a tree of Agent Skills and gates each one. Implemented:
`version`, `lint` (+ `--check redlines`), `tests`, `verify` (+ `--gates`,
`--check redlines`), `link`, `index`, and `distill` (agent + http drivers) — the
whole pipeline. It is a pure CLI tool (Pattern B, `ff/v4`):
`main.go` at the root, one command per package under `cmd/`, pure logic under
`internal/`.

## Handoff context

`exegesis` certifies a skill tree's **structure**; the **skillsaw** CLI (via the
`skillsaw-skill`) then optimizes each skill's **quality**. See
`../skillsaw/TODO.md` for the other side of the seam. The two tools share a
`test-prompts.json` JSON contract (below) — the seam is that file plus the
`skills-manifest.json` emitted by `exegesis verify`.

## Seam-closing work (this pass)

- [x] **E1 — Build the pipeline commands at all.** DONE: `lint`, `tests`,
      `verify` implemented as ff/v4 command packages over pure `internal/`
      libraries, with tests. (`distill`, `index`, `link` remain below.)
- [x] **E2 — `exegesis tests --scaffold` also writes `checks` stubs.** DONE:
      `internal/testprompts.DeriveChecks` seeds each case's `checks` from
      `expected` (heading/quoted-name → `section_present`; "≤N chars" →
      `max_chars`; explicit "contains X" → `contains`; etc.), conservative enough
      never to emit a wrong guess.
- [x] **E3 — Fold runtime-neutrality into `exegesis lint`.** DONE:
      `internal/neutrality` uses the byte-identical red-light pattern from
      `skillsaw scan`, surfaced as lint findings.
- [x] **E4 — `exegesis verify` emits `skills-manifest.json`.** DONE:
      `internal/manifest` builds the sorted, machine-readable manifest
      (`structure_verified`, slugs, dirs, test-prompt paths); `verify` writes it.

## The shared `test-prompts.json` contract

One file, read by both tools. Each case carries the activation `type` (exegesis
gates it) **and** optional per-case `checks` (skillsaw's `judge` consumes them):

```json
{"tests": [
  {"id": 1, "type": "should_trigger", "prompt": "...", "expected": "...",
   "checks": [{"op": "section_present", "arg": "Risks"}]}
]}
```

- `type` ∈ `should_trigger` | `should_not_trigger` | `edge_case`.
- Composition gate: ≥3 `should_trigger`, ≥2 `should_not_trigger`, ≥1 `edge_case`.
- `checks` operators mirror skillsaw's judge: `section_present`, `regex`,
  `contains`, `tool_called`, `max_chars`, `min_chars`.

## Remaining pipeline (future passes, out of this pass's scope)

- [x] `exegesis distill` — the resumable, agent-driven pipeline. DONE: `cmd/distill` over a pure
      `internal/distill` core. `run.go`'s `Run` walks an ordered `stages()` sequence (the first stage
      needing prompts stops the round; all satisfied = complete) with shared generic
      `decode[T]`/`writeArtifact` helpers. Stages: `stage0` book → gated `BOOK_OVERVIEW.md` (a gate
      failure re-prompts via a growing content-address so the walk terminates); `stage1` 5 parallel
      extractors → `candidates/<type>.md`; `stage2` construct (assembles candidates with **skillet
      v0.4.0 `ruleset/synthesize`**) → `<slug>/SKILL.md` + `test-prompts.json`; `stage3` deterministic
      `INDEX.md` via the shared `internal/indexgen` (extracted from `cmd/index`, which now delegates
      to it — no drift). Both drivers: `--driver agent` emits prompt batches as JSON and stops;
      `--driver http` answers them itself against an OpenAI-compatible endpoint (`HTTPAnswerer` +
      `RunHTTP` loop, bounded against non-advancing answers) — flags `--endpoint`/`--model`/`--api-key`
      (or `EXEGESIS_API_KEY`). Tests assert the generated tree passes `lint.Check` + `testprompts.Validate`.
      Offloads: `identity.Hash`, `atomicfile`, `overview.Check`, `skill.Slug`, `ruleset/synthesize`,
      `testprompts`, `internal/related`.
- [x] `exegesis index` — regenerate `INDEX.md` (skill list + Mermaid graph +
      dependency-ordered learning path) from each skill's `## Related skills`. DONE (PR #2):
      `cmd/index` over the pure `internal/related` core (parse/serialize + deterministic
      Kahn topo-sort + Mermaid + marker-delimited render). `--check` exits 1 when stale;
      `--title`/`--author` override the `BOOK_OVERVIEW.md`-derived header; sections added
      below the generated block are preserved.
- [x] `exegesis link` — append a related-skill edge to a skill (idempotent). DONE (PR #2):
      `cmd/link` writes via `atomicfile`, idempotent by (kind, target) — a re-link with a
      new rationale updates in place. `--kind` validated against the three known kinds.
      Both commands share `internal/related`; the undifferentiated heavy lifting (skill
      discovery/load/slug, atomic write, title derivation) is offloaded to skillet v0.1.0.
      Would move to `skillet/related` if a 2nd consumer (e.g. skillsaw) appears.
- [x] `exegesis verify --gates overview` — Stage-0 `BOOK_OVERVIEW.md` gate, standalone.
      DONE: `verify` gained a `--gates` selector (comma-separated `overview`/`skills`;
      empty = all, unchanged). `--gates overview` runs only the Stage-0 gate and requires
      `BOOK_OVERVIEW.md` to exist (missing = failure), where the default full run stays
      lenient; an overview-only run writes no manifest. Pure `gateSet`/`parseGates` selector
      over the existing `internal/overview.Check`; no new heavy lifting (overview's markdown
      parse could later offload to `skillet/markdown`, deferred — works, single consumer).
- [x] `exegesis lint --check redlines` — the mechanical Quality Red Lines. DONE: opt-in
      `--check redlines` (or `all`) on both `lint` and `verify`, sharing `lint.ParseCheck`.
      `internal/lint` gains a pure `checkRedlines` enforcing #2 (the six RIA segments R/I/A1/A2/E/B
      are present), #3 (quotations ≤ 150 words/paragraph), and #5 (the description states a trigger
      condition — heuristic); the shell adds #4's presence (test-prompts.json exists). Default
      `lint`/`verify` are unchanged (the red lines are book2skill-specific, so opt-in). Reuses
      `skillet/finding`; base frontmatter checks stay on `skillet/speclint` (not re-checked here).

## Eval-methodology adoptions (from cc-thinking-skills `evals/`)

A survey of `/Users/steve/Documents/git/cc-thinking-skills/evals` (an outcome-based
eval harness) found deterministic pieces worth adopting. exegesis stays the
*structure* tier — no model calls.

- [x] **Adopt-1 — registry-driven word/token budget + required-section lint.**
      DONE: `internal/registry` (Load) + `lint.Options{MaxBodyWords,
      MaxDescriptionWords, RequiredSections}` (opt-in; zero value enforces
      nothing). `lint` gains `--registry` / `--max-body-words` / `--max-desc-words`;
      `verify` gains `--registry` and a catalog check (missing/unexpected skills
      vs `expected_skills`). Required sections match by heading-substring +
      non-empty body, so they fit the RIA sections.
- [x] **Adopt-3 — manifest carries per-skill content SHA-256 (hash-pin).** DONE:
      `skill.Hash` (first-16 of sha256, byte-identical to `skillsaw hash`);
      `manifest.Skill.Hash` (`"sha256"`); `verify` sets it per skill. Verified on
      the nfrs-skills tree (e.g. `"sha256": "578cab7148f300fb"`).

## Deliberately NOT absorbed (kept in the separate outcome tier)

- The model-in-the-loop runners (`run-routing`, `run-objective` live,
  `run-pairwise`, calibration, experiments) + `droid` transport are the outcome
  tier; folding them into exegesis would break the deterministic contract. That
  tier already exists as cc-thinking-skills' harness.
- Heavy stats (cluster bootstrap, Holm, power) + dataset-split leakage validation:
  overkill until exegesis runs many-item evals with variance.

## Process takeaways (adopt as principles, not code)

- "Enforce, don't instruct — put must-happen checks in code, not prose" and
  "prefer deterministic asserts over LLM judges" (their `AGENTS.md`) directly
  endorse what exegesis is; use them as the guardrail against drifting into the
  model tier.

## Cross-repo (shared with skillsaw)

- [x] Extract the `test-prompts.json` + `checks` schema and the agentskills.io
      frontmatter lint into a **shared Go module** so exegesis and skillsaw cannot
      drift. DONE (2026-08-03): the schema was already shared via `skillet/testprompts`
      + `skillet/judge`; the frontmatter spec now lives in `skillet/speclint`
      (`DescriptionMaxRunes`, `AllowedFrontmatterKey`, `Frontmatter`). `internal/lint`
      delegates the frontmatter checks to `speclint` and now emits
      `skillet/finding.Diagnostic` (dropping exegesis's local `Finding` type — closes
      the deferred finding→skillet migration; `--json` output unchanged). skillsaw
      shares the cap via `speclint.DescriptionMaxRunes`.

## Housekeeping

- [x] Replace the root `ShortHelp` "TODO: describe exegesis here" with a real
      description + a `LongHelp` listing subcommands and their status. DONE.
- [x] Make an unknown subcommand exit non-zero with usage. DONE: the dispatcher
      detects a selected group parent (`Exec == nil`) with a leftover positional
      after Parse and returns `"<cmd>: unknown subcommand \"x\""` (exit 1); a bare
      invocation still returns `ff.ErrNoExec` → exit 0.
- [ ] Drop the stale `pgx/v5 + sqlc` note in `RULES.md` (§ near line 648) —
      inherited template boilerplate; exegesis has no database code. `climax lint`
      is otherwise clean. (survey 2026-08-02)

## Cross-repo alignment (2026-08-05 survey)

- [x] Bump skillet **v0.4.0 → v0.5.0** to stay on the shared kernel. DONE (2026-08-05):
      go.mod/go.sum only — no code change, 7 packages test green, `golangci-lint` clean.
      `go mod tidy` added `toerr v0.1.0` as an indirect dep (reached through
      `ruleset/synthesize`). This keeps exegesis on the same `speclint`/`testprompts`
      revision as the rest of the family, guarding the frontmatter/judge drift skillet
      exists to prevent.
- [x] Bump skillet **v0.5.0 → v0.7.0** with the scaffold work. DONE (2026-08-05):
      go.mod/go.sum only — additive across the two minors (calibration, skill ENOTFOUND,
      errs/toerr consolidation); 8 packages test green, `golangci-lint` clean.
- Two offload candidates remain deferred (single-consumer, no action yet): the pure
      `internal/overview` Markdown parse → a future `skillet/markdown` adoption (converge
      on goldmark), and `internal/related` → `skillet/related` if a 2nd consumer
      (skillsaw/canonizer skill-graphs) appears.

## Convenience gaps (from the gemini_skills gap analysis, 2026-08-05)

Source: `~/Documents/agent-orange/gemini_skills/processing/gap_analysis.md` — a
retrospective mapping the Python bulk-processing utilities used during the skill
campaigns against exegesis's native commands. Two genuine gaps; both compose pieces
exegesis already owns rather than adding a new tier.

- [x] **Bulk offline scaffolder — `exegesis scaffold --schema candidates.json --output-dir …`.**
      `distill` (agent-driven, LLM-in-the-loop, slow/costly) and `tests --scaffold` (one
      file's test stubs) exist, but there is no fast **offline** command that takes a
      structured list of candidate skill definitions and, in one pass, writes each skill's
      directory, a RIA-TV++ `SKILL.md` frame (the six R/I/A1/A2/E/B segments `lint --check
      redlines` already enforces), YAML frontmatter honoring `speclint.DescriptionMaxRunes`,
      and a `test-prompts.json` seeded via `testprompts.DeriveChecks` (≥3 `should_trigger`,
      ≥2 `should_not_trigger`, ≥1 `edge_case`). Replaces the external
      `generate_skills_from_schema.py`. Compose what exegesis already has (`skill.Slug`,
      `atomicfile`, `testprompts.Scaffold`/`DeriveChecks`, `speclint`, `lint.checkRedlines`).
      **Closed-loop: verify on write** — run the structural gates over what it emits and
      refuse to leave a failing tree (the "generator executes `verify` on write" the analysis
      calls for).
      DONE (2026-08-05): `cmd/scaffold` over a pure `internal/scaffold` core. `RenderSkill`
      emits the frontmatter + the six RIA-TV++ segment headings + a Related-skills section;
      `BuildTests` seeds via `testprompts.DeriveChecks` (or a `Scaffold` stub). Verify-on-write
      runs `lint.Check{Redlines:true}` + `testprompts.Validate` per skill and `RemoveAll`s any
      newly-created skill that fails (exit non-zero) — so no failing tree is left; existing
      dirs are skipped. Closed-loop test: scaffold → `verify --check redlines` passes.
- [ ] **Centralized / bulk link mapper — `exegesis index --interactive` (or a `relations`
      edge table).** `link` appends one edge at a time and `index` requires every `SKILL.md`
      to already carry a `## Related skills` section, so cold-starting a book of 20+ new
      skills means hand-editing 20 files. Add a single-file relationship mapper: read a
      centralized CSV/JSON edge table (or an interactive checklist) and programmatically write
      the `## Related skills` sections across all skills through the existing `internal/related`
      + `link` write path, then regenerate `INDEX.md` via `index`. Replaces
      `update_index_structure.py`'s book-level indexing convenience.
- Cross-repo note: if skillsaw adopts the closed-loop structural pre-flight (see
      `../skillsaw/TODO.md`), `internal/lint`'s redline/RIA checks become a
      `skillet`-promotion candidate (a `skillet/redlines` or `speclint` extension) so both
      tools gate structure identically instead of skillsaw shelling out to `exegesis verify`.

## Reasoning-toolkit survey (unified-thinking, 2026-08-05)

Source: a survey of `~/Documents/git/unified-thinking` (a deterministic Go reasoning
toolkit). **Lowest relevance of the family** — exegesis is structural gating
(lint/verify/redlines), off-axis from the statistical judgment where that toolkit's
deterministic rigor lives. No meaningful code to lift.

- The one plausible touch, deferred / low priority: a timeseries **regression gate** to
  track distill/gate quality over time (fail on a drop vs. a rolling baseline), modeled on
  unified-thinking's `benchmarks/reporting/timeseries.go` (`DetectRegression`). Its reasoning
  algorithms (Bayesian/causal/fallacy/MCDA) and keyword detectors do not fit exegesis's
  structure tier.
