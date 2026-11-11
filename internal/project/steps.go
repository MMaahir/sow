package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// followUpStep is one thing sow does in a project after it writes the files,
// such as copying .env.example to .env. A step runs only if its file exists.
type followUpStep struct {
	// name says what the step does. It prefixes the error when the step fails.
	name string
	// file is a path relative to the project root. The step runs only if this
	// file exists.
	file string
	// run does the step's work in the project directory.
	run func(ctx context.Context, dir string, environ []string) error
	// newOnly marks a step that sow new runs and sow update skips, such as one
	// that writes a gitignored file. git reset --hard && git clean -fd can't
	// undo a change to a gitignored file, so an update never makes one
	// (ADR 0012).
	newOnly bool
}

// followUpSteps returns the follow-up steps in the order they run.
func followUpSteps() []followUpStep {
	return []followUpStep{
		{
			name:    "copy .env.example to .env",
			file:    ".env.example",
			run:     copyEnvExample,
			newOnly: true,
		},
		{
			name: "install the dependencies",
			file: "pnpm-workspace.yaml",
			run:  installDependencies,
		},
	}
}

// runFollowUpSteps runs each follow-up step whose file exists in dir, in
// order, and stops at the first error. For an update, it skips the steps
// marked newOnly. A failure leaves the written files in place, so the author
// can fix the cause and run the command again.
func runFollowUpSteps(ctx context.Context, environ []string, dir string, update bool) error {
	for _, step := range followUpSteps() {
		if update && step.newOnly {
			continue
		}
		_, err := os.Stat(filepath.Join(dir, step.file))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if err := step.run(ctx, dir, environ); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}
	return nil
}

// copyEnvExample copies .env.example to .env so a new project starts with the
// example settings. It leaves an existing .env alone.
func copyEnvExample(_ context.Context, dir string, _ []string) error {
	destination := filepath.Join(dir, ".env")
	_, err := os.Lstat(destination)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	content, err := os.ReadFile(filepath.Join(dir, ".env.example"))
	if err != nil {
		return err
	}
	return os.WriteFile(destination, content, 0o644)
}

// installDependencies runs "vp install" at the project root. Vite+ installs
// the pnpm version that packageManager names and writes pnpm-lock.yaml, which
// holds the exact versions of the project's dependencies (ADR 0007).
func installDependencies(ctx context.Context, dir string, environ []string) error {
	return run(ctx, environ, dir, "vp", "install")
}
