// Package cmd is the dispatcher for the exegesis CLI.
// It registers all commands and routes incoming arguments
// to the matching command implementation.
package cmd

// climax:name exegesis
// climax:root-pkg root
// climax:env-prefix EXEGESIS

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/StevenACoffman/exegesis/cmd/index"
	"github.com/StevenACoffman/exegesis/cmd/link"
	"github.com/StevenACoffman/exegesis/cmd/lint"
	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/cmd/tests"
	"github.com/StevenACoffman/exegesis/cmd/verify"
	"github.com/StevenACoffman/exegesis/cmd/version"
)

// Run parses args and dispatches to the matching command.
// args must not include the executable name (pass os.Args[1:]).
//
// Every flag can be set via a EXEGESIS_-prefixed environment variable.
// The mapping rule is: prepend EXEGESIS_, uppercase, replace dashes with
// underscores.
//
// Flags supplied on the command line always take precedence over env vars.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	r := root.New(stdin, stdout, stderr)
	version.New(r)
	lint.New(r)
	tests.New(r)
	verify.New(r)
	link.New(r)
	index.New(r)
	// register new commands here

	if err := r.Command.Parse(args, ff.WithEnvVarPrefix("EXEGESIS")); err != nil {
		_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command))
		return fmt.Errorf("parse: %w", err)
	}

	// An unmatched token leaves the selected command a group parent (Exec == nil)
	// with a leftover positional. Without this it would fall through to Run,
	// return ff.ErrNoExec, and exit 0 — indistinguishable from a bare invocation.
	// A bare invocation has no leftover arg and is left to the ErrNoExec path.
	if sel := r.Command.GetSelected(); sel.Exec == nil {
		if rest := sel.Flags.GetArgs(); len(rest) > 0 {
			_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(sel))
			return fmt.Errorf("%s: unknown subcommand %q", sel.Name, rest[0])
		}
	}

	if err := r.Command.Run(ctx); err != nil {
		// Don't print usage help for ErrNoExec (no subcommand given) or
		// ExitError (command already reported its own outcome).
		var exitErr root.ExitError
		if !errors.Is(err, ff.ErrNoExec) && !errors.As(err, &exitErr) {
			_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command.GetSelected()))
		}
		return err
	}

	return nil
}
