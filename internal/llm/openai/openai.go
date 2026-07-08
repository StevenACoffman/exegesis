// Package openai implements book2skill.LLM against an OpenAI-compatible
// chat/completions endpoint. The default base URL targets the GoModel gateway
// (github.com/ENTERPILOT/GoModel), a unified OpenAI-/Anthropic-compatible proxy,
// but any OpenAI-compatible endpoint works by changing the base URL.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

const (
	// DefaultBaseURL is the local GoModel gateway address.
	DefaultBaseURL = "http://localhost:8080"

	completionsPath = "/v1/chat/completions"
	defaultTimeout  = 120 * time.Second
)

// Client is an OpenAI-compatible chat/completions client.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New returns a Client targeting baseURL — falling back to DefaultBaseURL when
// empty — authenticated with apiKey. When httpClient is nil a client with a
// default timeout is used.
func New(baseURL, apiKey string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    httpClient,
	}
}

// Complete implements book2skill.LLM by POSTing an OpenAI-compatible
// chat/completions request and returning the first choice's message content.
func (c *Client) Complete(ctx context.Context, req *book2skill.LLMRequest) ([]byte, error) {
	const op = "openai.Client.Complete"

	body, err := buildRequestBody(req)
	if err != nil {
		return nil, &book2skill.Error{Op: op, Err: err}
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+completionsPath, bytes.NewReader(body),
	)
	if err != nil {
		return nil, &book2skill.Error{Op: op, Err: err}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, &book2skill.Error{Op: op, Err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &book2skill.Error{Op: op, Err: err}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &book2skill.Error{
			Code: codeForStatus(resp.StatusCode),
			Message: "llm endpoint returned status " + strconv.Itoa(resp.StatusCode) +
				": " + string(respBody),
		}
	}
	return extractContent(respBody)
}

// buildRequestBody marshals req into an OpenAI-compatible chat/completions body.
// A JSON Schema, when present, is sent as response_format.json_schema so
// providers that support structured outputs enforce it.
func buildRequestBody(req *book2skill.LLMRequest) ([]byte, error) {
	type jsonSchema struct {
		Name   string          `json:"name"`
		Schema json.RawMessage `json:"schema"`
		Strict bool            `json:"strict"`
	}
	type responseFormat struct {
		Type       string      `json:"type"`
		JSONSchema *jsonSchema `json:"json_schema,omitempty"`
	}
	type chatRequest struct {
		Model          string               `json:"model"`
		Messages       []book2skill.Message `json:"messages"`
		Temperature    float64              `json:"temperature,omitempty"`
		ResponseFormat *responseFormat      `json:"response_format,omitempty"`
	}

	cr := chatRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
	}
	if len(req.Schema) > 0 {
		name := req.SchemaName
		if name == "" {
			name = "response"
		}
		cr.ResponseFormat = &responseFormat{
			Type:       "json_schema",
			JSONSchema: &jsonSchema{Name: name, Schema: req.Schema, Strict: true},
		}
	}

	b, err := json.Marshal(cr)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}
	return b, nil
}

// extractContent returns the content of the first choice's message.
func extractContent(respBody []byte) ([]byte, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("decode completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("completion contained no choices")
	}
	return []byte(resp.Choices[0].Message.Content), nil
}

// codeForStatus maps an HTTP status to a book2skill error code.
func codeForStatus(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return book2skill.EUNAUTHORIZED
	case http.StatusNotFound:
		return book2skill.ENOTFOUND
	case http.StatusConflict:
		return book2skill.ECONFLICT
	default:
		if status >= http.StatusBadRequest && status < http.StatusInternalServerError {
			return book2skill.EINVALID
		}
		return book2skill.EINTERNAL
	}
}
