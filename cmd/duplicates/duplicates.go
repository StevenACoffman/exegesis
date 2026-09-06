// Package duplicates implements `exegesis duplicates`: it reports a skill that exists
// in more than one tree and says whether the copies still agree. The comparison is pure
// (internal/duplicates); this command reads the trees and renders the report.
package duplicates

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/duplicates"
)

// longHelp is the command's help text, kept out of New so the constructor reads as the
// flag-binding sequence it is.
const longHelp = `Report every skill that exists in more than one of the named TREEs, and whether the
copies have diverged.

One corpus kept in several places is the normal state of an installed skill tree: a
market repository is the source and each runtime reads its own directory. The copies are
made by copying, so nothing keeps them in step -- and each tree passes its own gates
while answering a different question from the tree beside it.

Duplication alone is therefore reported, not failed: a deliberate install target is not a
defect. Divergence is the finding, and only divergence exits non-zero -- two copies of
one skill whose content differs mean at least one is stale and a reader cannot tell
which.

Content is compared by the same hash exegesis writes into skills-manifest.json, so a
verdict here and a manifest entry there mean the same thing.

At least two trees are required: one tree cannot duplicate across trees.`

// Config holds the duplicates command configuration.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the duplicates command.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("duplicates").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "duplicates",
		Usage:     "exegesis duplicates TREE TREE [TREE...]",
		ShortHelp: "report skills that exist in more than one tree, and whether they diverged",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) < 2 {
		// Refused rather than tolerated: a one-tree run can only ever report nothing,
		// and a check that cannot fire reads as a clean bill of health.
		return root.Usagef("duplicates: need at least two trees to compare")
	}
	copies, empty, err := duplicates.Scan(args)
	if err != nil {
		return fmt.Errorf("duplicates: %w", err)
	}
	for _, tree := range empty {
		_, _ = fmt.Fprintf(cfg.Stdout,
			"%s: no skills found, so nothing from it was compared\n", tree)
	}
	return cfg.report(duplicates.Find(copies), len(args))
}

// report renders the findings and signals divergence with a non-zero exit.
func (cfg *Config) report(findings []duplicates.Finding, trees int) error {
	diverged := 0
	for _, f := range findings {
		if f.Diverged() {
			diverged++
		}
		cfg.renderFinding(f)
	}
	if len(findings) == 0 {
		_, _ = fmt.Fprintf(cfg.Stdout, "no skill appears in more than one of the %d trees\n", trees)
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%d skill(s) in more than one tree, %d diverged\n",
		len(findings), diverged)
	if diverged > 0 {
		return root.ExitError(1)
	}
	return nil
}

// renderFinding writes one skill's copies, marking the divergent case.
func (cfg *Config) renderFinding(f duplicates.Finding) {
	verdict := "same in every tree"
	if f.Diverged() {
		verdict = "DIVERGED"
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%s: in %d trees, %s\n", f.Slug, len(f.Copies), verdict)
	for _, c := range f.Copies {
		_, _ = fmt.Fprintf(cfg.Stdout, "    %s  %s\n", c.Hash, c.Tree)
	}
}
