// Package root defines the root configuration for the CLI.
package root

import (
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"
)

// ExitError is returned by commands that want a specific non-zero exit code
// without printing an additional error message. run() in main.go checks for
// ExitError with errors.As and calls os.Exit(int(e)) directly, bypassing the
// default "error: ..." printer.
type ExitError int

// Config holds shared I/O writers and the root ff.Command.
// All subcommand configs embed *Config to inherit these.
type Config struct {
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Flags   *ff.FlagSet
	Command *ff.Command
}

func (e ExitError) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

// New returns a new root Config with the given I/O writers.
func New(stdin io.Reader, stdout, stderr io.Writer) *Config {
	var cfg Config
	cfg.Stdin = stdin
	cfg.Stdout = stdout
	cfg.Stderr = stderr
	// No shared flags — cfg.Flags is nil; ff provides --help automatically.
	// Subcommands call SetParent(parent.Flags)
	// which is a no-op here; add shared flags (e.g. BoolVar) to activate.
	// To add shared flags, uncomment and bind before constructing the command:
	// cfg.Flags = ff.NewFlagSet("exegesis")
	// cfg.Flags.BoolVar(&cfg.MyFlag, 0, "my-flag", "", "description")
	cfg.Command = &ff.Command{
		Name:  "exegesis",
		Usage: "exegesis <SUBCOMMAND> ...",
		ShortHelp: "distill a book into a tree of Agent Skills and gate " +
			"their structure",
		LongHelp: "exegesis is the deterministic pipeline/gate CLI behind the " +
			"book2skill skill.\n\n" +
			"Implemented gates:\n" +
			"  lint     validate a skill's frontmatter, body links, and " +
			"runtime-neutrality\n" +
			"  tests    check a skill's test-prompts.json composition; " +
			"--scaffold writes a starter\n" +
			"  verify   run every gate over a skill tree and emit " +
			"skills-manifest.json\n" +
			"  version  print version information\n\n" +
			"Planned (not yet built): distill, index, link.",
	}
	return &cfg
}
