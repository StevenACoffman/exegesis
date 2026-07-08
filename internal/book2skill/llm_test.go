package book2skill_test

import (
	"context"
	"errors"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

// fakeLLM returns scripted replies and errors, one per call.
type fakeLLM struct {
	replies [][]byte
	errs    []error
	calls   int
}

type answerPayload struct {
	Answer string `json:"answer"`
}

func (f *fakeLLM) Complete(_ context.Context, _ *book2skill.LLMRequest) ([]byte, error) {
	i := f.calls
	f.calls++
	if i < len(f.errs) && f.errs[i] != nil {
		return nil, f.errs[i]
	}
	return f.replies[i], nil
}

func TestCompleteIntoRetriesUntilValid(t *testing.T) {
	t.Parallel()
	llm := &fakeLLM{
		replies: [][]byte{[]byte("not json at all"), []byte(`{"answer":"42"}`)},
	}
	got, err := book2skill.CompleteInto[answerPayload](
		context.Background(), llm, &book2skill.LLMRequest{}, 2,
	)
	if err != nil {
		t.Fatalf("CompleteInto: %v", err)
	}
	if got.Answer != "42" {
		t.Errorf("Answer = %q, want %q", got.Answer, "42")
	}
	if llm.calls != 2 {
		t.Errorf("calls = %d, want 2", llm.calls)
	}
}

func TestCompleteIntoExhaustsRetries(t *testing.T) {
	t.Parallel()
	llm := &fakeLLM{replies: [][]byte{[]byte("nope"), []byte("still nope")}}
	_, err := book2skill.CompleteInto[answerPayload](
		context.Background(), llm, &book2skill.LLMRequest{}, 1,
	)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if book2skill.ErrorCode(err) != book2skill.EINVALID {
		t.Errorf("ErrorCode = %q, want %q", book2skill.ErrorCode(err), book2skill.EINVALID)
	}
}

func TestCompleteIntoPropagatesTransportError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("transport boom")
	llm := &fakeLLM{replies: [][]byte{nil}, errs: []error{sentinel}}
	_, err := book2skill.CompleteInto[answerPayload](
		context.Background(), llm, &book2skill.LLMRequest{}, 3,
	)
	if err == nil {
		t.Fatal("expected transport error")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error chain missing sentinel: %v", err)
	}
	if llm.calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry on transport error)", llm.calls)
	}
}
