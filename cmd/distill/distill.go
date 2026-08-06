// Package distill implements `exegesis distill`: it runs the book2skill pipeline
// as a resumable loop. With --driver agent it prints the pending prompts as JSON
// and stops; the invoking agent answers them and re-runs. With --driver http it
// answers the prompts itself against an OpenAI-compatible endpoint and loops to
// completion. The pipeline logic lives in internal/distill; this command does
// the flag handling, driver selection, and prints the round's Outcome.
package distill

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/distill"
	"github.com/StevenACoffman/skillet/skill"
)

// defaultEndpoint is the OpenAI-compatible chat-completions URL used by
// --driver http unless --endpoint overrides it.
const defaultEndpoint = "https://api.openai.com/v1/chat/completions"

// Config holds the distill command configuration.
type Config struct {
	*root.Config
	Driver   string
	Title    string
	Out      string
	Endpoint string
	Model    string
	APIKey   string
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// New creates and registers the distill command.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("distill").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Driver, 0, "driver", "agent",
		"how prompts are answered: agent (emit JSON and stop) or http (call an endpoint)")
	cfg.Flags.StringVar(
		&cfg.Title,
		0,
		"title",
		"",
		"book title (required; the tree slug is derived from it)",
	)
	cfg.Flags.StringVar(
		&cfg.Out,
		0,
		"out",
		"books",
		"parent directory for the generated skill tree",
	)
	cfg.Flags.StringVar(&cfg.Endpoint, 0, "endpoint", defaultEndpoint,
		"OpenAI-compatible chat-completions endpoint (--driver http)")
	cfg.Flags.StringVar(&cfg.Model, 0, "model", "gpt-4o-mini", "model name (--driver http)")
	cfg.Flags.StringVar(&cfg.APIKey, 0, "api-key", "",
		"bearer token for --driver http (or set EXEGESIS_API_KEY)")
	cfg.Command = &ff.Command{
		Name:      "distill",
		Usage:     "exegesis distill [--driver agent|http] --title TITLE [--out DIR] BOOK_FILE",
		ShortHelp: "run the book2skill pipeline as a resumable loop (agent or http driver)",
		LongHelp: `Run the RIA-TV++ pipeline over BOOK_FILE, writing the skill tree under
<out>/<title-slug>: BOOK_OVERVIEW.md, candidates/, one <slug>/ per skill
(SKILL.md + test-prompts.json), and INDEX.md. A content-addressed cache under the
tree is the only state, so the loop is resumable and idempotent.

With --driver agent, distill does the deterministic work and, whenever it needs a
model, prints the pending prompts as JSON and stops; the agent writes each JSON
reply to its response_path and re-runs the printed "resume" command. With
--driver http, distill answers the prompts itself against --endpoint (using
--model and --api-key) and loops until the tree is complete.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return root.Usagef("distill: need exactly one book file")
	}
	if cfg.Title == "" {
		return root.Usagef("distill: --title is required")
	}
	bookPath := args[0]
	tree := filepath.Join(cfg.Out, skill.Slug(cfg.Title))
	resume := fmt.Sprintf("exegesis distill --driver %s --title %q --out %q %q",
		cfg.Driver, cfg.Title, cfg.Out, bookPath)
	out, err := cfg.drive(ctx, tree, bookPath, resume)
	if err != nil {
		return err
	}
	return cfg.emit(&out)
}

// drive runs the pipeline with the selected driver and returns the round's (or,
// for http, the terminal) Outcome.
func (cfg *Config) drive(
	ctx context.Context,
	tree, bookPath, resume string,
) (distill.Outcome, error) {
	switch cfg.Driver {
	case "agent":
		out, err := distill.Run(tree, bookPath, resume)
		if err != nil {
			return distill.Outcome{}, fmt.Errorf("distill: %w", err)
		}
		return out, nil
	case "http":
		ans, err := cfg.answerer()
		if err != nil {
			return distill.Outcome{}, err
		}
		out, err := distill.RunHTTP(ctx, tree, bookPath, resume, ans)
		if err != nil {
			return distill.Outcome{}, fmt.Errorf("distill: %w", err)
		}
		return out, nil
	default:
		return distill.Outcome{}, root.Usagef(
			"distill: unknown --driver %q (known: agent, http)",
			cfg.Driver,
		)
	}
}

// answerer builds the http driver's model client from the flags.
func (cfg *Config) answerer() (distill.Answerer, error) {
	if cfg.APIKey == "" {
		return nil, root.Usagef(
			"distill: --driver http requires --api-key (or EXEGESIS_API_KEY)")
	}
	return &distill.HTTPAnswerer{
		Client:   &http.Client{Timeout: 2 * time.Minute},
		Endpoint: cfg.Endpoint,
		Model:    cfg.Model,
		APIKey:   cfg.APIKey,
	}, nil
}

// emit writes the outcome as indented JSON to stdout.
func (cfg *Config) emit(out *distill.Outcome) error {
	enc := json.NewEncoder(cfg.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("distill: encode outcome: %w", err)
	}
	return nil
}
