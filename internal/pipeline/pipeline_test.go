package pipeline_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/llm/agent"
	"github.com/StevenACoffman/exegesis/internal/pipeline"
)

const overviewJSON = `{"structure":{"genre":"methodology",
"one_sentence_summary":"Invert problems to avoid failure.",
"skeleton":["a","b","c"],"argument_relationship":"progressive","core_problem":"decisions"},
"interpretation":{"key_terms":[
{"term":"t1","author_definition":"d","differs_from_common":"x"},
{"term":"t2","author_definition":"d","differs_from_common":"x"},
{"term":"t3","author_definition":"d","differs_from_common":"x"},
{"term":"t4","author_definition":"d","differs_from_common":"x"},
{"term":"t5","author_definition":"d","differs_from_common":"x"}],
"core_propositions":["p1","p2","p3","p4","p5"],"argument_chain":"chain"},
"critique":{"era_limitations":["e1"],"author_blind_spots":["b1"],
"unproven_assumptions":["u1"],"strongest_objection":"o"},
"applicability":{"skillable_topics":["inversion"],"non_skillable_content":[],
"estimated_skill_count_low":1,"estimated_skill_count_high":3,"priority_ranking":["inversion"]}}`

const skillJSON = `{"slug":"inversion-thinking","title":"Inversion Thinking",
"description":"Invoke when a user is stuck on a decision.","tags":["decision"],
"reading":{"quote":"invert","attribution":"Jacobi"},
"interpretation":"Ask what would guarantee failure, then avoid it.",
"application":[{"name":"c","problem":"p","methodology_use":"m","conclusion":"c","result":"r"}],
"trigger":{"scenarios":["stuck"],"language_signals":["how do I succeed"],"adjacent_distinctions":[]},
"execution":[{"text":"list failure modes","completion_criterion":"three listed"}],
"boundary":{"anti_scenarios":["lookup"],"author_warned_failures":[],
"author_blind_spots":[],"confusable_neighbors":[]}}`

const testsJSON = `{"test_cases":[
{"id":1,"type":"should_trigger","prompt":"p1","expected":"invoke"},
{"id":2,"type":"should_trigger","prompt":"p2","expected":"invoke"},
{"id":3,"type":"should_trigger","prompt":"p3","expected":"invoke"},
{"id":4,"type":"should_not_trigger","prompt":"d1","expected":"skip"},
{"id":5,"type":"should_not_trigger","prompt":"d2","expected":"skip"},
{"id":6,"type":"edge_case","prompt":"x","expected":"maybe"}]}`

// scriptedLLM returns canned JSON per stage, keyed on the system prompt.
type scriptedLLM struct{}

// emptyOverviewLLM always returns an empty overview, to exercise the quality gate.
type emptyOverviewLLM struct{}

func (scriptedLLM) Complete(_ context.Context, req *book2skill.LLMRequest) ([]byte, error) {
	return cannedResponse(req.Messages[0].Content)
}

// cannedResponse returns the stub reply for a stage, keyed on its system prompt.
func cannedResponse(sys string) ([]byte, error) {
	switch {
	case strings.Contains(sys, "analytical reading"):
		return []byte(overviewJSON), nil
	case strings.Contains(sys, "independent extractors"):
		return []byte(`{"candidates":[{"title":"Inversion","source_quote":"invert",` +
			`"summary":"s","tags":["t"]}]}`), nil
	case strings.Contains(sys, "screen one candidate"):
		return []byte(`{"v1_cross_domain":{"passed":true,"evidence":` +
			`[{"location":"c1","summary":"a"},{"location":"c2","summary":"b"}]},` +
			`"v2_predictive_power":{"passed":true,"novel_question":"q","derived_answer":"a"},` +
			`"v3_exclusivity":{"passed":true,"why_not_common":"w"}}`), nil
	case strings.Contains(sys, "construct one executable skill"):
		return []byte(skillJSON), nil
	case strings.Contains(sys, "identify genuine"):
		return []byte(`{"relationships":[]}`), nil
	case strings.Contains(sys, "Design stress-test"):
		return []byte(testsJSON), nil
	default:
		return nil, &book2skill.Error{Code: book2skill.EINTERNAL, Message: "unexpected prompt"}
	}
}

func TestPipelineRunProducesTree(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	files := map[string][]byte{}

	p := &pipeline.Pipeline{
		LLM: scriptedLLM{},
		WriteFile: func(rel string, data []byte) error {
			mu.Lock()
			defer mu.Unlock()
			files[rel] = data
			return nil
		},
		Check: func(context.Context, string) (bool, error) { return true, nil }, // uv absent → skipped
		Cfg: pipeline.Config{
			Title: "Poor Charlie's Almanack", Author: "Munger", Year: "2005",
			Slug: "poor-charlies-almanack", Model: "test-model",
			QuoteMaxRunes: book2skill.QuoteMaxRunesLatin, AutoConfirm: true,
		},
	}

	res, err := p.Run(context.Background(), "Invert, always invert. "+strings.Repeat("text ", 50))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if res.SkillCount == 0 {
		t.Error("expected at least one skill")
	}
	if !res.SkillcheckSkipped {
		t.Error("expected SkillcheckSkipped=true (uv absent)")
	}

	wantFiles := []string{
		"BOOK_OVERVIEW.md",
		"candidates/frameworks.json",
		"verified.json",
		"inversion-thinking/SKILL.md",
		"inversion-thinking/test-prompts.json",
		"INDEX.md",
	}
	for _, f := range wantFiles {
		if _, ok := files[f]; !ok {
			t.Errorf("expected pipeline to write %q", f)
		}
	}

	// The emitted test-prompts.json must be the darwin bare-array shape.
	var arr []map[string]any
	if err := json.Unmarshal(files["inversion-thinking/test-prompts.json"], &arr); err != nil {
		t.Fatalf("test-prompts.json is not a JSON array: %v", err)
	}
	if len(arr) != 6 {
		t.Errorf("test-prompts has %d cases, want 6", len(arr))
	}

	// The rendered SKILL.md must satisfy the segment contract.
	segments := book2skill.ParseSegments(string(files["inversion-thinking/SKILL.md"]))
	for _, tag := range book2skill.SegmentTags() {
		if _, ok := segments[tag]; !ok {
			t.Errorf("SKILL.md missing segment %q", tag)
		}
	}
}

func TestPipelineQualityGateBlocks(t *testing.T) {
	t.Parallel()
	// An LLM that returns an empty overview must trip the Stage-0 quality gate.
	p := &pipeline.Pipeline{
		LLM:       emptyOverviewLLM{},
		WriteFile: func(string, []byte) error { return nil },
		Cfg:       pipeline.Config{Slug: "x", AutoConfirm: true},
	}
	_, err := p.Run(context.Background(), "text")
	if book2skill.ErrorCode(err) != book2skill.EINVALID {
		t.Errorf("ErrorCode = %q, want %q", book2skill.ErrorCode(err), book2skill.EINVALID)
	}
}

func (emptyOverviewLLM) Complete(context.Context, *book2skill.LLMRequest) ([]byte, error) {
	return []byte(`{}`), nil
}

// TestPipelineAgentDriverLoop drives the pipeline in agent mode: each round it
// fulfills the deferred prompts by writing canned responses to their cache
// paths, then re-runs, until the run completes. This proves the batched
// content-addressed loop converges and produces the same tree.
func TestPipelineAgentDriverLoop(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	llm := agent.New(cacheDir)

	var mu sync.Mutex
	files := map[string][]byte{}
	p := &pipeline.Pipeline{
		LLM: llm,
		WriteFile: func(rel string, data []byte) error {
			mu.Lock()
			defer mu.Unlock()
			files[rel] = data
			return nil
		},
		Check: func(context.Context, string) (bool, error) { return true, nil },
		Cfg: pipeline.Config{
			Title: "Poor Charlie's Almanack", Author: "Munger", Year: "2005",
			Slug: "pca", Model: "test-model",
			QuoteMaxRunes: book2skill.QuoteMaxRunesLatin, AutoConfirm: true,
		},
	}

	res, batches := driveAgentLoop(t, p, llm)
	if res.SkillCount == 0 {
		t.Fatal("agent loop completed with no skills")
	}
	if _, ok := files["inversion-thinking/SKILL.md"]; !ok {
		t.Error("no SKILL.md written by the agent-driven loop")
	}
	// The run must progress one stage-batch at a time, not all at once.
	if batches < 2 {
		t.Errorf("expected multiple deferred batches, got %d", batches)
	}
}

// driveAgentLoop runs p in agent mode, fulfilling each round's deferred prompts
// from the canned responses until the run completes. It returns the final result
// and the number of deferred batches observed.
func driveAgentLoop(t *testing.T, p *pipeline.Pipeline, llm *agent.LLM) (*pipeline.Result, int) {
	t.Helper()
	const maxRounds = 12
	batches := 0
	for range maxRounds {
		res, err := p.Run(context.Background(), "Invert, always invert. "+strings.Repeat("t ", 40))
		if err == nil {
			return res, batches
		}
		if !book2skill.IsDeferred(err) {
			t.Fatalf("unexpected error: %v", err)
		}
		pending := llm.Pending()
		if len(pending) == 0 {
			t.Fatal("deferred but no pending prompts")
		}
		batches++
		fulfill(t, pending)
	}
	t.Fatalf("agent loop did not complete in %d rounds", maxRounds)
	return nil, batches
}

// fulfill writes a canned response to each pending prompt's cache path.
func fulfill(t *testing.T, pending []agent.Prompt) {
	t.Helper()
	for _, prompt := range pending {
		answer, err := cannedResponse(prompt.Messages[0].Content)
		if err != nil {
			t.Fatalf("no canned response: %v", err)
		}
		if err := os.WriteFile(prompt.ResponsePath, answer, 0o600); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}
}
