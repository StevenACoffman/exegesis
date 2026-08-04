# exegesis — TODO

`exegesis` is the deterministic pipeline/gate CLI behind the **book2skill** skill:
it distills a book into a tree of Agent Skills and gates each one. Implemented
today: `version`, `lint`, `tests`, `verify`, `link`, `index`. `distill` and the
two flag-gates below remain. It is a pure CLI tool (Pattern B, `ff/v4`):
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

- [ ] `exegesis distill` — the agent-driver loop (Stage 0→4) that emits prompts as JSON and
      resumes from a content-addressed cache. Largest piece; being built in phases.
      **Done (`--driver agent`, Stages 0–1):** `cmd/distill` over a pure `internal/distill` core.
      `protocol.go` (Message/PromptRequest/Outcome), `cache.go` (content-addressed response store;
      presence = answered), `run.go` (a generic `Run` that walks an ordered `stages()` sequence:
      the first stage needing prompts stops the round; all satisfied = complete — plus shared
      generic `decode[T]`/`writeArtifact` helpers). `stage0.go` = book → gated `BOOK_OVERVIEW.md`
      (a gate failure re-prompts via a growing content-address so the walk always terminates).
      `stage1.go` = 5 parallel extractor prompts (frameworks/principles/cases/counter-examples/
      glossary) → `candidates/<type>.md` (emits the whole batch; writes each file as its prompt is
      answered). Offloads `identity.Hash`, `atomicfile.WriteFile`, `internal/overview.Check`,
      `skill.Slug`.
      **Still to build:** Stage 1.5/2 (triple-verify → `rejected/`; RIA++ construct →
      `<slug>/SKILL.md`) — **bump to skillet v0.4.0** and offload to `ruleset/distill` +
      `ruleset/synthesize` (`LoadInputs`/`FillTemplate` on `{{RULESETS}}`); Stage 3 (deterministic
      link/index via `internal/related`); Stage 4 (`testprompts` scaffold + `tests`/`verify` gating
      + `manifest`); and `--driver http` behind a `Driver` seam. New stages slot into `stages()`.
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
- [ ] `exegesis verify --gates overview` — Stage-0 `BOOK_OVERVIEW.md` gate
      (implemented this pass as part of `verify`; standalone flag still TODO).
- [ ] `exegesis lint --check redlines` — the mechanical Quality Red Lines.

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
