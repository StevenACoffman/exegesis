// Package mergestatus implements the "merge-status" command family: it appends
// validated entries to, and validates, the append-only `## Merge Status` ledger
// that a merge-skills run maintains in each affected source skill. The state
// decision is the agent's judgment; writing and checking the ledger is
// mechanical.
package mergestatus

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/mergedoc"
)

const (
	skillFile = "SKILL.md"
	filePerm  = 0o644
)

// Config is the parent "merge-status" command; it owns the append and check
// subcommands.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// appendConfig is the "merge-status append" subcommand.
type appendConfig struct {
	*Config
	Run      string
	State    string
	Pair     string
	Into     string
	Reason   string
	Excluded string
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// checkConfig is the "merge-status check" subcommand.
type checkConfig struct {
	*Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the merge-status command family.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("merge-status").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "merge-status",
		Usage:     "exegesis merge-status <append|check> [FLAGS] <path>",
		ShortHelp: "append to or validate a skill's `## Merge Status` ledger",
		Flags:     cfg.Flags,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	newAppend(&cfg)
	newCheck(&cfg)
	return &cfg
}

func newAppend(parent *Config) {
	var cfg appendConfig
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("append").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Run, 0, "run", "", "merge run slug (required)")
	cfg.Flags.StringVar(&cfg.State, 0, "state", "", "state: merged|partial|rejected|…  (required)")
	cfg.Flags.StringVar(&cfg.Pair, 0, "pair", "", "pair id (for surface/complementary/rejected)")
	cfg.Flags.StringVar(&cfg.Into, 0, "into", "", "merged skill slug (for merged/partial)")
	cfg.Flags.StringVar(&cfg.Reason, 0, "reason", "", "rejection reason code (for rejected)")
	cfg.Flags.StringVar(&cfg.Excluded, 0, "excluded", "", "excluded content (for partial)")
	cfg.Command = &ff.Command{
		Name:      "append",
		Usage:     "exegesis merge-status append [FLAGS] <skill-dir>",
		ShortHelp: "append one validated entry to a source skill's ledger",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
}

func (cfg *appendConfig) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return einval("merge-status append: a skill directory is required")
	}
	entry := book2skill.MergeStatusEntry{
		Run: cfg.Run, State: book2skill.MergeState(cfg.State),
		Pair: cfg.Pair, Into: cfg.Into,
		Reason: book2skill.MergeReason(cfg.Reason), Excluded: cfg.Excluded,
	}
	if problems := entry.Validate(); len(problems) > 0 {
		return einval("merge-status append: invalid entry: " + strings.Join(problems, "; "))
	}
	path := filepath.Join(args[0], skillFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return einval("merge-status append: cannot read " + path)
	}
	out, err := mergedoc.Append(string(data), &entry)
	if err != nil {
		return fmt.Errorf("merge-status append: %w", err)
	}
	if err := os.WriteFile(path, []byte(out), filePerm); err != nil {
		return &book2skill.Error{Op: "mergestatus.append", Err: err}
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "appended %s entry (run %s) to %s\n", cfg.State, cfg.Run, path)
	return nil
}

func newCheck(parent *Config) {
	var cfg checkConfig
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("check").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "check",
		Usage:     "exegesis merge-status check <dir>",
		ShortHelp: "validate every `## Merge Status` ledger under a directory",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
}

func (cfg *checkConfig) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return einval("merge-status check: a directory is required")
	}
	problems := checkTree(args[0])
	for _, p := range problems {
		_, _ = fmt.Fprintln(cfg.Stdout, p)
	}
	if len(problems) > 0 {
		return root.ExitError(1)
	}
	_, _ = fmt.Fprintln(cfg.Stdout, "merge-status: all ledgers valid")
	return nil
}

// checkTree walks dir for SKILL.md files and returns every merge-status problem,
// each prefixed with the file's path relative to dir.
func checkTree(dir string) []string {
	var problems []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != skillFile {
			return nil //nolint:nilerr // unreadable entries are skipped, not fatal
		}
		rel, _ := filepath.Rel(dir, path)
		problems = append(problems, checkFile(path, rel)...)
		return nil
	})
	return problems
}

func checkFile(path, rel string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{rel + ": cannot read"}
	}
	entries, err := mergedoc.Parse(string(data))
	if err != nil {
		return []string{rel + ": " + err.Error()}
	}
	var problems []string
	for i := range entries {
		for _, p := range entries[i].Validate() {
			problems = append(problems, rel+": "+p)
		}
	}
	return problems
}

func einval(msg string) error {
	return &book2skill.Error{Code: book2skill.EINVALID, Message: msg}
}
