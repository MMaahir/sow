// Command sow generates and updates projects from the sow template.
package main

import (
	"context"
	"os"
	"os/signal"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/charmbracelet/x/term"

	"github.com/MMaahir/sow/internal/cli"
)

var releaseVersion = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dir, err := os.Getwd()
	if err != nil {
		_, _ = os.Stderr.WriteString("sow: " + err.Error() + "\n")
		return 1
	}
	env := cli.Env{
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Dir:     dir,
		Environ: os.Environ(),
		Version: version(),
	}
	// Prompts go to stderr, so a terminal on stdin is enough to ask questions
	// even when stdout is redirected.
	if term.IsTerminal(os.Stdin.Fd()) {
		env.Prompter = cli.TerminalPrompter{In: os.Stdin, Out: os.Stderr}
	}
	return cli.Run(ctx, os.Args[1:], env)
}

// version returns the release this binary was built from. A binary installed
// with "go install ...@v0.1.0" reports 0.1.0. Any other build is a development
// build, because only a release tag names a template that sow update can fetch.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || !releaseVersion.MatchString(info.Main.Version) {
		return cli.DevVersion
	}
	return strings.TrimPrefix(info.Main.Version, "v")
}
