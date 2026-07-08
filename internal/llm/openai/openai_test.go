package openai_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/llm/openai"
)

func TestClientComplete(t *testing.T) {
	t.Parallel()

	var (
		gotAuth        string
		gotPath        string
		gotContentType string
		gotBody        map[string]any
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{\"ok\":true}"}}]}`)
	}))
	t.Cleanup(srv.Close)

	client := openai.New(srv.URL, "secret-key", srv.Client())
	got, err := client.Complete(context.Background(), &book2skill.LLMRequest{
		Model:      "gpt-5-mini",
		Messages:   []book2skill.Message{{Role: book2skill.RoleUser, Content: "hi"}},
		Schema:     json.RawMessage(`{"type":"object"}`),
		SchemaName: "resp",
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if string(got) != `{"ok":true}` {
		t.Errorf("content = %q, want %q", got, `{"ok":true}`)
	}
	if gotAuth != "Bearer secret-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer secret-key")
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("path = %q, want /v1/chat/completions", gotPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	rf, ok := gotBody["response_format"].(map[string]any)
	if !ok || rf["type"] != "json_schema" {
		t.Errorf("response_format = %v, want type json_schema", gotBody["response_format"])
	}
}

func TestClientCompleteHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"bad key"}`)
	}))
	t.Cleanup(srv.Close)

	client := openai.New(srv.URL, "", srv.Client())
	_, err := client.Complete(context.Background(), &book2skill.LLMRequest{Model: "m"})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if book2skill.ErrorCode(err) != book2skill.EUNAUTHORIZED {
		t.Errorf("ErrorCode = %q, want %q", book2skill.ErrorCode(err), book2skill.EUNAUTHORIZED)
	}
}

func TestNewDefaultsBaseURL(t *testing.T) {
	t.Parallel()
	// A nil http client and empty base URL must not panic; the client is usable.
	if openai.New("", "", nil) == nil {
		t.Fatal("New returned nil")
	}
}
