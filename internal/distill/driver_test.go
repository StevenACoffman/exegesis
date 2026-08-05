package distill_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/distill"
)

// fakeAnswerer returns a valid canned reply for each stage, keyed off the
// prompt's schema.
type fakeAnswerer struct{}

// errAnswerer always fails.
type errAnswerer struct{}

// stuckAnswerer always returns a gate-failing overview, so the pipeline never
// advances past Stage 0.
type stuckAnswerer struct{}

func (fakeAnswerer) Answer(_ context.Context, p *distill.PromptRequest) ([]byte, error) {
	switch s := string(p.Schema); {
	case strings.Contains(s, "skeleton"):
		return json.Marshal(validResponse())
	case strings.Contains(s, "units"):
		return json.Marshal(extractResponse())
	case strings.Contains(s, "skills"):
		return json.Marshal(constructResponse())
	default:
		return nil, errors.New("unexpected schema")
	}
}

func (errAnswerer) Answer(context.Context, *distill.PromptRequest) ([]byte, error) {
	return nil, errors.New("boom")
}

func (stuckAnswerer) Answer(context.Context, *distill.PromptRequest) ([]byte, error) {
	return json.Marshal(sparseResponse())
}

func TestRunHTTPDrivesToComplete(t *testing.T) {
	t.Parallel()
	tree, bookPath := writeBook(t)

	out, err := distill.RunHTTP(context.Background(), tree, bookPath, "resume", fakeAnswerer{})
	if err != nil {
		t.Fatalf("RunHTTP: %v", err)
	}
	if out.Status != distill.StatusComplete {
		t.Fatalf("want complete, got %+v", out)
	}
	for _, p := range []string{"INDEX.md", "reverse-thinking/SKILL.md", "reverse-thinking/test-prompts.json"} {
		if _, statErr := os.Stat(filepath.Join(tree, p)); statErr != nil {
			t.Errorf("expected %s: %v", p, statErr)
		}
	}
}

func TestRunHTTPPropagatesAnswerError(t *testing.T) {
	t.Parallel()
	tree, bookPath := writeBook(t)
	_, err := distill.RunHTTP(context.Background(), tree, bookPath, "resume", errAnswerer{})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want the answerer error propagated, got %v", err)
	}
}

func TestRunHTTPStopsWhenStuck(t *testing.T) {
	t.Parallel()
	tree, bookPath := writeBook(t)
	_, err := distill.RunHTTP(context.Background(), tree, bookPath, "resume", stuckAnswerer{})
	if err == nil || !strings.Contains(err.Error(), "did not complete") {
		t.Fatalf("want a bounded-loop error, got %v", err)
	}
}

func TestHTTPAnswerer(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["model"] != "m" {
			t.Errorf("request model = %v, want m", req["model"])
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"ok\":true}"}}]}`))
	}))
	defer srv.Close()

	a := &distill.HTTPAnswerer{Client: srv.Client(), Endpoint: srv.URL, Model: "m", APIKey: "k"}
	got, err := a.Answer(context.Background(), &distill.PromptRequest{
		Schema:   []byte(`{}`),
		Messages: []distill.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if string(got) != `{"ok":true}` {
		t.Errorf("Answer = %q, want the message content", got)
	}
}

func TestHTTPAnswererErrorStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("nope"))
	}))
	defer srv.Close()

	a := &distill.HTTPAnswerer{Client: srv.Client(), Endpoint: srv.URL, Model: "m", APIKey: "k"}
	_, err := a.Answer(context.Background(), &distill.PromptRequest{Schema: []byte(`{}`)})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("want a 500 error, got %v", err)
	}
}
