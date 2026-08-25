# Exegesis

The deterministic pipeline and gate CLI behind the **book2skill** skill. It distills a book
into a tree of [Agent Skills](https://agentskills.io) and **gates their structure** —
whether each skill is well formed, whether the tree's relationships resolve, whether the
test prompts are composed correctly.

It says nothing about whether a skill is any *good*. That is
[`skillsaw`](https://github.com/StevenACoffman/skillsaw)'s job, which scores quality against
a 9-dimension rubric. The two meet at the shared `test-prompts.json` contract.

## Install

```bash
go install github.com/StevenACoffman/exegesis@latest
```

## Commands

| Command         | What it does                                                                                                                                                                                |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `lint`          | Validate a skill's frontmatter, body links, and runtime neutrality. `--check redlines` adds book2skill's mechanical Quality Red Lines; `--check skilllens` adds the SkillLens quality tier. |
| `tests`         | Check a skill's `test-prompts.json` composition, scaffold one, or migrate a foreign one.                                                                                                    |
| `verify`        | Run every gate over a whole tree and emit `skills-manifest.json`.                                                                                                                           |
| `link`          | Record a related-skill edge in a skill's `## Related Skills` section.                                                                                                                       |
| `index`         | Regenerate `INDEX.md` from every skill's `## Related Skills` section.                                                                                                                       |
| `distill`       | Run the book2skill pipeline as a resumable loop (agent or http driver).                                                                                                                     |
| `scaffold`      | Write a tree of skill frames offline from a schema, gated on write.                                                                                                                         |
| `relate`        | Apply a relations edge table across a book's skills, then rebuild `INDEX.md`.                                                                                                               |
| `normalize`     | Rewrite every `## Related Skills` section into the canonical bullet format.                                                                                                                 |
| `quotecheck`    | Report R-segment quotations that appear in none of the source texts.                                                                                                                        |
| `merge-status`  | Append to, or validate, a source skill's merge ledger.                                                                                                                                      |
| `merge-index`   | Regenerate a merged tree's cross-book provenance `INDEX.md`.                                                                                                                                |
| `merge-migrate` | Move a merged skill's provenance from frontmatter into the body.                                                                                                                            |
| `a2check`       | Report the language signals a merged skill adds to its sources'.                                                                                                                            |

Run `exegesis <SUBCOMMAND> -h` for the flags each one takes.

## Related-Skill Edges

A skill declares its relationships as bullets under `## Related Skills`:

```markdown
- depends-on: `other-skill` — needs it first
```

Five kinds are valid. `depends-on` is the only one that **orders** anything — `index`
topologically sorts the learning path on those edges alone.

| Kind             | Meaning                                                                                                        |
| ---------------- | -------------------------------------------------------------------------------------------------------------- |
| `depends-on`     | The source needs the target first. Orders the learning path.                                                   |
| `composes-with`  | The two are used together.                                                                                     |
| `contrasts-with` | The two are alternatives worth comparing.                                                                      |
| `informs`        | The source shapes how the target is applied, without being needed first. Directional, but **not** an ordering. |
| `superseded-by`  | A merge run replaced the source with the target, usually in another tree.                                      |

**Reading is deliberately more tolerant than writing.** `exegesis` writes exactly one
format; the reader also accepts the dialects found in trees written before that format
settled, so a legacy section still yields its edges instead of being silently ignored.
Reading a dialect is not an endorsement of it — `normalize` rewrites them, and `lint` still
reports a parent-escaping link as a defect.

One spelling is deliberately **not** read: `prerequisite for`. It is the inverse of
`depends-on`, so the edge it declares belongs in the *target's* file — and a reader only
ever speaks for the file it is parsing. Moving those is a rewrite, not a read.

## Design

A **pure core, imperative shell**: everything in `internal/` is value-in, value-out with no
I/O, and file access and exit codes live only in the command `exec` methods. Every check is
a function from a loaded skill to diagnostics, so the same logic serves `lint`, `verify`,
and `scaffold`'s gate-on-write.

Definitions with a second consumer live in the shared
[`skillet`](https://github.com/StevenACoffman/skillet) module rather than here, so exegesis
and skillsaw cannot drift on a rule they both enforce: `speclint` owns the agentskills.io
frontmatter spec, `redlines` owns book2skill's Quality Red Lines, and `testprompts` owns the
shared `test-prompts.json` contract.

### `speclint` Is the Spec, Not the Style Guide

`speclint` gates what `agentskills.io` specifies: the required keys, the key allowlist, the
1024-character description cap. That is a machine-readable contract, and enforcing it is
exegesis's job. Anthropic also publishes *authoring guidance* — how to word a description,
how to structure a body — which arrives from the same source, reads with the same authority,
and **is not part of the contract.** exegesis does not enforce it, and a rule proposed on the
strength of that guidance alone does not belong in `speclint`.

The reason is evidence, not preference. `superpowers` draws the same line and says why: its
own head-to-head wording tests contradict parts of the published advice, so it accepts the
contract from that document and refuses the prose, declining changes made to "comply" with it
absent eval evidence that outcomes improve. This repository has since measured the same thing
twice — a mechanical form of "a description must not summarise the workflow" flags
`zero-touch-production` for containing *"Do Not Use This Skill When"*, a negative trigger
condition the guidance itself asks for; and the RIA-TV++ segment rule, applied to a
hand-written skill, reports six defects about a format the document never claimed.

So: a new rule needs a contract clause or a measurement. Guidance is a reason to read
carefully, not a reason to gate.

The two cases above are instances of a shape measured four times across this family, recorded
once in `skillet/TODO.md` under *Mechanical Forms of Good Advice*: the mechanical proxy for a
correct piece of advice selects on a surface feature that the **good** case exhibits at least
as strongly as the bad one, so no threshold separates them. The practical rule from that
record is worth repeating here, because it is what turns this section from opinion into
process — before building a mechanical form of any advice, run the predicate over the corpus
and **read what it flags, not how many**. "It is a string comparison, not a heuristic" is not
evidence of safety; two of the four were exact and deterministic.

## The Family

| Repo                                                                           | Role                                                      |
| ------------------------------------------------------------------------------ | --------------------------------------------------------- |
| [`skillet`](https://github.com/StevenACoffman/skillet)                         | The shared kernel every tool imports.                     |
| **exegesis**                                                                   | Gates a tree's structure.                                 |
| [`skillsaw`](https://github.com/StevenACoffman/skillsaw)                       | Scores skill quality and runs the keep-or-revert ratchet. |
| [`canonizer`](https://github.com/StevenACoffman/canonizer)                     | Grades rulesets rather than skills.                       |
| [`agentic-dev-harness`](https://github.com/StevenACoffman/agentic-dev-harness) | Drives a change through a five-stage arc loop.            |
