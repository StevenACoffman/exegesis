package pipeline

import "github.com/StevenACoffman/exegesis/internal/book2skill"

// Stage prompts. Kept as consts so the package holds no mutable global state.
// Each prompt tells the model to reply with only a JSON object whose keys match
// the json tags of the Go type the reply is decoded into (see stages.go).

const overviewSystem = `You perform Mortimer Adler's analytical reading of a book.
Read the provided text and extract the author's actual framework — not your own
opinions, and using the author's own term definitions, not dictionary ones. The
critique step is the most important: find the author's genuine limitations.

Reply with ONLY a JSON object with these keys:
{
  "structure": {"genre","one_sentence_summary","skeleton":[3-7 items],
                "argument_relationship","core_problem"},
  "interpretation": {"key_terms":[{"term","author_definition","differs_from_common"}] (>=5),
                     "core_propositions":[5-15 items],"argument_chain"},
  "critique": {"era_limitations":[],"author_blind_spots":[],
               "unproven_assumptions":[],"strongest_objection"} (>=3 limitations total),
  "applicability": {"skillable_topics":[],"non_skillable_content":[],
                    "estimated_skill_count_low","estimated_skill_count_high",
                    "priority_ranking":[]}
}`

const extractSystem = `You are one of several independent extractors distilling a book
into reusable methodology units. Extract only units in your assigned category.
Over-extract rather than screen; a later stage validates. Every unit needs a
verbatim source quote no longer than the given rune limit.

Reply with ONLY a JSON object: {"candidates": [ {
  "title","type","source_chapter","source_quote","summary","tags":[],
  "bound_to":[],"outcome","failure_mode","mechanism","warning_signs":[],
  "author_definition","key_distinction","why_it_matters"
} ]}. Populate only the fields relevant to your category; leave the rest empty.`

const validateSystem = `You screen one candidate methodology unit for whether it deserves
to be a standalone skill. Apply three tests strictly:
V1 cross-domain: the book supports it with evidence in >=2 independent contexts.
V2 predictive power: it answers a question the book does not explicitly address.
V3 exclusivity: it is the author's distinctive insight, not common sense.

Reply with ONLY a JSON object:
{"v1_cross_domain":{"passed","evidence":[{"location","summary"}]},
 "v2_predictive_power":{"passed","novel_question","derived_answer"},
 "v3_exclusivity":{"passed","why_not_common"}}`

const constructSystem = `You construct one executable skill from a validated methodology
unit, in the RIA++ form. The description must be third person, at most 1024
characters, plain text, naming when to invoke and when not to. Execution steps
must be concrete actions with measurable completion criteria.

Reply with ONLY a JSON object:
{"slug"(kebab-case),"title","description","tags":[],
 "reading":{"quote","attribution"},
 "interpretation",
 "application":[{"name","problem","methodology_use","conclusion","result"}],
 "trigger":{"scenarios":[],"language_signals":[],
            "adjacent_distinctions":[{"skill","difference"}]},
 "execution":[{"text","completion_criterion","stop_condition"}],
 "boundary":{"anti_scenarios":[],"author_warned_failures":[],
             "author_blind_spots":[],"confusable_neighbors":[]}}`

const relateSystem = `Given a list of skills (slug + summary), identify genuine
relationships. Kinds: depends-on, contrasts-with, composes-with. Do not invent
relationships; sparse and real beats dense and fake.

Reply with ONLY a JSON object:
{"relationships":[{"from","to","kind","rationale"}]}`

const testGenSystem = `Design stress-test prompts for one skill. Include at least 3
should_trigger, at least 2 should_not_trigger decoys (plausible but wrong), and
at least 1 edge_case. Decoys are mandatory.

Reply with ONLY a JSON object:
{"test_cases":[{"id"(int),"type"(should_trigger|should_not_trigger|edge_case),
 "prompt","expected"(short description of expected behaviour),"notes"}]}`

// extractor describes one Phase-1 extractor: the category it owns and the
// guidance appended to the shared extract system prompt.
type extractor struct {
	kind     book2skill.CandidateType
	file     string
	guidance string
}

// extractors returns the five Phase-1 extractors in a fresh slice.
func extractors() []extractor {
	return []extractor{
		{
			book2skill.TypeFramework, "frameworks.json",
			"Category: mental models, decision frameworks, reasoning methods.",
		},
		{
			book2skill.TypePrinciple, "principles.json",
			"Category: principles, checklists, rules, maxims (directly applicable).",
		},
		{
			book2skill.TypeCase, "cases.json",
			"Category: examples the author personally applied. Each needs bound_to.",
		},
		{
			book2skill.TypeCounterExample, "counter-examples.json",
			"Category: failure modes, traps, biases. Each needs failure_mode and mechanism.",
		},
		{
			book2skill.TypeTerm, "glossary.json",
			"Category: key terms in the author's specific usage. Fill author_definition.",
		},
	}
}
