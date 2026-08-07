# exegesis — TODO

`exegesis` is the deterministic pipeline/gate CLI behind the **book2skill** skill:
it distills a book into a tree of Agent Skills and gates each one. Implemented:
`version`, `lint` (+ `--check redlines`), `tests` (+ `--scaffold`, `--merge`,
`--migrate`), `verify` (+ `--gates`, `--check redlines`), `link`, `relate`,
`index`, `normalize`, `scaffold`, `quotecheck`, `merge-status` (`append`/`check`),
and `distill` (agent + http drivers) — the whole pipeline. Pinned to
`skillet v0.11.0`. It is a pure CLI tool
(Pattern B, `ff/v4`):
`main.go` at the root, one command per package under `cmd/`, pure logic under
`internal/`.

## Handoff context

`exegesis` certifies a skill tree's **structure**; the **skillsaw** CLI (via the
`skillsaw-skill`) then optimizes each skill's **quality**. See
`../skillsaw/TODO.md` for the other side of the seam. The two tools share a
`test-prompts.json` JSON contract (below) — the seam is that file plus the
`skills-manifest.json` emitted by `exegesis verify`.

The structural rules themselves are shared as code, not just as files:
`skillet/speclint` holds the agentskills.io frontmatter spec and
`skillet/redlines` — promoted out of this repo's `internal/lint` once skillsaw
became a second consumer — holds the Quality Red Lines. exegesis gates a tree on
them; skillsaw rejects an edit on them. Neither can drift by hand.

## Seam-closing work (2026-08-03, complete)

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

### The merge-skills CLI surface (documented, not built — audited 2026-08-06)

`~/Documents/agent-orange/steve-skill-market/skills/merge-skills/SKILL.md` instructs agents
to run four exegesis subcommands, three flags and one edge kind. As of 2026-08-07
`quotecheck` and `tests --merge`/`--migrate` exist; `merge-status`, `merge-index`,
`a2check`, `verify --merge` and the `superseded-by` edge kind still do not, so an agent
following that skill still fails at those steps. The skill is deliberately left as-is; it
reads as the spec for this work.

Nothing else in the exegesis pipeline depends on these — book2skill's flow is unaffected.

- [x] **`exegesis quotecheck --source-text a.txt,b.txt SKILL_DIR`** — the fabrication guard. DONE (2026-08-07, skillet v0.11.0).
      Locates each `R`-section quote in the supplied plain-text sources and flags any found in
      **none** of them (`MISS`). Source must be plain text (EPUB/PDF extracted first). Judging
      paraphrase distance on the rest stays the agent's.
      Uses `redlines.Quotes` rather than a second extractor, so the guard and the
      quotation-length red line cannot disagree about what a quotation is.
      **The spec said "each R-section quote"; measuring the corpus changed the design.**
      95% of the 179 skills with R quotations have the whole segment as a *single*
      blockquote, median 860 characters. Matching that as one unit means any single
      editorial difference condemns the entire segment and says nothing about where the
      problem is — and one real skill demonstrated exactly that. Matching is therefore per
      *passage* (sentence-sized, `MinPassageWords` = 6), which also catches the case that
      matters most: one invented sentence inside an otherwise faithful quotation.
      Both sides are normalized — whitespace collapsed, curly quotes/dashes/ellipses folded
      to ASCII — because a quotation is line-wrapped in Markdown and its source is not; a
      literal comparison reports every quotation missing and nobody runs the guard.
      Verified on a real skill: faithful source 18/18 located, exit 0; one sentence deleted
      from the source names that exact sentence MISS, 17/18, exit 1.
      Markdown tables inside an R blockquote (1% of skills) will still MISS — they are the
      author's restructuring, not verbatim source text, so that is arguably correct.
- [x] **`exegesis merge-status append|check`** — the per-source-skill merge ledger. DONE (2026-08-07), except `--link`.
      `append --run <slug> --state <state> [--pair --into --reason --excluded] SKILL_DIR` is
      append-only by construction and validates a closed state/reason vocabulary plus the
      per-state required fields; `check <dir>` validates every ledger under a tree. The skill
      says "do not hand-edit the block", which is impossible without this.
      **Append-only is a property of the code, not a rule the caller is asked to respect:**
      a new entry is spliced in as text ahead of the closing fence, so prior entries are
      copied through byte for byte and an append cannot reformat, reorder or lose an
      earlier run's record even if the rendering here later changes. Tested against a prior
      entry written in a style this package would never emit.
      `Validate` rejects a field the state has no use for as well as one it needs and lacks:
      a `rejected` entry naming what it merged `into` is not a harmless extra, it is two
      contradictory accounts of one decision in a file kept as evidence.
      **Heading case:** written as title-case `## Merge Status` because `rumdl` MD063
      rewrites a lowercase heading, and read case-insensitively so a ledger written either
      way is found — a reader seeing one spelling would call a populated ledger absent and
      then append a second section below the first. Verified the written form adds zero
      rumdl issues, and that `exegesis lint` still passes on a skill carrying a ledger.
      The first two-level command in this CLI (`merge-status <append|check>`), matching the
      syntax merge-skills/SKILL.md already tells agents to run.
      **`--link` is NOT built**: it writes a `superseded-by` bullet, and that edge kind is
      still an open decision below. The flag is accepted and returns a usage error saying
      so, rather than failing as an unknown flag.
- [ ] **DECIDE FIRST: which provenance model is authoritative.** `merge-index` cannot be
      built until this is settled, because the spec and the only real merged tree describe
      two different data models and the spec's has zero instances on disk (surveyed
      2026-08-07 against `~/Documents/agent-orange/books/merged/all-books-v1`).

      | what `merge-skills/SKILL.md` says merge-index reads | found on disk |
      | --- | --- |
      | `## Merge Status` ledgers in source skills | **0** across all of `books/` |
      | `source-verification/<pair-id>-{r,a1}.md` headers | directory exists, **empty** |
      | `superseded-by` edges | edge kind still undecided |

      What the tree actually has: 27 merged skills carrying provenance in **frontmatter** --
      `source_skills` (slug/book/author) and `related_skills` with `relation:` -- and
      `relation` uses `supersedes` (54) and `composes-with` (1). Note `supersedes` is the
      *forward* direction recorded on the merged skill, while the spec's `superseded-by` is
      the *inverse* recorded on the source skill: different name, different direction,
      different file.

      **All 27 fail `exegesis lint`** -- `id`, `title`, `type`, `source_skills`,
      `related_skills` are disallowed frontmatter keys and none carries `name`, so
      `name "" != folder` too. The spec is explicit that `merge_status` goes in the *body*
      precisely because frontmatter would fail lint; the tree puts more in frontmatter, not
      less. Only 9 of 27 have a body `## Related Skills` section at all.

      The hand-written `INDEX.md` also does not match the spec's section list: it has
      Cross-Book Provenance Table, Mermaid Provenance Graph, Cluster Summary, Rejected
      Pairs and Rejected Pair Cross-References, and **no** Source Verification Summary. Its
      **Cluster column has no machine source** -- `cluster` appears in zero files, so that
      column is editorial judgment that no generator can reproduce. `rejected/pair-NNN.md`
      are free-form prose with bold labels, not structured records.

      **The four decisions:**
      1. Is `all-books-v1` governed by exegesis at all, or is it an artifact of an earlier
         pipeline that should be left alone?
      2. If governed: does merged-skill frontmatter get an allowlist exemption in
         `speclint`, or does the provenance move into the body?
      3. Is the Cluster column kept? If so it needs a source -- a `cluster:` key, or a
         config file the generator reads.
      4. Does the spec's ledger model survive, now that `merge-status append|check` exists
         to write it, or is frontmatter `source_skills` the real contract?

      Until these are answered, building `merge-index` produces a command that reads
      nothing that exists and cannot regenerate the INDEX that is actually there.
- [ ] **`exegesis merge-index MERGED_TREE`** — INDEX for a merged tree. Reads the YAML headers
      of `source-verification/<pair-id>-{r,a1}.md` to build a "Source Verification Summary"
      table (its V1–V4 column comes from the ledgers), so each verdict is written once instead
      of hand-maintained. Distinct from `index`, which knows nothing about merge provenance.
      **Header reading is unblocked upstream (needs the skillet bump).** Use
      `frontmatter.Split(text) (block, body)` and unmarshal `block` yourself — do **not**
      write a second frontmatter parser. It was promoted out of `skill.splitFrontmatter` for
      exactly this call site, and it handles the cases a fresh implementation gets wrong: an
      empty header, an unterminated one, and CRLF (normalized inside `Split`, so this caller
      must not normalize first). `skill.Load` is no use here — it takes a *directory* and
      appends `SKILL.md`.
      The header shape (the key is `pair`, not `pair-id` -- an earlier note here had it
      wrong; `<pair-id>` is the *value* it holds):

      ```go
      type header struct {
          Pair    string `yaml:"pair"`
          Check   string `yaml:"check"`   // r-quote-accuracy | a1-attribution
          Sources []struct {
              Book      string `yaml:"book"`
              Skill     string `yaml:"skill"`
              Status    string `yaml:"status"`    // accurate|drifted-minor|drifted-major|not-found
              Corrected bool   `yaml:"corrected"` // A1: verified|mismatch|not-found
          } `yaml:"sources"`
      }
      ```

      **Blocked on the decision above, not on `merge-status`** (which now exists). The
      V1-V4 column reads the ledgers, and no skill has one. Note the ledger is a markdown *section*,
      not frontmatter — `merge_status` is not a spec-allowed frontmatter key and would fail
      lint — so `frontmatter.Split` does not help there.
- [ ] **`exegesis a2check --source-skill A,B MERGED_SKILL_DIR`** — the A2-sharpness gate: the
      merged `A2` must carry ≥2 language signals neither source has. Advisory by default,
      `--strict` to fail. Counting is structural; whether the signals are semantically
      distinct stays the agent's.
- [x] **`exegesis tests --merge` and `--migrate`.** DONE (2026-08-07, skillet v0.11.0). `--merge` enforces a **four**-category gate
      (≥3 `should_trigger`, ≥2 `should_not_trigger`, ≥2 `edge_case`, ≥2
      `prefer_merged_over_source`) rather than the current three, exiting non-zero until it
      passes; `prefer_merged_over_source` is the quality gate unique to merged skills.
      `--merge` builds its Composition here, where the policy belongs; skillet deliberately
      does not know what merging is. The tally is composition-driven rather than a fixed
      list of three, so a gate that gains a category cannot leave the display behind — the
      defect skillet's `Tally` had.
      **Caught by comparing against the previous binary over 183 real skills:** making the
      tally composition-driven had silently reordered the columns alphabetically, so
      `edge_case` led instead of `should_trigger`. Fixed with an explicit display order —
      the three standard categories in their usual progression, added categories after —
      and the test now asserts the whole line, not three substrings, which is why the
      reshuffle slipped past the first version of it. Output is now byte-identical to
      v0.10.0 across all 183 skills.
      **`--migrate` covers the shapes skillet's reader accepts** — bare array, `test_cases`,
      `expected_behavior`, string and missing ids — reporting every change and leaving a
      canonical file untouched. It is idempotent. The foreign shapes the item also listed
      (`prompts`/`test_prompts` keys, category-grouped arrays, type synonyms, other fields
      preserved in `notes`) need a new tolerant reader here and were never blocked on
      skillet; they remain open.
      **It refuses rather than migrates when a file carries both `tests` and `test_cases`:**
      the reader keeps `tests` and drops the rest, so rewriting deletes cases still on disk.
      That check re-reads the two keys instead of pattern-matching `File.Rewrites`, because
      deciding whether to destroy data on a human-readable string would break the first
      time that wording changed.
      `--migrate` adopts a foreign `test-prompts.json` (object wrapper, `prompts`/
      `test_prompts` keys, category-grouped arrays) into canonical form: `expected*` variants,
      type synonyms, renumbered ids, other fields preserved in `notes`.
- [ ] **`exegesis verify --merge [--source-book A,B] [--strict]`** — one pass over a merged
      tree: `MERGE_OVERVIEW.md` presence, per-skill lint, and the `tests --merge` gate;
      `--source-book` additionally runs the a2check advisory across every merged skill.
- [x] **`exegesis normalize` and `rumdl` fought over the `## Related skills` heading.** DONE (2026-08-07).
      Found while deciding the ledger's heading case (2026-08-07). `related.sectionHeading`
      is lowercase `## Related skills` and `normalize` rewrites the heading to it;
      `rumdl` MD063 then flags that and `rumdl fmt` rewrites it straight back to
      `## Related Skills`. Each tool undoes the other on the same line.
      **The corpus has already picked a side: 179 skills use `## Related Skills`, zero use
      lowercase.** So `normalize` is the odd one out — running it across the tree today
      would flip all 179 to a form rumdl rejects.
      Reading was already safe (`isSectionHeading` uses `EqualFold`), so this was a
      write-side fix only. `sectionHeading` is now `## Related Skills`.
      **Measured on the real corpus, old binary against new:** normalize used to leave
      179 lowercase / 7 title case; it now leaves 179 title case / 0 lowercase, matching
      what was already on disk. The 19 files whose heading text it still rewrites are the
      `(Stage 3 Filling)` suffixed variants being canonicalised, which is its job. A
      normalized file used to trip MD063 and no longer does.
      **The same rule applied to every other heading exegesis writes.** Six more were
      flagged, all in `distill`'s Stage-0 overview: `One-sentence summary`, `Key terms`,
      `Core propositions`, `Era limitations`, `Author blind spots`,
      `Unproven assumptions`. All now title case; safe because the overview gate lowercases
      before matching (`overview.headingKey`), so a file written either way still passes.
      The gate's own message was updated to name the form it wants written.
      **Two flagged headings were deliberately left alone:** `# <book> — Book Overview` and
      `# <type> candidates` are flagged only on their *interpolated* portion — a book title
      and a type name. Case-folding data to satisfy a linter would be a worse defect than
      the lint it silenced, so the rule applied is "title-case static heading words, never
      touch interpolated identifiers".
      Test inputs deliberately keep the lowercase spellings: they are now the coverage
      proving the readers stayed case-insensitive.
- [ ] **A `superseded-by` edge kind — decide, then build or reject.** merge-skills links a
      source skill to the merged skill that replaced it
      (`link --kind superseded-by --to <merged-slug> books/<slug-a>/<source-skill>/`), but
      `related.Kind` admits only `depends-on`, `contrasts-with` and `composes-with`, so that
      call is rejected today. This is the smallest of the merge-surface items and the one
      that touches the most shared code, so decide it before the rest.

      **What a fourth kind gets for free.** Adding the constant and a `Valid()` case is
      enough for most of the pipeline: the dialect reader accepts any `Valid()` kind, dedup
      keys on `(kind, target)`, the report sort orders by kind, and `prereqs` filters on
      `DependsOn` alone, so a supersession correctly does **not** reorder the learning path.

      **Pros**
      - The relationship is genuinely a skill-to-skill edge, which is exactly what the
        section models. Recording it anywhere else splits one concept across two mechanisms.
      - `index` renders it in the Mermaid graph automatically (`edgeLines` emits every known
        kind), which is what merge-skills wants: a cross-book graph showing supersession.
      - `link`/`relate`/`normalize` all work on it with no change, since none of them
        special-cases a kind.
      - It makes a dead skill self-describing: a reader who opens the superseded
        `SKILL.md` sees where to go, without consulting a ledger elsewhere.

      **Cons**
      - **It points across trees, and the graph gate is per-tree.** The source skill lives in
        `books/<slug-a>/`, the merged skill in `books/merged/<merge-slug>/`. `DanglingEdges`
        resolves targets against `indexgen.CollectNodes(tree)` for one tree, so every
        `superseded-by` edge would be reported by `verify` as pointing at a skill that does
        not exist. That is the real blocker, and it is not cosmetic — the graph gate exists
        precisely to catch that shape. Fixing it means one of: exempting this kind from the
        gate (weakens the gate), teaching the gate about sibling trees (new concept: a
        multi-tree resolution root), or writing the target as a qualified reference rather
        than a bare slug (changes the wire format for every kind).
      - **It duplicates the merge ledger.** The `## Merge Status` block already records
        `into: <merged-skill-slug>` for `merged`/`partial` states, and merge-skills says
        `merge-status append --link` writes both in one call. Two records of one fact drift;
        the ledger is the more expressive of the two (it also carries `state`, `reason`,
        `excluded`).
      - **The semantics differ from the other three.** `depends-on`, `contrasts-with` and
        `composes-with` relate skills that both live. `superseded-by` is terminal — it says
        "this one is dead." That likely implies more than an edge: should a superseded skill
        still appear in the skill list, the learning path, or the `expected_skills` catalog?
        None of those questions arise for the existing kinds.
      - It is a wire-format change to a format just normalized across 232 files. A kind added
        later is cheap to read but means another normalization pass to write consistently.

      **The alternative worth pricing first:** do not add the kind, and let the ledger be the
      single source of truth. `merge-index` already reads the ledgers (for the V1–V4 column),
      so it can build the supersession table and the cross-book graph from `into:` without
      any new edge kind — and without the cross-tree dangling problem, because the ledger
      names a tree-qualified run (`run:` matches `books/merged/<slug>/`). That trades the
      self-describing `SKILL.md` for a gate that stays honest. Decide which of those two
      properties matters more before writing code.

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
- [x] **Usage output was printed for every runtime error.** DONE (2026-08-06): the dispatcher
      printed the selected command's whole help block for any error from an `exec` except
      `ff.ErrNoExec` and `root.ExitError`, so a failed read, a bad JSON payload, a failed
      write, or `relate`'s "no such skill" each buried the one useful line under a screen of
      flags. New `root.UsageError` + `root.Usagef` mark the 16 genuine misuse-of-the-CLI
      sites (wrong positional count, missing required flag, invalid flag value) across all 10
      command packages; `cmd.Run` now prints usage for those and only those. The negative
      condition ("not ErrNoExec and not ExitError") became a positive one, so it no longer
      grows a clause per error kind. Marks go on the *producer* — `parseGates` marks its own
      "unknown gate" and the existing `fmt.Errorf("verify: %w", err)` above it is untouched,
      since `errors.As` sees through the wrap; that also keeps the two identically-wrapped
      errors in `verify.exec` (usage vs runtime) distinguishable. `internal/lint.ParseCheck`
      stays a plain error and is classified at the two command boundaries that call it,
      because a subpackage must not import the command layer. `Usagef` returns the concrete
      `UsageError` rather than `error` so `wrapcheck` does not demand that the constructor of
      an error wrap something.
- [x] **Skip the name/folder check when the frontmatter did not parse.** DONE (2026-08-06).
      skillet v0.9.0 records `Skill.FrontmatterErr`, so `internal/lint.Check` now compares the
      name only when there was a name to read. On the book skill with the malformed
      `source_book:` line, `lint` went from four diagnostics — two of them consequences of the
      one YAML error — to **one**, naming `[10:45]`.
      The guard is deliberately narrow: only checks that read a field *out of the parsed
      block* are suppressed. The body checks still run, because `splitFrontmatter` separates
      the block before the parse is attempted, so `s.Body` is intact and its defects are real.
      Suppressing those too would make an author fix the YAML and lint again just to be told
      what could have been said the first time.
- [x] **Take the `redlines` trigger guard.** DONE (2026-08-06) by bumping to skillet
      v0.10.0; no code change here. `redlines.Check` no longer demands a trigger condition of
      a description it could not read. Only that check is guarded upstream — `checkSegments`
      and `checkQuotes` read the body, which `splitFrontmatter` produces before the parse is
      attempted, so a blanket suppression would have hidden the genuine 219-word quotation on
      this very skill.
      Verified on `books/site-reliability-engineering/blameless-postmortem-process`:
      `lint --check redlines` reports **2** diagnostics and **0** trigger complaints. That
      closes a chain spanning four packages — `skill` records the parse error, `speclint`
      reports it as itself, `internal/lint` stops comparing a name it could not read, and
      `redlines` stops asking for a trigger. The skill began the chain at four diagnostics,
      two of which were consequences of one YAML syntax error.
      (This entry was left open by a merge: PR #15 closed it, but was branched before #14
      landed and the overlapping edits resolved in #14's favour, discarding the closure.)
- [x] Drop the stale `pgx/v5 + sqlc` note in `RULES.md`. DONE (2026-08-07): removed the
      seven-line blockquote under §8 that claimed "this repo uses `pgx/v5` + `sqlc`" —
      exegesis has no database code at all. The remaining `pgx`/`sqlc` mentions are the
      generic Go guidance inherited from the shared rules ("for PostgreSQL, prefer pgx/sqlc")
      and are correct as advice.
      Left alone, but noted: the SQL checklist near line 2057 still names `districtsql.DBTX`,
      a type from the districts codebase, which is the same class of inherited boilerplate.

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

- [x] Bump skillet **v0.9.0 → v0.10.0.** DONE (2026-08-06), go.mod/go.sum only. v0.10.0
      carries the `redlines` trigger guard; taking it closed the last false diagnostic this
      repo produced on a skill whose frontmatter does not parse.
- [x] Bump skillet **v0.7.0 → v0.9.0.** DONE (2026-08-06), go.mod/go.sum only, twice.
      **v0.8.0** is the release carrying `skillet/redlines`, promoted *out of this repo* once
      skillsaw became the second consumer that justified it; `internal/lint.checkRedlines`
      and its five helpers were deleted in favour of it. **v0.9.0** adds
      `Skill.FrontmatterErr`, which is what let `lint` stop reporting a name/folder mismatch
      on a skill whose name could not be read.

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
- [x] **Centralized / bulk link mapper — `exegesis relate --edges edges.json TREE`.**
      `link` appends one edge at a time and `index` requires every `SKILL.md` to already
      carry a `## Related skills` section, so cold-starting a book of 20+ new skills meant
      hand-editing 20 files. DONE (2026-08-05): new `relate` command over a pure
      `internal/relate.Parse` (JSON `{"edges":[{from,kind,to,rationale}]}` → validated,
      slug-normalized, sorted `[]Group`) + `internal/related.UpsertAll` (the batched,
      idempotent `link` write path). It writes each source skill's `## Related skills`
      section, then regenerates `INDEX.md` via the shared `indexgen` (no `index` overload).
      Re-running the same table is a no-op; a missing source skill errors. Chose a
      deterministic JSON edge table over `--interactive` (scriptable, testable). Replaces
      `update_index_structure.py`.
- [x] **Promote `internal/lint`'s redline/RIA checks to skillet.** DONE (2026-08-06), code
      complete but **not yet publishable — see the blocker below**. New `skillet/redlines`
      with `Check(s *skill.Skill) []finding.Diagnostic` and an exported `MaxQuoteWords`,
      mirroring `speclint.Frontmatter`; `internal/lint.checkRedlines` and its five helpers are
      deleted and `Check` now calls `redlines.Check(s)`.
      Chose a **new package over a `speclint` extension**: speclint encodes the agentskills.io
      *spec* and changes when agentskills.io does; the red lines encode book2skill's *house
      quality rules* and change when the methodology does. Different authorities, so merging
      them would have coupled two independent change cadences.
      Behaviour is byte-identical by construction — the diagnostic message strings were moved
      verbatim, and `cmd/redlines_test.go` (which asserts on them through the CLI) passes
      unchanged, which is the proof the move preserved behaviour.
      **Blocker: exegesis requires `skillet v0.7.0` from the proxy, which has no `redlines`
      package**, so `GOWORK=off go build ./...` fails. Local development is bridged by a
      `go.work` (already gitignored, so nothing is committed) rather than a `replace` line that
      would break every other consumer. To finish: commit and push skillet, tag `v0.8.0`, then
      in exegesis `go get github.com/StevenACoffman/skillet@v0.8.0` and delete `go.work`.
      Publishing is deliberately left undone — it is an outward-facing release, not a code change.
      Follow-on: skillsaw is the second consumer that justified this (see `../skillsaw/TODO.md`);
      wiring it up is its own item and is still open.

## Gap-analysis re-examination (2026-08-06)

Source: the revised `~/Documents/agent-orange/gemini_skills/processing/gap_analysis.md`,
re-examining `relate`. One genuine gap; its stated mechanism was wrong, and the fix it
proposes is in the wrong place.

- [x] **Dangling edge targets are never caught — not by `relate`, `link`, `index`,
      `lint`, or `verify`.** DONE (2026-08-06). Reproduced (2026-08-06): a table with
      `{"from":"other-skill","kind":"depends-on","to":"user-lifecycles"}` against a tree
      whose real slug is `user-lifecycle` → `relate` exits **0** and writes the bullet
      into `other-skill/SKILL.md`; `verify` exits 1 only on unrelated test-prompts gates
      and says nothing about the edge; `index` exits 0; `lint` says `ok`. The INDEX.md
      Mermaid block renders both nodes and **no edge at all**, and the learning path is
      unaffected — the edge is silently dropped by `related.prereqs`/`edgeLines`, which
      both filter on `known[e.Target]` (`internal/related/graph.go:80,103`).
      **The analysis claimed a later `verify`/`index` crash; there is no crash.** The
      real failure is worse: the corruption is permanent and invisible, so a book's graph
      quietly loses edges nobody knows are missing.
      **Do NOT take the proposed fix** (`relatelib.Parse` accepting `tree` and stat-ing
      both endpoints): it breaks Parse's documented purity ("bytes in, values out", so the
      command owns I/O — `internal/relate/relate.go:1-4`), and it misses the same hole in
      `link` (which never checks `--to` either, `cmd/link/link.go:74-90`) and every
      hand-edited `## Related skills` section. Two-part fix instead:
      1. **A graph-integrity check in `verify` (and a warning from `index`)** that reports
         every edge whose target is not a discovered slug. This is the load-bearing half —
         it catches edges already on disk regardless of who wrote them, and `indexgen`
         already has the full slug set in hand.
      2. **A pre-write target-existence check in the `relate` and `link` execs** (not in
         `Parse`), so a typo fails fast at the point of authorship instead of at the next
         verify. Keep the check in the command layer, where the tree path already lives.
      Ordering note: (1) alone closes the exposure; (2) is the ergonomic add-on. If
      `internal/related` is ever promoted to `skillet/related` (see the offload candidates
      above), the integrity check should travel with it.
      **As built:** a pure core in `internal/related` — `DanglingEdges(nodes)` (the exact
      complement of the `known[e.Target]` filter it sits beside, so the two cannot drift)
      and `UnknownSlugs(nodes, want)` for the pre-write case. `collectNodes` was exported
      as `indexgen.CollectNodes` and is now the single tree walk behind `index`, `verify`,
      `relate`, and `link`, so a graph report names exactly what INDEX.md would drop.
      `verify` gained a tree-scope `checkGraph` mirroring `checkCatalog` (blocking, folded
      into `structure_verified`, `graph: ` prefix) as part of the existing skills gate — no
      new `--gates` name, since a dangling edge is a property of the relationships among
      skills and callers cannot value that knob better than we can. `relate` validates both
      endpoints of every edge **before the first write**, which also makes the batch atomic
      (it used to commit group-by-group, so a bad target in the last group left the earlier
      ones written). `link` warns instead of failing, because it is given only a skill
      directory and must infer the tree as its parent; the warning names the tree it
      checked, and `relate` — which is handed the tree explicitly — errors on the same
      condition.
      **Deviation from the plan above: no `index` warning.** It needed either a second tree
      walk or an exported two-line `Render` wrapper for one caller, and it puts diagnostics
      inside a renderer. Authorship-time (`relate`/`link`) plus gate-time (`verify`) already
      cover the lifecycle.
- [x] **Real trees' `## Related skills` sections do not parse — `index` renders an empty
      graph for every book.** DONE (2026-08-06). Found while validating the check above, and a
      bigger silent-drop than the dangling-target one. `ParseSection` requires the exact
      heading `## Related skills` and the canonical bullet
      `` - <kind>: `<target>` — <rationale> ``. Across `~/Documents/agent-orange/books/`:
      36 files use the exact heading but write bullets as
      `` - **composes-with** [`slug`](../slug/SKILL.md): rationale `` (bolded kind, markdown
      link), and 28 more use the heading `## Related skills (Stage 3 Filling)`. Net result:
      **zero edges in any real tree are visible to exegesis**, so every `INDEX.md` graph and
      learning path is empty, and the new graph gate is trivially clean there. (The
      `districts-ff/.claude/skills` deployment tree is worse still — mdformat rewrote its
      `---` frontmatter delimiters to `______`, so `skill.Load` parses no frontmatter at
      all.) Decide one of: (a) widen `ParseSection` to accept the bolded-kind/linked-target
      form and a heading *prefix* match, or (b) treat the canonical form as the only
      contract and normalize the books through `relate` once. (a) is more forgiving but
      makes the wire format ambiguous for round-tripping; (b) keeps one format but needs a
      migration pass. Until then the graph gate can only catch edges authored through
      `relate`/`link`.
      **Chose (a), narrowly: a tolerant reader, an unchanged canonical writer.** A full
      survey found **five** bullet families, not two — bold-kind + linked backticked target
      (120), a **reversed** `**slug** (kind):` form (59), bare-token-then-prose (36),
      plain-kind + linked bare target (30), and plain-kind backticked incl. multi-target (20)
      — plus the two headings. New `internal/related/dialects.go` reads all of them;
      `findSection` matches the heading by prefix; `ParseSection` expands a multi-target
      bullet to one edge per target and dedupes by (kind, target).
      **Result: 223 Mermaid edges now render across the 32 book trees (was 0), 32/32 trees
      have a non-empty graph, and 0 dangling edges are reported** — so the graph gate added
      above stays clean and no book newly fails. The learning path also surfaced a real
      `depends-on` cycle in site-reliability-engineering
      (`fifty-percent-engineering-time-cap` ↔ `on-call-sustainability-model`) that was
      invisible while the edges were unparsed.
      Design notes worth keeping: (1) **`parseBullet` was deliberately NOT widened** — it is
      the *writer's* matcher, and making it multi-target-aware would let an upsert of
      ``(composes-with, a)`` onto `` - composes-with: `a`, `b` `` rewrite the line and
      silently drop `b`. Tolerance lives only on the read path; verified that `relate` over a
      legacy tree is idempotent, writes into the suffixed section rather than appending a
      second one, and leaves legacy bullets intact. (2) **Targets are extracted anchored to
      the head of the bullet, never by scanning the line** — a first draft manufactured edges
      to `--force` and `--yes` out of backticked flags in a *rationale*, which the graph gate
      then reported as dangling; that is now a regression test. (3) Tolerating a target
      written as `[`slug`](../slug/SKILL.md)` does **not** contradict `lint` still flagging
      the parent-escaping link: the reader takes the slug and ignores the path, and the two
      answer different questions.
      **Known limit (deliberate, evidence-based):** a bullet that names no resolvable skill
      yields no edge and is not reported. All 5 such bullets in the corpus are intentional
      prose about non-skill concepts (`- contrasts-with: (traditional ops team
      headcount-scaling model)`), so a diagnostic would have been a 5-of-5 false positive.
      The residual risk is that a genuinely typo'd bullet (`- depends-on: Four Golden
      Signals`) stays invisible. Revisit only with a case that is not prose.
- [x] **Normalize the books to the canonical bullet format.** DONE (2026-08-06) for every
      dialect exegesis can map; see the residual below. New `related.Normalize` +
      `exegesis normalize [--check] TREE`. The rewrite **substitutes only the lines it
      understands** — a bullet whose target is prose, an intro sentence, fenced code, and
      everything outside the section are copied through byte-identical — so it cannot discard
      content it did not parse. `relate --edges` could not have done this: it appends canonical
      bullets and leaves the legacy ones.
      Two data-loss risks were found by measuring first and are covered by named regression
      tests: **9 wrapped rationales** (continuation lines are now folded into the bullet, which
      also fixed truncated rationales in the reader) and **5 prose bullets** that name no skill
      (preserved verbatim; regenerating the section from parsed edges would have deleted them).
      **Mid-implementation discovery: `## Related Skills` (capital S) appears in 189 files** —
      my original survey grepped case-sensitively and missed it, so those sections were still
      invisible. `isSectionHeading` is now case-insensitive, which took the visible graph from
      **223 to 270 edges**.
      Applied to `~/Documents/agent-orange/books`: **232 files rewritten**, all 32 trees pass
      `normalize --check`, graph edges 270 before == 270 after, 0 dangling. Verified
      information-preserving by a word-multiset comparison over all 242 skills, discounting the
      heading suffix and the removed link paths: **0 words lost**.
- [ ] **Decide the kind vocabulary for the sixth bullet dialect (250 bullets).** Found while
      normalizing. A sixth dialect exists that the reader does not map:
      `` - **[slug](../slug/SKILL.md)** — informs: why ``. Its kinds are a *different taxonomy*
      from the three canonical ones — `informs` (93), `prerequisite for` (44), `combines` (40),
      `depends on` (39), `relates` (22), `compares` (12). Only `depends on` is an unambiguous
      spelling of `depends-on`. The rest need a semantic decision that is yours, not the
      parser's: `combines`→`composes-with` and `compares`→`contrasts-with` are plausible, but
      **`prerequisite for` looks like the inverse of `depends-on`** (A is prerequisite for B
      means B depends-on A), so mapping it without flipping the direction would silently
      reverse 44 edges, and `informs`/`relates` have no canonical equivalent at all. Decide the
      mapping (including whether to add kinds) and the reader can absorb the dialect in an
      afternoon; until then these 250 bullets stay invisible to `index` and untouched by
      `normalize`.
- [ ] **13 skills have two `## Related skills` sections.** Pre-existing in `HEAD`, not caused by
      the normalization — typically a suffixed lowercase heading followed by a capital-S one.
      `findSection` takes the first and stops at the next heading, so the second section's
      bullets are invisible. Merge them in the content, or teach the reader to concatenate
      every related-skills section in a file. Merging is a content edit, so it is left to you.
- Cross-repo decision (no action yet): the same analysis proposes writing
      `testprompts.DeriveChecks` output back into `test-prompts.json` on the skillsaw side.
      exegesis is already half of that — `scaffold`'s `BuildTests` persists derived checks —
      so if the write-back lands as a producer feature it belongs on `exegesis tests`
      (`--derive-checks`, alongside `--scaffold`), filling checks for cases that have none.
      Pick one home; see `../skillsaw/TODO.md` for the consumer-side framing and the
      legacy-shape normalization caveat.

## Reasoning-toolkit survey (unified-thinking, 2026-08-05)

Source: a survey of `~/Documents/git/unified-thinking` (a deterministic Go reasoning
toolkit). **Lowest relevance of the family** — exegesis is structural gating
(lint/verify/redlines), off-axis from the statistical judgment where that toolkit's
deterministic rigor lives. No meaningful code to lift.

- The one plausible touch: a timeseries **regression gate** to track distill/gate quality
  over time (fail on a drop vs. a rolling baseline). **The shared half now exists** —
  wanting it here *and* in skillsaw was the 2nd consumer that promoted it, so it landed as
  `skillet/timeseries.Detect(history, current, Config) Verdict` (2026-08-07, needs the
  bump). It is not a copy of unified-thinking's `DetectRegression`: that one is not a
  rolling window at all (it takes the single most recent run as the baseline), reads a zero
  baseline as an absent one, and divides by the baseline. Consuming it here is still
  deferred / low priority — exegesis is structural gating, off-axis from statistical
  judgment. When it is picked up: `Tolerance` is absolute, in the metric's own units, and
  must be set deliberately; a zero tolerance calls any drop a regression. Its reasoning
  algorithms (Bayesian/causal/fallacy/MCDA) and keyword detectors still do not fit
  exegesis's structure tier.
