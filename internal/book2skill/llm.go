package book2skill

import (
	"context"
	"encoding/json"
	"errors"
)

// Chat message roles understood by the OpenAI-compatible LLM adapter.
const (
	// RoleSystem marks the system instruction message.
	RoleSystem = "system"
	// RoleUser marks a message from the caller.
	RoleUser = "user"
	// RoleAssistant marks a message from the model.
	RoleAssistant = "assistant"
)

// Message is one chat message. Its JSON form matches the OpenAI-compatible
// chat/completions message object, so adapters may marshal it directly.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMRequest is a single structured chat-completion request. Schema, when
// non-empty, is a JSON Schema sent to the provider as response_format; it is
// advisory, because callers validate the reply on the Go side by decoding it
// into a concrete type (see CompleteInto).
type LLMRequest struct {
	Model       string
	Messages    []Message
	Schema      json.RawMessage
	SchemaName  string
	Temperature float64
}

// LLM performs a single chat completion against an OpenAI-compatible endpoint.
// Implementations live in adapter packages (see internal/llm/openai).
type LLM interface {
	// Complete sends req and returns the assistant message content as bytes. It
	// returns an *Error with code EINVALID when the endpoint rejects the request
	// and EINTERNAL on transport or decoding failure. req must be non-nil.
	Complete(ctx context.Context, req *LLMRequest) ([]byte, error)
}

// DeferredError is returned by an LLM whose Complete records the request for an
// external agent to fulfill later (agent-driven mode) instead of answering it
// inline. Stages propagate it unchanged; a driver recognizes it via IsDeferred
// and emits the pending prompts rather than treating the run as failed.
type DeferredError struct{}

// Error implements the error interface.
func (DeferredError) Error() string { return "llm prompt deferred to agent" }

// CompleteInto runs req through llm and decodes the reply into a value of type
// T. When the reply is not valid JSON for T it appends a corrective instruction
// and retries, up to retries additional attempts. It returns EINVALID if no
// attempt yields decodable JSON, and propagates any transport error unchanged.
func CompleteInto[T any](ctx context.Context, llm LLM, req *LLMRequest, retries int) (T, error) {
	const op = "book2skill.CompleteInto"
	var zero T
	messages := req.Messages
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		try := *req
		try.Messages = messages
		raw, err := llm.Complete(ctx, &try)
		if err != nil {
			return zero, &Error{Op: op, Err: err}
		}
		var out T
		if err := json.Unmarshal(raw, &out); err != nil {
			lastErr = err
			messages = correctionMessages(req.Messages, err)
			continue
		}
		return out, nil
	}
	return zero, &Error{
		Code:    EINVALID,
		Message: "model did not return schema-valid JSON after retries: " + errText(lastErr),
	}
}

// IsDeferred reports whether err, or any error it wraps, is a DeferredError.
func IsDeferred(err error) bool {
	var d DeferredError
	return errors.As(err, &d)
}

// correctionMessages returns base with a user message appended asking the model
// to fix the JSON that failed to decode. It never mutates base.
func correctionMessages(base []Message, decodeErr error) []Message {
	out := make([]Message, 0, len(base)+1)
	out = append(out, base...)
	return append(out, Message{
		Role: RoleUser,
		Content: "Your previous reply was not valid JSON for the required schema. " +
			"Reply with only the JSON value, no prose. Decode error: " + errText(decodeErr),
	})
}

// errText returns err's message, or the empty string when err is nil.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
