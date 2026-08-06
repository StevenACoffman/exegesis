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

// UsageError marks an error as a misuse of the command line — a wrong argument
// count, a missing required flag, an invalid flag value — for which printing the
// command's usage is the helpful response. The dispatcher prints usage for these
// and only these: after a runtime failure (an unreadable file, a failed write, a
// data file whose contents are wrong) the invocation itself was correct, so a flag
// list is noise in front of the line that matters.
//
// Wrapping preserves the mark, so a command that already wraps an inner error needs
// no change: errors.As finds a UsageError through fmt.Errorf("cmd: %w", err). Mark
// the error where the knowledge is — the function that knows a flag value is invalid
// — rather than at the call site that wraps it.
type UsageError struct{ Err error }

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

// Error reports the wrapped message. The zero UsageError is constructible by any
// caller, so it describes itself rather than panicking on a nil Err — a panic while
// reporting a failure would bury the failure it was reporting.
func (e UsageError) Error() string {
	if e.Err == nil {
		return "usage error"
	}
	return e.Err.Error()
}

// Unwrap exposes the underlying error so errors.Is/As reach past the marker.
func (e UsageError) Unwrap() error { return e.Err }

// Usagef returns a UsageError whose message is formatted as fmt.Errorf does, so a
// %w verb still wraps an underlying error.
//
// It returns the concrete UsageError rather than error so that callers returning it
// are not reported by wrapcheck: there is no external error here to wrap, since this
// is the constructor of the error itself.
func Usagef(format string, args ...any) UsageError {
	return UsageError{Err: fmt.Errorf(format, args...)}
}

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
			"  link     record a related-skill edge in a skill's " +
			"`## Related skills` section\n" +
			"  index    regenerate INDEX.md from every skill's " +
			"`## Related skills` section\n" +
			"  distill  run the book2skill pipeline as a resumable " +
			"agent-driven loop (Stage 0 so far)\n" +
			"  version  print version information",
	}
	return &cfg
}
