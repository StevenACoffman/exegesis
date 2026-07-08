// Package distill implements the "distill" command: it runs the RIA-TV++
// pipeline over a book file, writing the books/<slug>/ skill tree. It supports
// two LLM drivers: "http" drives the model itself over an OpenAI-compatible API
// (default: the GoModel gateway); "agent" emits the model prompts for an
// external agent to run, making book2skill agent-agnostic.
package distill

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/llm/agent"
	"github.com/StevenACoffman/exegesis/internal/llm/openai"
	"github.com/StevenACoffman/exegesis/internal/pipeline"
	"github.com/StevenACoffman/exegesis/internal/skilllint"
	"github.com/StevenACoffman/exegesis/internal/store"
)

const (
	defaultModel = "claude-sonnet-4-6"
	defaultOut   = "books"
	dirPerm      = 0o755
	cacheSubdir  = ".b2s/cache"
)

// Config holds the flags and wiring for the distill command.
type Config struct {
	*root.Config
	Title          string
	Author         string
	Year           string
	Out            string
	Slug           string
	Driver         string
	LLMBaseURL     string
	APIKey         string
	Model          string
	MaxChunkTokens int
	QuoteMaxRunes  int
	Bulk           bool
	Yes            bool
	Flags          *ff.FlagSet
	Command        *ff.Command
}

// New creates and registers the distill command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("distill").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Title, 0, "title", "", "book title")
	cfg.Flags.StringVar(&cfg.Author, 0, "author", "", "book author")
	cfg.Flags.StringVar(&cfg.Year, 0, "year", "", "publication year")
	cfg.Flags.StringVar(&cfg.Out, 0, "out", defaultOut, "output root directory")
	cfg.Flags.StringVar(&cfg.Slug, 0, "slug", "", "output slug (defaults from --title)")
	cfg.Flags.StringVar(&cfg.Driver, 0, "driver", "http", "llm driver: http or agent")
	cfg.Flags.StringVar(&cfg.LLMBaseURL, 0, "llm-base-url", openai.DefaultBaseURL,
		"OpenAI-compatible base URL (http driver; default: GoModel gateway)")
	cfg.Flags.StringVar(&cfg.APIKey, 0, "api-key", "", "bearer token (env EXEGESIS_API_KEY)")
	cfg.Flags.StringVar(&cfg.Model, 0, "model", defaultModel, "model id")
	cfg.Flags.IntVar(&cfg.MaxChunkTokens, 0, "max-chunk-tokens", 0, "chunk size (0 = none)")
	cfg.Flags.IntVar(&cfg.QuoteMaxRunes, 0, "quote-max-runes", 0, "quote cap (0 = auto)")
	cfg.Flags.BoolVar(&cfg.Bulk, 0, "bulk", "process all units instead of piloting one")
	cfg.Flags.BoolVar(&cfg.Yes, 0, "yes", "skip confirmation gates")
	cfg.Command = &ff.Command{
		Name:      "distill",
		Usage:     "exegesis distill [FLAGS] <book-file>",
		ShortHelp: "distill a book into a set of executable skills",
		LongHelp: `Distill a plain-text or markdown book into a set of atomic,
executable skills under <out>/<slug>/, following the RIA-TV++ pipeline.

--driver http (default) calls an OpenAI-compatible endpoint directly; by default
that is the local GoModel gateway. --driver agent instead prints the model
prompts as JSON for an external agent to run, then resumes from a cache when the
agent re-invokes the command — making book2skill agent-agnostic.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return &book2skill.Error{
			Code:    book2skill.EINVALID,
			Message: "distill: a book file path is required",
		}
	}
	return cfg.runDistill(ctx, args[0])
}

func (cfg *Config) runDistill(ctx context.Context, bookPath string) error {
	text, err := store.LoadText(bookPath)
	if err != nil {
		return fmt.Errorf("load book: %w", err)
	}
	slug := cfg.slug()
	if slug == "" {
		return &book2skill.Error{
			Code:    book2skill.EINVALID,
			Message: "distill: provide --title or --slug",
		}
	}
	quoteMax := cfg.QuoteMaxRunes
	if quoteMax <= 0 {
		quoteMax = book2skill.QuoteMaxRunes(text)
	}
	outRoot := filepath.Join(cfg.Out, slug)
	pcfg := pipeline.Config{
		Title: cfg.Title, Author: cfg.Author, Year: cfg.Year, Slug: slug,
		Model: cfg.Model, QuoteMaxRunes: quoteMax, MaxChunkRunes: cfg.MaxChunkTokens,
		Bulk: cfg.Bulk, AutoConfirm: cfg.Yes,
	}
	switch cfg.Driver {
	case "agent":
		return cfg.runAgent(ctx, text, slug, outRoot, &pcfg, bookPath)
	case "http", "":
		return cfg.runHTTP(ctx, text, outRoot, &pcfg)
	default:
		return &book2skill.Error{
			Code:    book2skill.EINVALID,
			Message: "distill: unknown --driver " + cfg.Driver,
		}
	}
}

func (cfg *Config) runHTTP(
	ctx context.Context, text, outRoot string, pcfg *pipeline.Config,
) error {
	writer := store.NewWriter(outRoot)
	p := &pipeline.Pipeline{
		LLM:       openai.New(cfg.LLMBaseURL, cfg.APIKey, nil),
		WriteFile: writer.WriteFile,
		Confirm:   newConfirm(cfg),
		Check:     newCheck(outRoot, cfg.Stderr),
		Cfg:       *pcfg,
	}
	res, err := p.Run(ctx, text)
	if err != nil {
		return fmt.Errorf("run pipeline: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout,
		"distilled %d skills into %s (%d rejected)\n",
		res.SkillCount, outRoot, res.RejectedCount)
	return nil
}

func (cfg *Config) runAgent(
	ctx context.Context, text, slug, outRoot string, pcfg *pipeline.Config, bookPath string,
) error {
	cacheDir := filepath.Join(outRoot, cacheSubdir)
	if err := os.MkdirAll(cacheDir, dirPerm); err != nil {
		return &book2skill.Error{Op: "distill.runAgent", Err: err}
	}
	llm := agent.New(cacheDir)
	writer := store.NewWriter(outRoot)
	pcfg.AutoConfirm = true // the agent loop, not an interactive prompt, drives the run
	p := &pipeline.Pipeline{
		LLM:       llm,
		WriteFile: writer.WriteFile,
		Check:     newCheck(outRoot, cfg.Stderr),
		Cfg:       *pcfg,
	}
	res, err := p.Run(ctx, text)
	switch {
	case book2skill.IsDeferred(err):
		return cfg.emitPrompts(llm.Pending(), slug, bookPath)
	case err != nil:
		return fmt.Errorf("run pipeline: %w", err)
	default:
		return cfg.emitComplete(res, outRoot)
	}
}

// emitPrompts prints the pending prompts and the resume command as JSON.
func (cfg *Config) emitPrompts(prompts []agent.Prompt, slug, bookPath string) error {
	type action struct {
		Status  string         `json:"status"`
		Book    string         `json:"book"`
		Pending int            `json:"pending_count"`
		Prompts []agent.Prompt `json:"prompts"`
		Resume  []string       `json:"resume"`
		Howto   string         `json:"instructions"`
	}
	return cfg.emitJSON(action{
		Status:  "needs_prompts",
		Book:    slug,
		Pending: len(prompts),
		Prompts: prompts,
		Resume:  cfg.resumeArgs(bookPath),
		Howto: "For each prompt, send messages to your model requiring a JSON reply " +
			"matching schema; write the reply to response_path; then re-run resume.",
	})
}

func (cfg *Config) emitComplete(res *pipeline.Result, outRoot string) error {
	type action struct {
		Status   string `json:"status"`
		Book     string `json:"book"`
		Out      string `json:"out"`
		Skills   int    `json:"skill_count"`
		Rejected int    `json:"rejected_count"`
	}
	return cfg.emitJSON(action{
		Status:   "complete",
		Book:     res.Slug,
		Out:      outRoot,
		Skills:   res.SkillCount,
		Rejected: res.RejectedCount,
	})
}

func (cfg *Config) emitJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode action: %w", err)
	}
	_, _ = fmt.Fprintln(cfg.Stdout, string(b))
	return nil
}

func (cfg *Config) resumeArgs(bookPath string) []string {
	return []string{
		"distill", "--driver", "agent",
		"--title", cfg.Title, "--author", cfg.Author, "--year", cfg.Year,
		"--out", cfg.Out, "--model", cfg.Model, "--slug", cfg.slug(),
		bookPath,
	}
}

func (cfg *Config) slug() string {
	if cfg.Slug != "" {
		return cfg.Slug
	}
	return slugify(cfg.Title)
}

// newConfirm returns a confirmation reader over the command's I/O.
func newConfirm(cfg *Config) func(context.Context, string) (bool, error) {
	return func(_ context.Context, question string) (bool, error) {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s [y/N]: ", question)
		line, err := bufio.NewReader(cfg.Stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return false, fmt.Errorf("read confirmation: %w", err)
		}
		answer := strings.ToLower(strings.TrimSpace(line))
		return answer == "y" || answer == "yes", nil
	}
}

// newCheck returns a validation runner that lints a rendered skill dir with the
// native skilllint engine (spec + quality) and fails when it finds errors. On
// failure it writes the offending diagnostics to stderr so the operator (or the
// driving agent) sees what to fix, not just a count. It never reports "skipped"
// (that path existed only for the old uvx wrapper).
func newCheck(outRoot string, stderr io.Writer) func(context.Context, string) (bool, error) {
	return func(_ context.Context, relDir string) (bool, error) {
		res, err := skilllint.Run(filepath.Join(outRoot, relDir), skilllint.Options{
			Categories: map[skilllint.Category]bool{
				skilllint.CategorySpec:    true,
				skilllint.CategoryQuality: true,
			},
		})
		if err != nil {
			return false, fmt.Errorf("skilllint: %w", err)
		}
		if n := res.Counts().Errors; n > 0 {
			skilllint.WriteText(stderr, res)
			return false, &book2skill.Error{
				Code:    book2skill.EINVALID,
				Message: "skill failed lint with " + strconv.Itoa(n) + " error(s)",
			}
		}
		return false, nil
	}
}

// slugify converts a title to a kebab-case slug.
func slugify(s string) string {
	out := make([]rune, 0, len(s))
	lastDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			out = append(out, r)
			lastDash = false
		case !lastDash && len(out) > 0:
			out = append(out, '-')
			lastDash = true
		default:
		}
	}
	return strings.Trim(string(out), "-")
}
