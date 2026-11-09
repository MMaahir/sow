// Package project creates and changes generated projects on disk.
package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MMaahir/sow/internal/answers"
	"github.com/MMaahir/sow/internal/render"
)

// Options describes the project that New creates.
type Options struct {
	// Dir is where the project goes. It must not exist yet.
	Dir string
	// Answers must come from answers.Resolve.
	Answers answers.Answers
	// Version is the sow release recorded in the answers file.
	Version string
	// Template holds the template directories.
	Template fs.FS
	// Environ is the environment for the commands New starts.
	Environ []string
}

// New renders the template into opts.Dir and makes the first commit. It
// renders everything before it writes anything, so a template error leaves no
// directory behind.
func New(ctx context.Context, opts Options) error {
	if _, err := os.Lstat(opts.Dir); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s already exists", filepath.Base(opts.Dir))
	}

	files, err := render.Render(opts.Template, opts.Answers.Dirs(), opts.Answers)
	if err != nil {
		return err
	}
	answersFile, err := answers.Marshal(opts.Version, opts.Answers)
	if err != nil {
		return err
	}
	files = append(files, render.File{Path: answers.FileName, Content: answersFile})

	if err := render.Write(opts.Dir, files); err != nil {
		return fmt.Errorf("write the project: %w", err)
	}

	for _, args := range [][]string{
		{"init", "--initial-branch", "main"},
		{"add", "--all"},
		{"commit", "--message", "chore: initial commit from sow"},
	} {
		if err := run(ctx, opts.Environ, opts.Dir, "git", args...); err != nil {
			return err
		}
	}
	return nil
}

// run starts a command in dir and waits for it. On failure the error names
// the command and its directory and carries the command's output, so the
// author can run it again by hand.
func run(ctx context.Context, environ []string, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = environ
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed in %s: %w\n%s",
			name, strings.Join(args, " "), dir, err, strings.TrimSpace(string(output)))
	}
	return nil
}
