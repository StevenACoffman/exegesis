// Package agent implements book2skill.LLM for agent-driven mode: instead of
// calling a model, Complete looks for an agent-supplied response in a
// content-addressed cache and, when absent, records the request as a pending
// prompt and returns a DeferredError. A driver collects the pending prompts,
// emits them for the agent to fulfill, and re-runs; the cache is the only state.
package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

const hashLen = 16

// Prompt is one deferred request for the agent to fulfill. The agent must send
// Messages to its model, require a JSON reply matching Schema, and write that
// reply to ResponsePath.
type Prompt struct {
	ID           string               `json:"id"`
	ResponsePath string               `json:"response_path"`
	Schema       json.RawMessage      `json:"schema,omitempty"`
	Messages     []book2skill.Message `json:"messages"`
}

// LLM records requests instead of answering them, unless an agent-supplied
// response already exists in the cache directory.
type LLM struct {
	cacheDir string
	mu       sync.Mutex
	pending  []Prompt
	seen     map[string]bool
}

// New returns an agent-driven LLM backed by the given cache directory.
func New(cacheDir string) *LLM {
	return &LLM{cacheDir: cacheDir, seen: make(map[string]bool)}
}

// Complete returns the cached agent response for req when present; otherwise it
// records req as a pending prompt and returns a book2skill.DeferredError.
func (l *LLM) Complete(_ context.Context, req *book2skill.LLMRequest) ([]byte, error) {
	key := requestHash(req)
	path := filepath.Join(l.cacheDir, key+".json")
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}
	l.mu.Lock()
	if !l.seen[key] {
		l.seen[key] = true
		l.pending = append(l.pending, Prompt{
			ID:           key,
			ResponsePath: path,
			Schema:       req.Schema,
			Messages:     req.Messages,
		})
	}
	l.mu.Unlock()
	return nil, book2skill.DeferredError{}
}

// Pending returns a copy of the prompts recorded so far, in first-seen order.
func (l *LLM) Pending() []Prompt {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Prompt, len(l.pending))
	copy(out, l.pending)
	return out
}

// requestHash content-addresses a request over its model, messages, and schema,
// so identical requests map to a stable cache key across runs.
func requestHash(req *book2skill.LLMRequest) string {
	payload, _ := json.Marshal(struct {
		Model    string               `json:"model"`
		Messages []book2skill.Message `json:"messages"`
		Schema   json.RawMessage      `json:"schema"`
	}{req.Model, req.Messages, req.Schema})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])[:hashLen]
}
