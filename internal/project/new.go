// Package project creates and changes generated projects on disk.
package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/MMaahir/sow/internal/answers"
)

// Options describes the project that New creates.
type Options struct {
	// Dir is where the project goes. It must not exist yet.
	Dir string
	// Answers must come from answers.Settle, with no problems.
	Answers answers.Answers
	// Version is the sow release recorded in the answers file.
	Version string
	// Template holds the template directories.
	Template fs.FS
	// Environ is the environment for the commands New starts.
	Environ []string
	// GitHub puts the project on GitHub after the first commit, in the repo
	// <github_user>/<project_name>.
	GitHub bool
}

// New renders the template into opts.Dir, runs the follow-up steps and makes
// the first commit. When the answers turn on a TypeScript component, it first
// checks that Vite+ is installed and set up. With opts.GitHub, it first checks
// that gh is installed and logged in, and after the first commit it creates
// and protects the GitHub repo and pushes main. It checks and renders
// everything before it writes anything, so a failed check or a template error
// leaves no directory behind. It returns the error of CheckTarget when
// opts.Dir exists.
func New(ctx context.Context, opts Options) error {
	_, err := pipeline{
		dir:      opts.Dir,
		environ:  opts.Environ,
		to:       opts.Version,
		wanted:   opts.Answers,
		template: opts.Template,
		github:   opts.GitHub,
	}.run(ctx)
	return err
}

// CheckTarget returns an error when dir, the directory of a new project,
// already exists, because New creates a project only in a new directory. Call
// it before asking the author questions for a project New would refuse.
func CheckTarget(dir string) error {
	if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s already exists", filepath.Base(dir))
	}
	return nil
}
