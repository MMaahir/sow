// Package cli is the sow command line. Run is the only entry point, and the
// tests drive every command through it.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// DevVersion is the version of a binary that wasn't built from a release tag.
const DevVersion = "0.0.0-dev"

// Env is everything a command takes from outside the process. Commands never
// read os.Stdin, os.Getenv or the working directory directly, so a test can
// run them against a temporary directory with a fixed environment.
type Env struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// Dir is the working directory.
	Dir string
	// Environ is the environment in "KEY=value" form. Commands that sow starts
	// inherit it.
	Environ []string
	// Version is the sow release, without a leading "v".
	Version string
}

// Getenv returns the value of key in e.Environ, or "" if it isn't set.
func (e Env) Getenv(key string) string {
	prefix := key + "="
	for i := len(e.Environ) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(e.Environ[i], prefix); ok {
			return value
		}
	}
	return ""
}

// Run executes sow with args and returns the process exit code: 0 on success
// and 1 on any failure. It prints the failure to env.Stderr.
func Run(ctx context.Context, args []string, env Env) int {
	root := &cobra.Command{
		Use:           "sow",
		Short:         "Set your projects up faster with sow.",
		Version:       env.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetArgs(args)
	root.SetIn(env.Stdin)
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)
	root.AddCommand(newCommand(env))

	if err := root.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(env.Stderr, "sow: %v\n", err)
		return 1
	}
	return 0
}
