// Package distill implements `exegesis distill`: it runs the book2skill pipeline
// as a resumable, agent-driven loop. With --driver agent it prints the pending
// prompts as JSON and stops; the invoking agent answers them and re-runs the
// command. The pipeline logic lives in internal/distill; this command does the
// flag handling and prints the round's Outcome.
package distill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/distill"
	"github.com/StevenACoffman/skillet/skill"
)

// Config holds the distill command configuration.
type Config struct {
	*root.Config
	Driver  string
	Title   string
	Out     string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the distill command.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("distill").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Driver, 0, "driver", "agent",
		"how prompts are answered: agent (emit JSON and stop) or http (not yet implemented)")
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
	cfg.Command = &ff.Command{
		Name:      "distill",
		Usage:     "exegesis distill --driver agent --title TITLE [--out DIR] BOOK_FILE",
		ShortHelp: "run the book2skill pipeline as a resumable agent-driven loop",
		LongHelp: `Run the RIA-TV++ pipeline over BOOK_FILE, writing the skill tree under
<out>/<title-slug>. With --driver agent, distill does all deterministic work and,
whenever it needs a model, prints the pending prompts as JSON and stops: the agent
sends each prompt's messages to a model, writes the JSON reply to its
response_path, and re-runs the printed "resume" command. A content-addressed cache
under the tree is the only state, so the loop is resumable and idempotent.

Phase 1 implements Stage 0 (book -> gated BOOK_OVERVIEW.md); later stages and
--driver http are not yet built.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("distill: need exactly one book file")
	}
	if cfg.Title == "" {
		return errors.New("distill: --title is required")
	}
	if err := checkDriver(cfg.Driver); err != nil {
		return err
	}
	bookPath := args[0]
	tree := filepath.Join(cfg.Out, skill.Slug(cfg.Title))
	resume := fmt.Sprintf("exegesis distill --driver %s --title %q --out %q %q",
		cfg.Driver, cfg.Title, cfg.Out, bookPath)
	out, err := distill.Run(tree, bookPath, resume)
	if err != nil {
		return fmt.Errorf("distill: %w", err)
	}
	enc := json.NewEncoder(cfg.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("distill: encode outcome: %w", err)
	}
	return nil
}

// checkDriver validates the --driver value for this phase.
func checkDriver(driver string) error {
	switch driver {
	case "agent":
		return nil
	case "http":
		return errors.New("distill: --driver http is not yet implemented")
	default:
		return fmt.Errorf("distill: unknown --driver %q (known: agent)", driver)
	}
}
