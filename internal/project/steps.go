package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// step is one thing sow does in a project, such as installing the
// dependencies or creating the GitHub repo. runSteps runs a list of them.
type step struct {
	// name says what the step does. The error names the step when it fails.
	name string
	// run does the step's work. t starts commands in the project directory.
	run func(ctx context.Context, t *tools) error
}

// command returns a step's run that starts the command name with args in the
// project directory.
func command(name string, args ...string) func(context.Context, *tools) error {
	return func(ctx context.Context, t *tools) error {
		return t.run(ctx, name, args...)
	}
}

// runSteps runs steps in order with t and stops at the first error. The error
// names the failed step, carries the error of its command, which names the
// command and the directory, and lists the steps that didn't finish, starting
// with the failed one.
func runSteps(ctx context.Context, t *tools, steps []step) error {
	for i, s := range steps {
		if err := s.run(ctx, t); err != nil {
			var left []string
			for _, l := range steps[i:] {
				left = append(left, l.name)
			}
			return fmt.Errorf("%s: %w\nThese steps didn't finish: %s", s.name, err, strings.Join(left, ", "))
		}
	}
	return nil
}

// followUpSteps returns the follow-up steps for the project in dir, in the
// order they run. A step is in the list only if its file exists in dir, and a
// step marked newOnly only if isNew is true. Every file is checked before any
// step runs, so a step's file is one the template writes, not one an earlier
// step creates.
func followUpSteps(dir string, isNew bool) ([]step, error) {
	all := []struct {
		step
		// file is a path relative to the project root.
		file string
		// newOnly marks a step that sow new runs and sow update skips, such
		// as one that writes a gitignored file. git reset --hard && git
		// clean -fd can't undo a change to a gitignored file, so an update
		// never makes one (ADR 0012).
		newOnly bool
	}{
		{
			step:    step{name: "copy .env.example to .env", run: copyEnvExample},
			file:    ".env.example",
			newOnly: true,
		},
		{
			// Vite+ installs the pnpm version that packageManager names and
			// writes pnpm-lock.yaml, which holds the exact versions of the
			// project's dependencies (ADR 0007).
			step: step{name: "install the dependencies", run: command("vp", "install")},
			file: "pnpm-workspace.yaml",
		},
	}
	var steps []step
	for _, s := range all {
		if s.newOnly && !isNew {
			continue
		}
		_, err := os.Stat(filepath.Join(dir, s.file))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		steps = append(steps, s.step)
	}
	return steps, nil
}

// copyEnvExample copies .env.example to .env so a new project starts with the
// example settings. It leaves an existing .env alone.
func copyEnvExample(_ context.Context, t *tools) error {
	destination := filepath.Join(t.dir, ".env")
	_, err := os.Lstat(destination)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	content, err := os.ReadFile(filepath.Join(t.dir, ".env.example"))
	if err != nil {
		return err
	}
	return os.WriteFile(destination, content, 0o644)
}
