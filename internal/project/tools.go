package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// tools starts external commands, such as git, gh and vp, in one directory
// with one environment. Every command that internal/project starts goes
// through it. It finds each executable on the PATH of its environment, never
// the real process's PATH, and looks each one up once.
type tools struct {
	environ []string
	dir     string
	// found maps a tool's name to its executable. The copies that in returns
	// share it.
	found map[string]string
}

// newTools returns tools that start commands in dir with the environment
// environ.
func newTools(environ []string, dir string) *tools {
	return &tools{environ: environ, dir: dir, found: map[string]string{}}
}

// in returns tools that start commands in dir, with the same environment.
func (t *tools) in(dir string) *tools {
	return &tools{environ: t.environ, dir: dir, found: t.found}
}

// require returns an error when the tool called name isn't on PATH. The error
// says the tool is missing, followed by advice, which tells the author how to
// install it.
func (t *tools) require(name, advice string) error {
	if _, err := t.lookPath(name); err != nil {
		return fmt.Errorf("%w: %s", err, advice)
	}
	return nil
}

// run starts the command name with args and waits for it. On failure the
// error names the command and the directory and carries the command's output,
// so the author can run it again by hand.
func (t *tools) run(ctx context.Context, name string, args ...string) error {
	_, _, err := t.start(ctx, 0, name, args...)
	return err
}

// output starts a command like run does, and returns what the command printed
// to stdout. It leaves stderr out, because a command such as git prints
// warnings there that would corrupt output a caller parses. The error carries
// both.
func (t *tools) output(ctx context.Context, name string, args ...string) (string, error) {
	stdout, _, err := t.start(ctx, 0, name, args...)
	return stdout, err
}

// exitCode starts a command like run does, and returns its exit code when the
// code is from 0 to highest. Any other exit is an error like run's. Use it for
// a command whose exit code is a result, such as git merge-file, which exits
// with the number of conflicts.
func (t *tools) exitCode(ctx context.Context, highest int, name string, args ...string) (int, error) {
	_, code, err := t.start(ctx, highest, name, args...)
	return code, err
}

// start runs the command and returns its stdout and its exit code. An exit
// code from 1 to highest isn't an error.
func (t *tools) start(ctx context.Context, highest int, name string, args ...string) (string, int, error) {
	command := strings.Join(append([]string{name}, args...), " ")
	executable, err := t.lookPath(name)
	if err != nil {
		return "", 0, fmt.Errorf("%s failed in %s: %w", command, t.dir, err)
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = t.dir
	cmd.Env = t.environ
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err == nil {
		return stdout.String(), 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() >= 1 && exit.ExitCode() <= highest {
		return stdout.String(), exit.ExitCode(), nil
	}
	return "", 0, fmt.Errorf("%s failed in %s: %w\n%s",
		command, t.dir, err, strings.TrimSpace(stdout.String()+"\n"+stderr.String()))
}

// lookPath returns the absolute path of the executable called name, found in
// the PATH of t's environment. The standard exec.LookPath reads the real
// process's PATH, which would ignore the environment sow was given.
//
// It skips PATH entries that aren't absolute. A relative entry means a
// different directory to the command than to sow, because the command runs in
// the project directory.
func (t *tools) lookPath(name string) (string, error) {
	if executable, ok := t.found[name]; ok {
		return executable, nil
	}
	var pathList string
	for _, entry := range t.environ {
		if value, ok := strings.CutPrefix(entry, "PATH="); ok {
			pathList = value
		}
	}
	for _, dir := range filepath.SplitList(pathList) {
		if !filepath.IsAbs(dir) {
			continue
		}
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			t.found[name] = candidate
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s isn't installed, or isn't on PATH", name)
}
