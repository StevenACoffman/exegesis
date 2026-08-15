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
to run four exegesis subcommands, three flags and one edge kind. As of 2026-08-08
`quotecheck`, `merge-status` and `tests --merge`/`--migrate` exist; `merge-index`,
`a2check`, `verify --merge`, `merge-status --link` and the `superseded-by` edge kind still
do not, so an agent following that skill still fails at those steps. Both open *decisions*
behind them are now settled (the provenance model, and adding the edge kind) — what is left
is construction. The skill is otherwise left as-is; it reads as the spec for this work,
with its frontmatter-key claim corrected 2026-08-08.

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
      **`--link` is NOT built**: it writes a `superseded-by` bullet. The flag is accepted and
      returns a usage error saying so, rather than failing as an unknown flag.
      **Unblocked 2026-08-08** — the edge kind is decided (add it; see below). Building
      `--link` now waits only on the kind itself, plus one sub-question it owns: `--into` is
      documented as a bare merged-skill slug while the bullet needs a tree-qualified target,
      so either `--link` composes `merged/<run>/<into>`, or `--into` starts carrying the
      qualified form and the ledger's `into:` becomes qualified with it.
- [x] **Which provenance model is authoritative — DECIDED 2026-08-08.** The follow-on work
      is `merge-migrate` and `merge-index` below; this item is the record of the decision and
      the evidence behind it. It was raised because the spec and the only real merged tree
      appeared to describe two different data models, the spec's with zero instances on disk
      (surveyed 2026-08-07 against `~/Documents/agent-orange/books/merged/all-books-v1`).

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

      **DECIDED 2026-08-08.** The framing above was wrong on one point, and the correction
      settles most of the rest: the spec is *not* ambiguous. `merge-skills/methodology/
      05-phase2` and `templates/MERGED_SKILL.md.template` both say, in as many words, do not
      emit `id`/`title`/`type`/`source_skills`/`related_skills` as frontmatter — capture the
      sources in a body `## Provenance` section. The single line reading otherwise
      (`SKILL.md:424`) names the *concept* `related_skills`, not the frontmatter key. So
      `all-books-v1` is not a rival model; it violates the spec of the skill that produced it.

      1. **Governed — migrate, staged.** Composition moves into the body now; the source-side
         supersession links wait on the `superseded-by` decision below, so this does not
         depend on it.
      2. **Body, as a fenced `yaml` block inside `## Provenance`** — the shape `## Merge
         Status` already uses, for the same reason: machine-readable without putting a
         non-spec key in frontmatter. No `speclint` exemption. (`metadata` was considered and
         rejected: the spec defines it as a map of string keys to *string* values, and a
         composition is a list of records.)
      3. **Cluster kept, sourced from an optional `clusters.yaml` at the merge root**, with a
         sensible default when absent (one cluster per skill, or no Cluster column at all —
         whichever reads better in the generated table). Editorial judgment stays in a
         human-owned file instead of becoming a per-skill key.
      4. **Both, with distinct jobs.** The ledger is authoritative for a source skill's
         *fate* — including `rejected` and `no-candidate`, which no merged skill exists to
         record. The body Provenance block is authoritative for a merged skill's
         *composition*. `merge-index` joins them on pair-id.

      **Measured, and it makes the migration cheap: the `supersedes` edges carry no
      information.** For all 27 merged skills the set of
      `related_skills[relation=supersedes].slug` **equals** the set of `source_skills.slug`.
      54 edges, zero bits beyond `source_skills`; the only unique content is 55 `note:`
      strings, which the Provenance block carries as `note:` per source. So the migration is
      lossless without inventing an edge kind, and forward supersession is derived rather
      than restated.

      **The frontmatter allowlist was itself wrong, and is now fixed.** Checked against
      <https://agentskills.io/specification> (2026-08-08): the defined keys are `name`,
      `description`, `license`, `compatibility`, `metadata`, `allowed-tools`.
      `skillet/speclint` was rejecting three of them and the merge-skills doc claimed
      `author` and `version` were allowed — neither is a top-level key at all; the spec's own
      example carries both inside `metadata`. `speclint.AllowedFrontmatterKey` now matches the
      spec, keeping `tags` as the one documented deviation (163 installed skills and every
      book2skill output carry it, so rejecting it would report a defect on nearly every skill
      in existence). The merge-skills doc and template are corrected. **Not released:**
      skillet needs a tag and an exegesis bump before the fix reaches this repo's lint.

      **Still to do, in order** (each is small and now unblocked):
      - `exegesis merge-migrate MERGED_TREE` (or a one-off): frontmatter → body
        `## Provenance` + fenced yaml, `name:` set to the folder, `title:`/`id:` dropped
        (both derivable), notes carried. 27 files. Verify with `lint` + a word-multiset check.
      - Then `merge-index` (below), reading Provenance blocks, ledgers, and `clusters.yaml`.
      - `verify --merge` is still needed for the *other* failure the tree has: the default
        test-prompt validator rejects `prefer_merged_over_source`, which `tests --merge`
        accepts. Three cases also have an empty `expected`, which is a content fix.
- [x] **`exegesis merge-migrate MERGED_TREE [--check]`** — DONE (2026-08-08). Stage one of the decision above:
      move a merged skill's provenance out of frontmatter and into the body. Per skill:
      `name:` ← folder (the tree has none, which is half its lint failures); `description`
      and `tags` kept; `id`/`title`/`type`/`source_skills`/`related_skills` removed; a
      `## Provenance` section written with the prose bullets plus the fenced `yaml` block;
      the 55 `note:` strings carried onto their source entries; the single
      `relation: composes-with` written as a body `## Related Skills` bullet, which is a
      canonical kind and needs no decision.
      **The 54 `supersedes` relations are dropped, not migrated** — measured identical to
      `source_skills` in all 27 skills, so writing them again would be one fact in two places.
      Source-side `superseded-by` bullets are a separate stage, waiting on the edge-kind
      decision; nothing here depends on it.
      **Measured on a copy of the real tree: 162 lint errors → 0, all 27 migrated,
      `--check` idempotent afterwards.** That number is the whole point of the exercise.
      A word-multiset comparison found the one real defect this could have shipped with:
      the 55 `note:` strings live on `related_skills`, not on `source_skills`, so dropping
      the redundant `supersedes` relations was taking somebody's prose with them. `withNotes`
      joins them onto their source by slug before the relation is dropped. After that fix the
      only words that disappear are structural keys (`id:`, `title:`, `relation:`,
      `supersedes`) and the frontmatter title itself — which is an exact duplicate of the
      body `# ` heading in the 17 skills that have one, and becomes that heading in the 10
      that do not.
      Built as a command rather than a one-off because `--check` gates a tree in CI, which a
      script cannot; the pure core is `internal/mergemigrate`, the shell mirrors `normalize`'s.
      **The real tree has not been migrated** — every measurement above was taken on a copy.
      Running `exegesis merge-migrate books/merged/all-books-v1` rewrites 27 files that are
      already carrying uncommitted changes, so it is left as your call, and `merge-index` is
      what needs it done.
- [x] **`exegesis merge-index MERGED_TREE`** — DONE. First the blocker: the migration is no
      longer pending — `exegesis merge-migrate` was run over `books/merged/all-books-v1`
      (27/27 skills), so every merged skill now carries the `## Provenance` block this reads.
      `internal/mergeindexgen.Generate(tree)` + `cmd/merge-index` (mirroring `indexgen`/`index`,
      `--check` staleness, `atomicfile`) regenerate `MERGED_TREE/INDEX.md`, replacing the
      hand-maintained cross-book provenance index. Deterministic (no date stamp) so `--check`
      works. Sections, one source of truth each:
      - **Cross-Book Provenance Table** ← each skill's `## Provenance` fenced `yaml`
      (`source_skills`), parsed with `mergemigrate.Provenance` (the exact shape
      `mergemigrate.Render` writes — no second parser). A source feeding ≥ 2 merged skills is
      marked `★`; the count is in the header. Verified against the hand-made INDEX: same rows,
      same fan-in marks (4).
      - **Source Verification Summary** ← `source-verification/*.md` headers via
      `frontmatter.Split`. Zero on disk → renders "No source-verification records on disk
      yet" (honest, as predicted). The **V1–V4-from-ledgers** column (`internal/mergestatus`)
      is deferred until those files exist — nothing to join today.
      - **Rejected Pairs** ← `rejected/pair-*.md`, rendered as links with each file's first
      `#` heading as the label (5 present).
      - **Cluster Summary / column** ← follow-up: `clusters.yaml` does not exist, so the column
      is omitted rather than guessed (per the original decision). The hand-made INDEX's
      hand-assigned clusters are therefore not in the generated file; add a `clusters.yaml`
      and extend `mergeindexgen` if the cluster column is wanted back.
- [x] **`exegesis a2check --source-skill A,B MERGED_SKILL_DIR`** — DONE (2026-08-08). The
      A2-sharpness gate: the merged `A2` must carry ≥2 language signals neither source has.
      Advisory by default, `--strict` to fail. Counting is structural; whether the signals are
      semantically distinct stays the agent's.
      **A signal is a double-quoted phrase in A2, not a "### Language Signals" subsection.**
      Measured: only 10 of the 27 merged skills have that heading or its bold-label variant,
      so a reader that required it would score two thirds of the tree as having no signals.
      **The threshold discriminates — measured across the 24 merged skills whose sources are
      on disk:** 15 clear the bar, 3 add exactly one, 1 adds none of the one it states, and
      **5 state no signal at all**. That last group is why the report distinguishes "states no
      A2 language signals" from "0 of N are new": they are different defects, in different
      places, fixed by different edits, and one message for both would send a reader looking
      in the wrong segment.
      `textnorm` was promoted out of `quotecheck` on this second consumer — two guards that
      folded text differently would disagree about whether the same words match, and no
      output could explain why. `quotecheck` now calls it directly rather than through a
      pass-through wrapper.
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
- [x] **`exegesis verify --merge`** — DONE (the measured blocker). `--merge` gates each
      skill's test-prompts under the merged composition and requires `MERGE_OVERVIEW.md`.
      The composition profile lives in `internal/testcomp.For(merged bool)`, which both
      `tests` and `verify` now call, so the two cannot disagree about a merged tree (the
      drift that let `verify` reject `prefer_merged_over_source` while `tests --merge`
      accepted it). Verified end-to-end on `all-books-v1`: plain verify raised 72
      unknown-type complaints, `--merge` drops them to 0 and reports `MERGE_OVERVIEW.md: ok`,
      then surfaces the genuine empty-`expected` content defects (tree fixes, as predicted).
      **Deferred (not the measured blocker):** `--source-book A,B` (the `a2check` attribution
      advisory across merged skills) and `--strict`. Both need each merged skill's source
      provenance wired in; filed as follow-up rather than gold-plated onto this pass.
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
- [x] **A `superseded-by` edge kind — DECIDED and BUILT 2026-08-08.**
      merge-skills links a source skill to the merged skill that replaced it
      (`link --kind superseded-by --to <merged-slug> books/<slug-a>/<source-skill>/`), but
      `related.Kind` admits only `depends-on`, `contrasts-with` and `composes-with`, so that
      call is rejected today. The pros and cons that informed the decision are kept below;
      what follows first is the design the corpus settled and the work it implies.

      **The corpus already writes these edges — 26 of them (measured 2026-08-08).** Every
      one is a body bullet in the bold-kind dialect with a *tree-qualified* target:
      `- **superseded-by**: merged/all-books-v1/<merged-slug> — why`. Five further
      `superseded-by` bullets name prose rather than a skill and must keep yielding no edge.
      All 31 are invisible today, because `Kind.Valid()` rejects the kind *and* `isSlug`
      rejects a target containing a slash.

      **The cross-tree blocker resolves into the wire format, and not as a special case.**
      The same qualified form already appears on the three existing kinds — 9 targets like
      `depends-on: site-reliability-engineering/<skill>`, all in the merged tree, which is
      exactly what merge-skills `methodology/06-phase3` prescribes. So a target is
      `[<tree-path>/]<slug>` for **every** kind, each segment strictly slug-shaped so prose
      still yields nothing; qualified means "another tree, resolved against the parent of
      this one". That is the third of the three options priced below, and it turns out not to
      change the wire format at all — it describes what the corpus is already doing.

      **Which makes the gate honest rather than weakened:**
      - `DanglingEdges` (pure, tree-scoped) **skips a qualified target**, because it cannot
        see the other tree. Reporting them would be a false positive on all 35 real edges.
      - The existence check moves to authorship time, where the filesystem is: `relate` errors
        and `link` warns — the split those two already have — resolving
        `<tree>/../<qualified-target>/SKILL.md`. `internal/related` stays pure; the resolution
        belongs in `indexgen`, which is the package that already walks trees on disk.
      - `Mermaid` will not render a qualified edge, since `edgeLines` filters on in-tree
        slugs. That is correct here and worth stating plainly: the cross-book graph is
        `merge-index`'s job, not a book's INDEX.

      **The terminality question, answered:** a superseded skill still appears in the skill
      list and the learning path. merge-skills is explicit that source skills are retained as
      the audit trail of what was merged, and `prereqs` filters on `DependsOn`, so
      supersession cannot reorder the path.

      **As built.** `related` gained the constant, `Kinds()` (the vocabulary as one value, so
      `Valid` and every usage message read from one definition — the shape `mergestatus.States`
      already uses), `Qualified(target)`, and a qualified-target grammar in `dialects.go` where
      `isTarget` accepts a path of strict slugs. `DanglingEdges` and `UnknownSlugs` skip
      qualified targets; `indexgen.MissingQualified` resolves them against the tree's parent,
      so `relate` errors and `link` warns — the split those two already had.
      **A pre-existing reader gap had to be closed first, and a probe found it rather than a
      reading of the code:** `- **kind**: target` parsed as *no edge at all*, for every kind,
      because only "→" was stripped after a bold kind and never ":". That is the exact shape
      all 25 real `superseded-by` bullets use, so the kind alone would have changed nothing.
      `afterBoldKind` now takes either separator.
      **`link` was silently destroying qualified targets:** it ran `skill.Slug` over the whole
      `--to`, which folds "/" to "-", so `merged/all-books-v1/x` would have been written as one
      slug naming a skill that cannot exist. Segments are slugged individually now.
      **Measured on a copy of the real books, old binary against new:** 21 files change, all of
      them a bold-kind `superseded-by` bullet becoming canonical; **0 words lost**; in-tree
      Mermaid edges 270 before == 270 after; **0 dangling reports**, which is the check that
      the gate did not start crying wolf about cross-tree targets. 25 raw bullets → 24 rewritten
      plus 1 in `effective-go-recipes/skills/`, whose skills sit two levels below the tree root
      and which converts when handed that path — a discovery-depth quirk, not a reader gap.
      **Still open, and it belongs to `--link` rather than here:** `--into` is documented as a
      bare merged-skill slug while the bullet needs a tree-qualified target, so either `--link`
      composes `merged/<run>/<into>` from the ledger fields, or `--into` starts carrying the
      qualified form and the ledger's `into:` becomes qualified with it.

      Noted while measuring, not part of this: **25 source skills carry
      `relation: superseded-by` in *frontmatter*** — the same lint-failing shape the merged
      tree has, on the source side. The migration item above only covers `books/merged/`;
      these are a second, larger population that needs the same treatment.

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

      **The alternative that was priced and not taken:** do not add the kind, and let the
      ledger be the single source of truth. `merge-index` already reads the ledgers (for the
      V1–V4 column), so it could build the supersession table and the cross-book graph from
      `into:` without any new edge kind — and without the cross-tree dangling problem,
      because the ledger names a tree-qualified run (`run:` matches `books/merged/<slug>/`).
      Rejected because the trade it was priced against turned out not to exist: qualified
      targets keep the gate honest *and* keep the `SKILL.md` self-describing. What remains of
      the objection is the duplication — the ledger's `into:` and the edge record the same
      fact — so the rule is that `--link` writes both in one call and neither is hand-edited.
      The ledger stays the more expressive record (`state`, `reason`, `excluded`); the edge is
      the navigation pointer. That is the same division the merge-skills spec draws between
      `merge_status` and a related-skill link, so it is not a new concept to hold.

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

- [x] **Release the `speclint` allowlist correction and bump to it.** DONE (2026-08-08):
      skillet **v0.12.0** tagged (PR #13), exegesis and skillsaw both bumped, both suites green. Checked against <https://agentskills.io/specification>: the defined
      frontmatter keys are `name`, `description`, `license`, `compatibility`, `metadata`,
      `allowed-tools`. `AllowedFrontmatterKey` permitted only four of them and rejected
      `license`, `compatibility` and `metadata` — a skill declaring its own license was told
      the key does not exist. `tags` stays permitted as the one documented deviation: the spec
      does not define it, but 163 installed skills and every book2skill output carry it, so
      rejecting it would report a defect on nearly every skill rather than describe one.
      `author` and `version` are **not** top-level keys at any level of the spec — its own
      example puts both inside `metadata` — and the merge-skills doc that claimed otherwise is
      corrected, along with its merged-skill template.
      Verified rather than asserted: a skill carrying `license:` + `metadata:` went from two
      `disallowed key` errors to `ok`. **0 skills in `books/` use the three keys and 15 of the
      installed skills do**, so the trees this repo gates were unaffected while the corpus an
      agent actually loads was the one being misreported.
      **Watch for the follow-on:** widening the allowlist can only *reduce* findings, so no
      tree newly fails — but any skill that was passing lint *because* it had no `metadata`
      block may now start using one, and `metadata` values are string-only. That constraint is
      not checked by anything today.
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
- [x] **`quotecheck --min-support N` — the mechanical half of book2skill's V1.** DONE
      (2026-08-08). Survey of book2skill (2026-08-08) for deterministic work still left to the
      agent found only this one; the rest of that skill's remainder is genuine judgment (V2
      predictive power, V3 uniqueness, RIA++ construction, test-prompt design), and its
      deterministic half was already offloaded — `verify` covers red lines #2-#5 and the graph,
      and `quotecheck` now covers quotation containment.
      V1 asks whether the book contains "at least two independent paragraphs providing
      supporting evidence". Whether a passage *supports* a unit is judgment; whether the
      claimed passages *exist in the source* is exactly what `quotecheck` already answers.
      `--min-support N` fails a skill with fewer than N located passages, gating the countable
      half and leaving the semantic half where it belongs.
      Partial by design — do not let it read as "V1 is now automated". The help text says so
      in the same words: it cannot tell whether a passage supports the unit, nor whether two
      located passages are independent rather than two sentences of one paragraph.
      **Support is counted in passages, not quotations,** which is forced by the same
      measurement that shaped `quotecheck` itself: 95% of the corpus writes the whole R
      segment as one blockquote, so a quotation count is 1 for nearly every skill and
      `--min-support 2` could never be met by a faithful one.
      **The counting lives in `quotecheck.Support`, not in the command**, so the number the
      gate compares and the number the "N/M passages located" line prints are the same number;
      the shell now derives its miss count from it rather than tallying separately.
      **The case that decided the semantics: a skill that quotes nothing.** It used to print
      "no quotations …" and pass, which under a threshold is exactly backwards — zero located
      passages is zero support — so `Support(nil) == 0` and the gate reads the same for both.
      Without the flag the output is byte-identical to before.
      Verified on a real skill (`site-reliability-engineering/blameless-postmortem-process`)
      against its own R text as the source: 10/10 located, exit 0 at `--min-support 3`; against
      an unrelated source, 0/10 and `SUPPORT 0 located, --min-support 3`, exit 1.
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
- [x] **Skills with two `## Related Skills` sections — the reader now merges them and
      `normalize` folds them into one.** DONE (2026-08-08). Pre-existing in `HEAD`, not caused
      by the normalization — typically a suffixed heading followed by a plain one.
      `findSection` took the first and stopped at the next heading, so the second section's
      bullets were invisible to `index`, `verify`, `relate` and `link`.
      **Count corrected again, 19 → 13 (2026-08-08), measured with the real matcher.** All 13
      are in `books/site-reliability-engineering/`. The 2026-08-07 figure of 19 does not
      reproduce; the count that matters is the one the code reports.
      **As built, in three parts.** `findSections` returns every section rather than the first;
      `ParseSection` reads them all, dedupes by (Kind, Target) first-occurrence-wins as before,
      and now returns the edges **sorted by kind then target** rather than in file order — a
      relationship means the same thing whichever section states it, so the reader's answer
      should not record which one won. `Upsert` looks for an existing bullet in every section
      but writes a new one to the first, so the next `relate` over one of these files cannot
      manufacture a duplicate of an edge the second section already states.
      **`normalize` merges a later section only when every line of it is a bullet, a blank, or
      a thematic break** — measured: all 13 have exactly that shape (heading, blank, bullets,
      blank, break). Its bullets move into the first section, canonical where understood and
      verbatim where not, and the span is deleted whole. A later section holding prose is left
      where it is and normalized in place; deleting a paragraph to tidy a heading is not a
      trade worth making, and that guard is the documented limit.
      **The merge recovers 0 new edges, and that is the honest finding.** Those second sections
      write `**depends_on**` and `**composes_with**` with underscores, which are not valid
      kinds, so they yielded nothing before and yield nothing now. Their bullets carry the only
      rationale anyone wrote, which is why they move verbatim rather than being dropped with
      the heading. Mapping the underscore spellings belongs with the sixth-dialect decision
      above, not here.
      **Measured on the real corpus** (a copy; `~/Documents/agent-orange/books` was not
      touched), old binary against new: **exactly the 13 files differ and nothing else**; the
      only words removed from each are `## Related Skills` and one `---`; every one now holds a
      single section; `normalize --check` is clean afterwards; Mermaid edges 270 before == 270
      after, 0 dangling in both; and **every INDEX.md is byte-identical**, which is the evidence
      for the claim that sorting the reader's output is invisible downstream.
      Noted while measuring, not acted on: `books/` currently has 287 files modified but
      uncommitted, written 2026-08-06 by the pre-title-case binary — their headings are the
      lowercase `## Related skills`. That is why 242 skills read as non-canonical today, under
      `HEAD` and under this change alike. It is a corpus question (commit or discard that
      working tree), not a code one.
- Cross-repo decision (no action yet): the same analysis proposes writing
      `testprompts.DeriveChecks` output back into `test-prompts.json` on the skillsaw side.
      exegesis is already half of that — `scaffold`'s `BuildTests` persists derived checks —
      so if the write-back lands as a producer feature it belongs on `exegesis tests`
      (`--derive-checks`, alongside `--scaffold`), filling checks for cases that have none.
      Pick one home; see `../skillsaw/TODO.md` for the consumer-side framing and the
      legacy-shape normalization caveat.

## SkillLens quality dimensions — a third lint tier (2026-08-08)

Source: `~/Documents/agent-orange/skillopt_changes_findings.md`. skillsaw scores three
dimensions taken from `microsoft/SkillLens` (arXiv:2605.23899) — failure-mechanism
encoding, actionable specificity, and a high-risk action blacklist, each validated at
65–66% predictive accuracy against downstream skill utility. exegesis gates structure and
today measures none of them, so a tree can pass every gate here and still be the thing
SkillLens names as the dominant failure mode: polished, comprehensive-sounding guidance
carrying no concrete failure knowledge.

- [x] **Add `lint --check skilllens` over `skillet/skilllens`.** DONE. A third opt-in tier
      beside `speclint` and `redlines`, on its own schedule: `internal/lint/skilllens.go`
      parses the body once (`markdown.Parse`) and translates the three `skilllens` detectors
      to diagnostics. `ParseCheck` now returns a `Checks{Redlines, SkillLens}` struct;
      `--check skilllens` enables just this tier, `--check all` enables both. `verify`
      honors it too. Default off, like `--check redlines` (the same 13-of-20 hand-written-
      skill problem applies). The unblock note above is now stale: skillet v0.13.0 (pinned)
      ships `skilllens`, so nothing was stranded.
      **All three signals are warnings, not the proposed error** — settled by the
      measurement below, and because the coarse "has any boundary section" test is not the
      finer "the B segment does its job" structural check the error tier would need.
- [x] **Measure before choosing the error/warning split above.** DONE, and it changed the
      severity. Reused the shipped tier as its own harness (`lint --check skilllens --json`
      over the corpus, no throwaway program). Across **277 finalized skills**: 5% lack a
      failure-mechanism span, **0%** carry ≥3 softening phrases, 6% lack a boundary section
      — and all 18 boundary-misses coincided with an already-broken skill. Nothing fires on
      more than 6%, so no signal blocks book2skill output; all three ship as warnings rather
      than the proposed structural `error`. The stricter "B segment present but empty of a
      reasoned forbidden pattern" check is left to future work.

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
