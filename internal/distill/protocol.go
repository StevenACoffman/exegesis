// Package distill runs the book2skill RIA-TV++ pipeline as a resumable,
// agent-driven loop. Each round does all deterministic work and, when it needs a
// model, prints the pending prompts as JSON and stops; the invoking agent
// answers them (writing each reply to its response_path) and re-runs the
// command. A content-addressed cache on disk is the only state, so the loop is
// idempotent and resumable. This package is the pure core (protocol types,
// prompt builders, renderers) plus a thin Run shell; cmd/distill wires the CLI.
//
// Phase 1 implements the loop machinery and Stage 0 (book -> gated
// BOOK_OVERVIEW.md). Later stages slot into the same shape.
package distill

import "encoding/json"

// Outcome status values, printed as the "status" field each round.
const (
	// StatusComplete means the pipeline is finished; nothing more to do.
	StatusComplete = "complete"
	// StatusNeedsPrompts means the agent must answer Prompts and re-run Resume.
	StatusNeedsPrompts = "needs_prompts"
)

// Message is one chat message the agent must send to its model.
type Message struct {
	Role    string `json:"role"` // "system" | "user"
	Content string `json:"content"`
}

// PromptRequest is one model call the agent must satisfy: send Messages to a
// model, require a JSON reply matching Schema, and write that reply verbatim to
// ResponsePath. ID is the content-address of the prompt and the cache key.
type PromptRequest struct {
	ID           string          `json:"id"`
	Messages     []Message       `json:"messages"`
	Schema       json.RawMessage `json:"schema"`
	ResponsePath string          `json:"response_path"`
}

// Outcome is the JSON distill prints on stdout each round. On StatusNeedsPrompts
// it carries the pending Prompts and the exact Resume command to re-run; on
// StatusComplete it carries a human Summary.
type Outcome struct {
	Status  string          `json:"status"`
	Stage   string          `json:"stage,omitempty"`
	Prompts []PromptRequest `json:"prompts,omitempty"`
	Resume  string          `json:"resume,omitempty"`
	Summary string          `json:"summary,omitempty"`
}
