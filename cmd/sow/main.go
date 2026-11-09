// Command sow generates and updates projects from the sow template.
package main

import (
	"context"
	"os"
	"os/signal"
	"regexp"
	"runtime/debug"
	"strings"

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
	return cli.Run(ctx, os.Args[1:], cli.Env{
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Dir:     dir,
		Environ: os.Environ(),
		Version: version(),
	})
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
