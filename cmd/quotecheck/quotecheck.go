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
	MinSupport int
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
	cfg.Flags.IntVar(&cfg.MinSupport, 0, "min-support", 0,
		"fail a skill unless at least N of its "+segment+" passages are located (0 disables)")
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
used, it does not attempt and cannot tell you.

--min-support N additionally fails a skill whose located passage count is below N,
including one that quotes nothing at all. It gates the countable half of book2skill's
V1 -- that the claimed evidence is really in the book -- and leaves the rest of V1
alone: it cannot tell whether a passage supports the unit, nor whether two located
passages are independent of each other rather than two sentences of one paragraph.
Passing --min-support 2 does not mean V1 passed.`,
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
	if cfg.MinSupport < 0 {
		return root.Usagef("quotecheck: --min-support cannot be negative")
	}
	sources, err := cfg.loadSources()
	if err != nil {
		return err
	}
	failed := 0
	for _, dir := range args {
		s, err := skill.Load(dir)
		if err != nil {
			return fmt.Errorf("quotecheck: %w", err)
		}
		if !cfg.report(filepath.Base(dir), checker.Check(s.Body, segment, sources)) {
			failed++
		}
	}
	if failed > 0 {
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

// report prints one skill's verdict and reports whether it passed both gates: no
// passage missing from every source, and at least --min-support passages located.
//
// Only misses are listed. A skill's R segment runs to hundreds of words, so echoing
// every located passage would bury the handful that matter under a wall of confirmation.
//
// The located count comes from checker.Support rather than being tallied here, so the
// number the gate compares and the number the tally prints cannot drift apart. Without
// --min-support the output is what it always was.
func (cfg *Config) report(name string, findings []checker.Finding) bool {
	located := checker.Support(findings)
	for _, f := range findings {
		if f.Missing() {
			_, _ = fmt.Fprintf(cfg.Stdout, "%s: MISS %s\n", name, excerpt(f.Passage))
		}
	}
	if len(findings) == 0 {
		_, _ = fmt.Fprintf(cfg.Stdout,
			"%s: no quotations of at least %d words in the %s segment\n",
			name, checker.MinPassageWords, segment)
	} else {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s: %d/%d passages located\n",
			name, located, len(findings))
	}
	if located < cfg.MinSupport {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s: SUPPORT %d located, --min-support %d\n",
			name, located, cfg.MinSupport)
		return false
	}
	missing := len(findings) - located
	return missing == 0
}

// excerpt shortens a passage to one identifiable line.
func excerpt(q string) string {
	q = strings.Join(strings.Fields(q), " ")
	if r := []rune(q); len(r) > excerptRunes {
		return string(r[:excerptRunes]) + "..."
	}
	return q
}
