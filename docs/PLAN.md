# Book2skill — Go Implementation Plan

Implementation plan for turning the existing `climax`/`ff/v4` scaffold in this
directory into the `book2skill` pipeline described in `SKILL.md`,
`methodology/`, `extractors/`, and `templates/`, and specified in
`../book2skill-spec.md`.

This plan was written against the pipeline spec and then improved against
`../go-advice/summary_rules.md`; see §11 for the improvement pass.

______________________________________________________________________

## 1. Goal and Starting Point

**Goal:** a `book2skill` CLI that distills a book into a set of atomic,
executable skills, following the RIA-TV++ pipeline (stages 0, 1, 1.5, 2, 3, 4).
Everything that can be ordinary code is ordinary code; a narrow, injected LLM
interface covers the steps that genuinely require a model.

**Already present (climax scaffold — do not fight it):**

- `main.go` — entry, signal handling, exit-code translation (CLI shape B).
- `cmd/cmd.go` — dispatcher; env prefix `BOOK2SKILL`.
- `cmd/root/root.go` — `Config` (Stdin/Stdout/Stderr/Flags/Command), `ExitError`.
- `cmd/version/version.go` — version subcommand.
- `go.mod` — module `github.com/StevenACoffman/book2skill`, `ff/v4`.

The scaffold is the **composition root + dispatch layer** (go-advice §7 shape B,
§1 rule "main wires dependencies; commands dispatch; neither owns business
logic"). All pipeline logic goes in packages the commands call.

______________________________________________________________________

## 2. Resolved Design Decisions

These close the inconsistencies surfaced during spec review and the darwin
compatibility question.

### D1 — Quote Length Cap (DETERMINISTIC)

Cap `source_quote` by **characters**, script-detected:

- **CJK-dominant** source text → **150 characters**.
- **Latin/other (unicode)** → **650 characters**.

Count by Unicode code points (`utf8.RuneCountInString`), not bytes. Script
detection: sample the book text; if the share of Han/Hiragana/Katakana/Hangul
runes exceeds a threshold (e.g. 20%), treat as CJK. One knob:
`QuoteMaxRunes` with a script-derived default, overridable by flag.

### D2 — Darwin `test-prompts.json` Schema (Verified Against the Local Darwin-Skill Repo)

darwin-skill (`SKILL.md` §Phase 0.5, lines 132–136) consumes a **bare JSON
array**:

```json
[
  {
    "id": 1,
    "prompt": "...",
    "expected": "..."
  },
  {
    "id": 2,
    "prompt": "...",
    "expected": "..."
  }
]
```

`id` is an **integer**; each object needs `prompt` and `expected` (a short
description of expected output). darwin ignores unknown keys.

**Decision:** emit exactly this top-level array. book2skill's own Phase-4
metadata rides along as **extra keys per object** that darwin ignores:

```json
[
  {
    "id": 1,
    "type": "should_trigger",
    "prompt": "...",
    "expected": "invokes inversion-thinking \u2026",
    "notes": "positive: decision dilemma"
  }
]
```

This makes **one file** both darwin-compatible and book2skill-complete.
`expected` (darwin) doubles as book2skill's `expected_behavior`; we do not emit a
separate `expected_behavior` key. This supersedes the object-wrapper schema in
the earlier spec draft (which had `{skill, version, test_cases:[…]}`).

### D3 — SKILL.md Segment-Parse Contract (Shared with Merge-Skills)

Rendered skills use **stable, machine-parseable segment headings**: each of the
six segments begins with a heading whose first whitespace-delimited token after
`##` is the segment tag: `## R`, `## I`, `## A1`, `## A2`, `## E`, `## B`
(decorative text may follow, e.g. `## R — Original text (Reading)`). A
deterministic parser keys off the tag. This is the contract merge-skills relies
on to extract segments without an LLM.

### D4 — Phase-4 Decoy Gate

`should_not_trigger` tolerance is **zero**: any decoy that fires blocks
acceptance regardless of overall pass rate. This is a hard gate layered over the
100% / ≥80% / \<80% ladder.

### D5 — Skillcheck Is an Optional External Tool

`skillcheck` runs via `uvx skillcheck` and depends on `uv`, which may be absent
(it is absent on the current machine). Treat it like `git` in the go-advice
subprocess pattern: locate with `exec.LookPath`; if `uv`/`skillcheck` is
unavailable, emit a clear warning and record `skillcheck: skipped` rather than
failing the run. A `--require-skillcheck` flag flips this to a hard error for CI.

### D6 — LLM Provider: Generic OpenAI-Compatible Gateway (GoModel)

The `LLM` adapter speaks the **OpenAI-compatible** `POST {base}/v1/chat/completions`
protocol to a **configurable base URL**, with `Authorization: Bearer <key>`.
Structured output uses `response_format: {"type": "json_schema", "json_schema": {name, schema, strict}}`; the model's JSON reply is validated against the same
Go-side schema (§5) and retried on mismatch.

**Default base URL is the GoModel gateway** (`github.com/ENTERPILOT/GoModel`,
"AI Gateway in Go") — a unified OpenAI-/Anthropic-compatible proxy in front of
OpenAI, Anthropic, Gemini, DeepSeek, xAI, Groq, etc. Pointing book2skill at
GoModel makes it provider-agnostic and inherits GoModel's failover, budget,
caching, and observability. The same adapter works against any OpenAI-compatible
endpoint (api.openai.com, a local Ollama, etc.) by changing `--llm-base-url`.

GoModel is used **as a running gateway over HTTP, not as a Go import**: its
module path is the unversioned `gomodel` and all its code lives under
`internal/`, so Go's internal-package rule forbids importing it from another
module. The HTTP boundary is the integration point, and it keeps book2skill's
only LLM dependency a narrow interface (§5) that is trivially faked in tests.

**Two drivers behind the same `LLM` seam.** `--driver http` (above) self-drives.
`--driver agent` makes book2skill fully **agent-agnostic**: a second `LLM`
implementation (`internal/llm/agent`) makes no network calls — on each request it
returns an agent-supplied response from a content-addressed cache, or records the
request as a pending prompt and returns a `book2skill.DeferredError`. The
`distill` command runs the pipeline; on a deferred result it prints the pending
prompts as a JSON Action (each with `messages`, `schema`, and a `response_path`)
plus a `resume` command, and exits 0. The agent runs the prompts with *its own*
model, writes replies to the cache paths, and re-invokes `resume`; the cache is
the only state, so the loop is resumable and idempotent. Per-item stages fan out
so each round emits one parallelizable batch. The agent loop is documented in
`SKILL.md`.

______________________________________________________________________

## 3. Target Package Layout

Root stays `package main` (climax requirement), so the domain "shared language"
lives in one internal domain package rather than the module root. go-advice §1
permits a named domain package once the domain is large enough; this pipeline
qualifies. `internal/` prevents external import and gives test seams (§10).

```text
book2skill/                         (module root, package main)
  main.go                           — entry (present)
  cmd/
    cmd.go                          — dispatcher (present)
    root/root.go                    — shared Config, ExitError (present)
    version/version.go              — version cmd (present)
    distill/distill.go              — NEW: `book2skill distill <book>` — runs the pipeline
  internal/
    book2skill/                     — DOMAIN package: the shared language (no I/O, no 3rd-party)
      doc.go
      book.go                       — BookOverview, CandidateUnit, VerifiedUnit, Skill, Relationship
      testprompt.go                 — TestCase, TestType, marshaling to the darwin array shape (D2)
      quote.go                      — QuoteMaxRunes, script detection, ValidateQuote (D1)
      skillmd.go                    — Segment tags, render + parse contract (D3)
      error.go                      — Error type + codes (go-advice §3)
      llm.go                        — LLM interface, request/response value types (point-of-use, §5)
    pipeline/                       — orchestration (imperative shell) + pure stage functions
      pipeline.go                   — Pipeline struct: injected deps (LLM, FS, Clock, Prompter, Checker)
      stage0_overview.go            … stage4_stresstest.go
      *_test.go                     — table-driven tests for the pure parts
    llm/openai/                     — OpenAI-compatible HTTP adapter (implements book2skill.LLM);
                                      default base URL = GoModel gateway (D6)
    prompts/                        — embedded extractor/stage prompt templates (go:embed)
    store/                          — filesystem adapter: read book text, write books/<slug>/… tree
    skillcheck/                     — exec.LookPath("uv") + `uvx skillcheck` wrapper (D5)
    render/                         — template substitution for BOOK_OVERVIEW/SKILL/INDEX/test-prompts
```

Rules enforced (go-advice §1): subpackages do not import each other laterally;
they depend only on `internal/book2skill` (the domain). `cmd/distill` and `main`
are the only composition points that import concrete adapters together.

______________________________________________________________________

## 4. Domain Types and Interfaces (`internal/book2skill`)

Plain values + interfaces only; no `os`, `net/http`, or vendor imports here.

- `BookOverview` — the stage-0 structure/interpretation/critique/applicability
  fields (spec §7). Includes `Critique` (feeds every skill's Boundary).
- `CandidateUnit` — `ID, Title, Type, SourceChapter, SourceQuote, Summary, Tags`
  - type-specific optional fields (`BoundTo`, `Outcome`, `FailureMode`,
    `Mechanism`, `WarningSigns`, `AuthorDefinition`, `KeyDistinction`,
    `WhyItMatters`). `Type` is a small enum with a validated constructor
    (go-advice §4: model constraints in types).
- `Validation` — `V1CrossDomain{Passed, Evidence []Context}`, `V2PredictivePower`,
  `V3Exclusivity`; `VerifiedUnit` = CandidateUnit + Validation.
- `Skill` — the six segments (`R, I, A1[], A2, E[], B`) + `Slug, Description, Tags, Provenance, Related []Relationship`.
- `Relationship{From, To, Kind}` where Kind ∈ {DependsOn, ContrastsWith, ComposesWith}.
- `TestCase{ID int, Type TestType, Prompt, Expected, Notes}` with `TestType` ∈
  {ShouldTrigger, ShouldNotTrigger, EdgeCase}; custom `MarshalJSON` emits the
  darwin array element (D2).
- `Error` — go-advice §3: `Code, Message, Op, Err`; `ErrorCode()/ErrorMessage()`
  helpers; codes `EINVALID, ENOTFOUND, EINTERNAL, ECONFLICT, EUNAUTHORIZED`
  (start with five, add as needed). Wrap with `Op = "pipeline.Stage2.Render"`.

**Interfaces defined at point of use** (go-advice §10, narrow):

- `LLM` (see §5).
- `Prompter` — user confirmation gate: `Confirm(ctx, prompt) (bool, error)`.
- `Clock` — `Now()`; injected, never `time.Now()` in core (go-advice §11).
- Filesystem access via `io/fs` + a small writer interface, injected.

Every interface method gets a godoc comment naming its error codes; interface
comments written **before** bodies (go-advice §4).

______________________________________________________________________

## 5. The LLM Boundary

One narrow interface in the domain package:

```go
// LLM performs a single structured completion. impl in internal/llm/anthropic.
type LLM interface {
	// Complete sends prompt+system and returns JSON bytes conforming to schema.
	// Returns EINVALID if the model output fails schema validation after retries.
	Complete(ctx context.Context, req LLMRequest) (json.RawMessage, error)
}

type LLMRequest struct {
	System      string
	Prompt      string
	Schema      json.RawMessage // JSON Schema the output must satisfy
	Temperature float64
}
```

- Schema validation + retry (`MaxLLMRetries`, default 3) lives in a **pure
  helper** wrapping the interface, so it is table-testable with a fake `LLM`.
- Each stage builds its `LLMRequest` from an embedded prompt template
  (`internal/prompts`, `go:embed`) + injected context (BookOverview, text chunk).
- Parallel extractors (stage 1) run as five concurrent `Complete` calls via
  `errgroup`; results are immutable values merged by the caller (go-advice §11:
  pass immutable values, single owner merges).

Determinism: temperature ~0.2 for judgment/validation, ~0.5 for generative
rendering. Not correctness-critical.

______________________________________________________________________

## 6. Command Surface

- `book2skill distill <book-path>` — primary command. Flags (every knob is a
  flag — go-advice CLI rule): `--title`, `--author`, `--year`, `--out`
  (default `books/`), `--slug`, `--bulk` (default false → pilot one unit),
  `--llm-base-url` (default the GoModel gateway), `--api-key` (env
  `BOOK2SKILL_API_KEY`), `--model`, `--max-chunk-tokens`, `--quote-max-runes`,
  `--require-skillcheck`, `--yes` (skip confirmation gates, for non-interactive
  runs).
- Later, optionally: `book2skill validate <skill-dir>` (run skillcheck + the
  Phase-4 harness on an existing skill). Not in the first milestone set.

`cmd/distill` wires adapters (LLM, store, skillcheck, prompter, clock) onto a
`pipeline.Pipeline` and calls `Run`. Business logic stays out of `cmd/` and
`main` (go-advice §1, §7).

______________________________________________________________________

## 7. Functional Core / Imperative Shell Split (Go-Advice §5)

**Pure core (values in, values out — no I/O, unit-tested first):**

- Quote validation + script detection (D1).
- SKILL.md render and parse (D3); template substitution (`internal/render`).
- Candidate dedup by exact/normalized quote match.
- Pass-rate arithmetic and gate evaluation (D4).
- Topological sort of `depends-on` for learning order; relationship-count sanity.
- Mermaid graph emission.
- Frontmatter allow-list enforcement; `description` length/plaintext checks.
- LLM-output schema validation.

**Imperative shell (thin, few branches):**

- Reading book text + chunking; writing the `books/<slug>/…` tree.
- LLM calls; skillcheck subprocess; user confirmation prompts.
- Stage sequencing and the parallel-extractor fan-out.

Rhythm when logic accretes in the shell: extract to a pure function via TDD,
then delete the shell code (go-advice §5 two-commit sequence).

______________________________________________________________________

## 8. Implementation Milestones (Each Is a Lint-Clean, Tested Checkpoint)

Ordered so every milestone compiles, passes `golangci-lint`, and has tests for
any pure logic it adds. After each: run `golangci-lint run` (no rule
relaxation), review against go-advice, improve, then proceed (per the user's
process).

- **M0 — Baseline green.** `go build ./...`, `go vet ./...`, `golangci-lint run`
  on the untouched scaffold. Record the baseline; fix any pre-existing lint.
  Add `.golangci.yaml` review (it already exists — do not relax it).
- **M1 — Domain package.** `internal/book2skill`: `error.go`, `book.go`,
  `testprompt.go` (+ MarshalJSON → darwin array), `quote.go` (script detect +
  validate), `skillmd.go` (segment tags + parse/render contract), `llm.go`
  (interfaces). Table-driven tests for quote validation, testprompt marshaling
  (golden JSON asserting darwin shape), and SKILL.md parse↔render round-trip.
- **M2 — render + prompts.** `internal/render` (template substitution for all
  four templates) and `internal/prompts` (embed the existing `extractors/*.md`
  and add stage prompts). Golden-file tests for rendering.
- **M3 — LLM adapter + retry helper.** `internal/llm/openai`: an
  OpenAI-compatible `/v1/chat/completions` client implementing `LLM`, with
  `response_format: json_schema` structured output, `Authorization: Bearer`,
  configurable base URL (default GoModel gateway, D6), and `context`-aware
  requests (`noctx`/`contextcheck`). A pure schema-validate-and-retry helper
  wraps it. Tests use `net/http/httptest` (go-advice: real connections, never
  mock `net.Conn`) plus a fake `LLM` for the retry helper.
- **M4 — store + skillcheck adapters.** `internal/store` (book text load +
  chunk; output tree writer) and `internal/skillcheck` (LookPath-guarded
  `uvx skillcheck`, D5). Tests: chunking on chapter boundaries; skillcheck
  wrapper with a helperProcess mock (go-advice §10 subprocess testing).
- **M5 — pipeline stage 0.** Analytical reading → `BookOverview` → render
  `BOOK_OVERVIEW.md`; quality gate; confirmation gate via `Prompter`.
- **M6 — stage 1 + 1.5.** Parallel extractors (errgroup) → candidates; dedup;
  triple validation → `verified.md` + `rejected/`.
- **M7 — stage 2.** Render six segments per verified unit → `SKILL.md`;
  frontmatter allow-list; run skillcheck; fix-loop scaffolding.
- **M8 — stage 3.** Relationship detection → per-skill `## Related skills` +
  `INDEX.md` (mermaid + topo-sorted learning order).
- **M9 — stage 4.** Test-case generation (darwin array via D2) → run trigger
  simulations → score against gates (D4) → `test-results.md`; rework routing.
- **M10 — `distill` command wiring + end-to-end.** Compose adapters in
  `cmd/distill`; `climax lint` clean; an end-to-end test with a fake LLM over a
  tiny fixture "book" asserting the full `books/<slug>/` tree.

______________________________________________________________________

## 9. Testing Strategy (Go-Advice §9–§10)

- stdlib `testing` only; three helpers (`assert/ok/equals`); no assertion libs.
- Table-driven with `t.Run`; long flat self-contained tests over helper-heavy.
- Golden files (`-update` flag) for rendered markdown/JSON.
- Pure core gets property-ish tests where invariants are clear (parse∘render =
  identity for skills; quote validator boundary preservation).
- LLM/store/skillcheck exercised via injected fakes and the helperProcess
  subprocess pattern; **never** mock what a real temp dir or real subprocess can
  do cheaply.
- `t.Parallel()` only where a test owns a fully isolated temp dir and no globals
  (go-advice resolution). No `t.SetEnv`. No `time.Sleep`.
- Test difficulty is a design signal: if a stage is hard to test, push its logic
  into the pure core and inject the rest.

______________________________________________________________________

## 10. Process / Tooling Notes

- After every milestone: `gofmt`/`goimports`, `go vet ./...`,
  `golangci-lint run` — **do not** edit `.golangci.yaml` to relax rules; fix the
  code. If a lint appears wrong, justify in the review notes rather than
  disabling it.
- `climax lint` after touching scaffold files to catch structural drift.
- uv is missing on this machine; `uvx skillcheck` will not run here — M4 must
  degrade gracefully (D5), and M7's skillcheck step is exercised in tests via a
  fake checker, not the real binary.

______________________________________________________________________

## 11. How Go-Advice/summary_rules.md Shaped This Plan (The Improvement Pass)

- **§1 layout:** kept `main` at root + `cmd/` (CLI tool), pushed the domain into
  a single `internal/book2skill` package rather than scattering types by role;
  adapters named after what they wrap (`anthropic`, `store`, `skillcheck`),
  none importing each other.
- **§3 errors:** single `Error{Code,Message,Op,Err}` type, `Op`-wrapping,
  translate external errors at adapter boundaries; no `(nil,nil)` lookups.
- **§4 interfaces:** narrow interfaces at point of use; model the `Type`/`Kind`
  enums and quote constraint in types with validated constructors; write
  interface comments before bodies.
- **§5 FCIS:** explicit pure-core vs shell split (§7); LLM/FS/clock injected;
  the retry/validate wrapper is pure and fully testable.
- **§7 CLI shape B:** confirmed the existing `main`/`run`/dispatcher shape is
  correct; every configurable knob is a registered flag; writes go to
  `cfg.Stdout`/`Stderr`; `ExitError` not `os.Exit`.
- **§10 testing mechanics:** stdlib-only, golden files, helperProcess for
  skillcheck, `exec.LookPath` (never hardcode `uv`), narrow test seams.
- **§11 concurrency:** stage-1 fan-out passes immutable values via `errgroup`;
  a single owner merges; inject `Clock` instead of `time.Now()`.
- **§13 naming/comments:** godoc on exported types/methods incl. error codes;
  comments explain *why*, not *what*.
- **§15 philosophy:** design-it-twice on the LLM boundary and the segment
  contract; delete shell code once logic is extracted; keep the critical path
  (parse/render/validate) branch-light.

______________________________________________________________________

## 12. Open Items to Confirm During Implementation

- Default model id and whether GoModel's `json_schema` `strict` mode is honored
  for every backing provider (some providers downgrade to `json_object`); the
  retry/validate helper must tolerate a non-strict provider.
- Whether `distill` should support resume (re-run from a completed stage using
  the on-disk `books/<slug>/` artifacts) — likely M11, not in the core set.
- Exact `.golangci.yaml` enabled linters (read at M0) so code is written to pass
  them the first time (e.g. `wrapcheck`, `exhaustive`, `err113`).
