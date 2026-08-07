// Package quotecheck implements the "quotecheck" command: the fabrication guard. It
// reports each quotation in a skill's R segment that appears in none of the supplied
// source texts. The matching is pure and shared (internal/quotecheck); this command
// reads the files and renders the report.
package quotecheck

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	checker "github.com/StevenACoffman/exegesis/internal/quotecheck"
	"github.com/StevenACoffman/skillet/skill"
)

// segment is the RIA-TV++ segment a skill quotes its sources in.
const segment = "R"

// excerptRunes is how much of a quotation to echo. Enough to identify which one is
// meant; a red line already caps quotations at 150 words, so printing one in full
// would bury the report it appears in.
const excerptRunes = 60

// Config holds the quotecheck command configuration.
type Config struct {
	*root.Config
	SourceText string
	Flags      *ff.FlagSet
	Command    *ff.Command
}

// New creates and registers the quotecheck command.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("quotecheck").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.SourceText, 0, "source-text", "",
		"comma-separated plain-text source files the quotations should be found in")
	cfg.Command = &ff.Command{
		Name:      "quotecheck",
		Usage:     "exegesis quotecheck --source-text a.txt,b.txt SKILL_DIR ...",
		ShortHelp: "report R-segment quotations that appear in none of the source texts",
		LongHelp: `Read each SKILL_DIR/SKILL.md, take every quotation in its "` + segment + `" segment, and
report whether that quotation appears in any of the --source-text files. A quotation
found in none of them is reported as MISS and the command exits non-zero.

Sources must be plain text: extract an EPUB or PDF first. Passing the book in its
original container silently yields no matches, since the quotation text is not present
in the bytes.

Quotations are the blockquote runs the quotation-length red line counts, so this guard
and that rule agree on what a quotation is. Only the "` + segment + `" segment is examined: an
illustrative blockquote elsewhere in a skill is not a claim about a book.

Both sides are normalized before matching -- runs of whitespace collapsed, and curly
quotes, en and em dashes, ellipses and exotic spaces folded to ASCII. A quotation is
line-wrapped in Markdown and its source is not, so a literal comparison would report
every quotation as missing. That normalization is the whole of the latitude given:
anything it does not forgive is reported.

This is a mechanical containment check, not a judgment. It answers "do these words
appear in the book at all". Whether a paraphrase is faithful, or a quotation fairly
used, it does not attempt and cannot tell you.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return root.Usagef("quotecheck: need at least one skill directory")
	}
	sources, err := cfg.loadSources()
	if err != nil {
		return err
	}
	missing := 0
	for _, dir := range args {
		s, err := skill.Load(dir)
		if err != nil {
			return fmt.Errorf("quotecheck: %w", err)
		}
		missing += cfg.report(filepath.Base(dir), checker.Check(s.Body, segment, sources))
	}
	if missing > 0 {
		return root.ExitError(1)
	}
	return nil
}

// loadSources reads every --source-text file. A missing source is fatal rather than
// skipped: dropping one silently would turn its quotations into fabrications.
func (cfg *Config) loadSources() ([]checker.Source, error) {
	var sources []checker.Source
	for _, path := range strings.Split(cfg.SourceText, ",") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue // tolerate stray/trailing commas
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("quotecheck: read source %s: %w", path, err)
		}
		sources = append(sources, checker.Source{Name: path, Text: string(b)})
	}
	if len(sources) == 0 {
		return nil, root.Usagef("quotecheck: --source-text names no readable file")
	}
	return sources, nil
}

// report prints the passages found in no source and a one-line tally, returning how
// many were missing.
//
// Only misses are listed. A skill's R segment runs to hundreds of words, so echoing
// every located passage would bury the handful that matter under a wall of confirmation.
func (cfg *Config) report(name string, findings []checker.Finding) int {
	if len(findings) == 0 {
		_, _ = fmt.Fprintf(cfg.Stdout,
			"%s: no quotations of at least %d words in the %s segment\n",
			name, checker.MinPassageWords, segment)
		return 0
	}
	missing := 0
	for _, f := range findings {
		if f.Missing() {
			missing++
			_, _ = fmt.Fprintf(cfg.Stdout, "%s: MISS %s\n", name, excerpt(f.Passage))
		}
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%s: %d/%d passages located\n",
		name, len(findings)-missing, len(findings))
	return missing
}

// excerpt shortens a passage to one identifiable line.
func excerpt(q string) string {
	q = strings.Join(strings.Fields(q), " ")
	if r := []rune(q); len(r) > excerptRunes {
		return string(r[:excerptRunes]) + "..."
	}
	return q
}
