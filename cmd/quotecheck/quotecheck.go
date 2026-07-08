// Package quotecheck implements the "quotecheck" command: it verifies that each
// quote in a skill's R (Reading) segment actually appears in the provided source
// text — the deterministic half of merge-skills Phase 1.5 source verification,
// guarding against fabricated or drifted quotes. Paraphrase-distance judgment
// stays with the agent.
package quotecheck

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

const skillFile = "SKILL.md"

// Config holds the flags and wiring for the quotecheck command.
type Config struct {
	*root.Config
	Source  string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the quotecheck command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("quotecheck").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Source, 0, "source", "",
		"comma-separated plain-text source file(s) to search for the R-segment quotes")
	cfg.Command = &ff.Command{
		Name:      "quotecheck",
		Usage:     "exegesis quotecheck --source <a.txt[,b.txt]> <skill-dir>",
		ShortHelp: "verify a skill's R-segment quotes appear in the source text",
		LongHelp: `Check that every quote in <skill-dir>/SKILL.md's R (Reading)
segment appears verbatim (whitespace-normalized) in at least one --source text
file. Flags quotes found in no source — the fabrication guard for merge-skills
Phase 1.5. Sources must be plain text (extract EPUB/PDF first). Exit code is 1
when any quote is unlocated.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return einval("quotecheck: a skill directory is required")
	}
	sources := splitCSV(cfg.Source)
	if len(sources) == 0 {
		return einval("quotecheck: at least one --source text file is required")
	}
	body, err := os.ReadFile(filepath.Join(args[0], skillFile))
	if err != nil {
		return einval("quotecheck: cannot read " + filepath.Join(args[0], skillFile))
	}
	corpus, err := readSources(sources)
	if err != nil {
		return err
	}
	quotes := book2skill.RSegmentQuotes(string(body))
	return cfg.report(quotes, corpus)
}

func (cfg *Config) report(quotes []string, corpus string) error {
	if len(quotes) == 0 {
		_, _ = fmt.Fprintln(cfg.Stdout, "quotecheck: no R-segment quotes found to verify")
		return nil
	}
	unlocated := 0
	for _, q := range quotes {
		if book2skill.QuoteFound(q, corpus) {
			_, _ = fmt.Fprintln(cfg.Stdout, "ok:   "+ellipsis(q))
			continue
		}
		unlocated++
		_, _ = fmt.Fprintln(cfg.Stdout, "MISS: "+ellipsis(q))
	}
	if unlocated > 0 {
		return root.ExitError(1)
	}
	return nil
}

// readSources concatenates the text of every source file.
func readSources(paths []string) (string, error) {
	var b strings.Builder
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return "", einval("quotecheck: cannot read source " + p)
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func ellipsis(s string) string {
	const limit = 70
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "…"
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func einval(msg string) error {
	return &book2skill.Error{Code: book2skill.EINVALID, Message: msg}
}
