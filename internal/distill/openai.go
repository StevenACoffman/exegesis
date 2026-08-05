package distill

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// HTTPAnswerer answers prompts against an OpenAI-compatible chat-completions
// endpoint, enforcing each prompt's Schema via response_format json_schema.
type HTTPAnswerer struct {
	Client   *http.Client
	Endpoint string
	Model    string
	APIKey   string
}

// chatRequest is the OpenAI-compatible chat-completions request body.
type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

// responseFormat pins the reply to the prompt's JSON Schema.
type responseFormat struct {
	Type       string     `json:"type"`
	JSONSchema jsonSchema `json:"json_schema"`
}

type jsonSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict"`
}

// chatResponse is the subset of the reply distill reads.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Answer posts p's messages to the endpoint and returns the reply content — the
// JSON the agent would otherwise have written to the response path.
func (a *HTTPAnswerer) Answer(ctx context.Context, p *PromptRequest) ([]byte, error) {
	body, err := json.Marshal(chatRequest{
		Model:    a.Model,
		Messages: p.Messages,
		ResponseFormat: &responseFormat{
			Type:       "json_schema",
			JSONSchema: jsonSchema{Name: "distill", Schema: p.Schema, Strict: true},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if a.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.APIKey)
	}
	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post to %s: %w", a.Endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("endpoint returned %s: %s", resp.Status, snippet)
	}
	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(cr.Choices) == 0 {
		return nil, errors.New("endpoint returned no choices")
	}
	return []byte(cr.Choices[0].Message.Content), nil
}
