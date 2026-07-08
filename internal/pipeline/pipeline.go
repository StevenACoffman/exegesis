// Package pipeline runs the RIA-TV++ distillation: it sequences the six stages
// that turn one book into a set of executable skills. Side-effecting
// dependencies (LLM, filesystem, clock, confirmation prompt, skillcheck) are
// injected as function fields so the whole pipeline runs under test with
// in-memory fakes.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

const (
	defaultMaxLLMRetries = 3
	defaultTemperature   = 0.3
)

// Config carries the run parameters for one book.
type Config struct {
	Title         string
	Author        string
	Year          string
	Slug          string
	Model         string
	QuoteMaxRunes int
	MaxChunkRunes int
	MaxLLMRetries int
	Bulk          bool
	AutoConfirm   bool
}

// Result summarizes a completed run.
type Result struct {
	Slug              string
	SkillCount        int
	RejectedCount     int
	Warnings          []string
	SkillcheckSkipped bool
}

// Pipeline holds the injected dependencies and configuration for a run.
type Pipeline struct {
	// LLM performs structured completions.
	LLM book2skill.LLM
	// WriteFile writes an artifact at a path relative to the book's output dir.
	WriteFile func(rel string, data []byte) error
	// Confirm asks the operator to approve continuing past a gate.
	Confirm func(ctx context.Context, question string) (bool, error)
	// Check runs skillcheck over a rendered skill dir, reporting whether it was
	// skipped (uv absent) and any validation error.
	Check func(ctx context.Context, dir string) (bool, error)
	// Cfg is the run configuration.
	Cfg Config
}

// Run executes the full pipeline over bookText and returns a summary. It stops
// with an error at the Stage-0 quality gate or if the operator declines to
// continue.
func (p *Pipeline) Run(ctx context.Context, bookText string) (*Result, error) {
	res := &Result{Slug: p.Cfg.Slug}

	overview, err := p.stage0Overview(ctx, bookText)
	if err != nil {
		return nil, err
	}
	if problems := overview.QualityGate(); len(problems) > 0 {
		return nil, &book2skill.Error{
			Code:    book2skill.EINVALID,
			Message: "stage 0 quality gate failed: " + strings.Join(problems, "; "),
		}
	}
	if err := p.confirmSkeleton(ctx, overview); err != nil {
		return nil, err
	}

	candidates, err := p.stage1Extract(ctx, bookText, overview)
	if err != nil {
		return nil, err
	}

	verified, rejected, err := p.stage15Validate(ctx, candidates, bookText)
	if err != nil {
		return nil, err
	}
	res.RejectedCount = rejected

	skills, skipped, err := p.stage2Construct(ctx, verified)
	if err != nil {
		return nil, err
	}
	res.SkillCount = len(skills)
	res.SkillcheckSkipped = skipped

	if err := p.stage3Link(ctx, overview, skills); err != nil {
		return nil, err
	}
	if err := p.stage4Test(ctx, skills); err != nil {
		return nil, err
	}
	return res, nil
}

// confirmSkeleton enforces the post-Stage-0 human confirmation gate unless the
// run is configured to auto-confirm.
func (p *Pipeline) confirmSkeleton(ctx context.Context, o *book2skill.BookOverview) error {
	if p.Cfg.AutoConfirm || p.Confirm == nil {
		return nil
	}
	ok, err := p.Confirm(ctx, "Stage 0 complete for "+o.Title+". Continue to extraction?")
	if err != nil {
		return &book2skill.Error{Op: "pipeline.confirmSkeleton", Err: err}
	}
	if !ok {
		return &book2skill.Error{
			Code:    book2skill.ECONFLICT,
			Message: "aborted by operator at the stage 0 confirmation gate",
		}
	}
	return nil
}

func (p *Pipeline) retries() int {
	if p.Cfg.MaxLLMRetries > 0 {
		return p.Cfg.MaxLLMRetries
	}
	return defaultMaxLLMRetries
}

// writeJSON marshals v as indented JSON and writes it to rel.
func (p *Pipeline) writeJSON(rel string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return &book2skill.Error{Op: "pipeline.writeJSON", Err: err}
	}
	return p.WriteFile(rel, b)
}

// complete runs a single structured completion built from system and user
// prompts, decoding the reply into T with retry.
func complete[T any](ctx context.Context, p *Pipeline, system, user string) (T, error) {
	req := &book2skill.LLMRequest{
		Model: p.Cfg.Model,
		Messages: []book2skill.Message{
			{Role: book2skill.RoleSystem, Content: system},
			{Role: book2skill.RoleUser, Content: user},
		},
		Temperature: defaultTemperature,
	}
	return book2skill.CompleteInto[T](ctx, p.LLM, req, p.retries())
}

// jsonString marshals v to a compact JSON string for embedding in a prompt. It
// returns "{}" if marshaling somehow fails, which cannot happen for the plain
// value types passed here.
func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func typePrefix(t book2skill.CandidateType) string {
	switch t {
	case book2skill.TypeFramework:
		return "f"
	case book2skill.TypePrinciple:
		return "p"
	case book2skill.TypeCase:
		return "c"
	case book2skill.TypeCounterExample:
		return "ce"
	case book2skill.TypeTerm:
		return "g"
	default:
		return "x"
	}
}

func candidateID(t book2skill.CandidateType, n int) string {
	return fmt.Sprintf("%s%02d", typePrefix(t), n)
}
