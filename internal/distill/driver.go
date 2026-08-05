package distill

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
)

// maxRounds bounds the http driver so an answerer that never satisfies a stage
// (e.g. an overview that keeps failing its gate) cannot loop forever.
const maxRounds = 40

// Answerer sends a prompt's messages to a model and returns the JSON reply,
// which must satisfy the prompt's Schema. It is defined at the point of use so
// tests can supply their own; HTTPAnswerer is the production implementation.
type Answerer interface {
	Answer(ctx context.Context, p *PromptRequest) ([]byte, error)
}

// RunHTTP drives the pipeline to completion, using ans to answer each pending
// prompt and writing every reply into the cache, and returns the terminal
// (complete) Outcome. Run stays the pure round; this is the imperative loop.
//
// It stops with an error if a round re-emits the exact prompts of the previous
// round (the answers are not advancing the pipeline) or if it exceeds maxRounds.
func RunHTTP(ctx context.Context, tree, bookPath, resume string, ans Answerer) (Outcome, error) {
	var prev string
	for round := 0; round < maxRounds; round++ {
		out, err := Run(tree, bookPath, resume)
		if err != nil {
			return Outcome{}, err
		}
		if out.Status == StatusComplete {
			return out, nil
		}
		sig := promptSignature(out.Prompts)
		if sig == prev {
			return Outcome{}, fmt.Errorf(
				"stage %q did not advance after answering its prompts",
				out.Stage,
			)
		}
		prev = sig
		if err := answerBatch(ctx, ans, out.Prompts); err != nil {
			return Outcome{}, err
		}
	}
	return Outcome{}, fmt.Errorf("did not complete within %d rounds", maxRounds)
}

// answerBatch answers each prompt and writes the reply to its ResponsePath.
func answerBatch(ctx context.Context, ans Answerer, prompts []PromptRequest) error {
	for i := range prompts {
		p := &prompts[i]
		reply, err := ans.Answer(ctx, p)
		if err != nil {
			return fmt.Errorf("answer %s: %w", p.ID, err)
		}
		if err := os.WriteFile(p.ResponsePath, reply, 0o644); err != nil {
			return fmt.Errorf("cache reply %s: %w", p.ID, err)
		}
	}
	return nil
}

// promptSignature is a stable key for a batch of prompts: their sorted IDs.
func promptSignature(prompts []PromptRequest) string {
	ids := make([]string, len(prompts))
	for i := range prompts {
		ids[i] = prompts[i].ID
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}
