package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MMaahir/sow/internal/cli"
)

// testVersion is the sow version every sandbox runs as.
const testVersion = "1.2.3"

// sandbox is a temporary machine for one test: a working directory, a home
// directory holding the personal defaults file, and an environment that
// ignores the real machine's git config.
type sandbox struct {
	t    *testing.T
	dir  string
	home string
	// extraEnv is added to the environment of every run, as "KEY=value".
	extraEnv []string
}

// result is what one sow run produced.
type result struct {
	code   int
	stdout string
	stderr string
}

// newSandbox returns a sandbox whose personal defaults file answers every
// personal question.
func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	s := &sandbox{t: t, dir: t.TempDir(), home: t.TempDir()}
	s.writePersonalDefaults("user_name: Test User\nuser_email: test@example.com\ngithub_user: test-user\n")
	return s
}

// run executes sow in the sandbox's working directory.
func (s *sandbox) run(args ...string) result {
	s.t.Helper()
	var stdout, stderr bytes.Buffer
	code := cli.Run(s.t.Context(), args, cli.Env{
		Stdin:   strings.NewReader(""),
		Stdout:  &stdout,
		Stderr:  &stderr,
		Dir:     s.dir,
		Environ: s.environ(),
		Version: testVersion,
	})
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// mustRun executes sow and fails the test if it exits non-zero.
func (s *sandbox) mustRun(args ...string) result {
	s.t.Helper()
	got := s.run(args...)
	if got.code != 0 {
		s.t.Fatalf("sow %s exited %d:\n%s", strings.Join(args, " "), got.code, got.stderr)
	}
	return got
}

func (s *sandbox) environ() []string {
	return append([]string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + s.home,
		"GIT_CONFIG_GLOBAL=" + filepath.Join(s.home, "gitconfig"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	}, s.extraEnv...)
}

// writePersonalDefaults replaces the personal defaults file.
func (s *sandbox) writePersonalDefaults(content string) {
	s.t.Helper()
	s.write(s.home, ".config/sow/config.yml", content)
}

// removePersonalDefaults deletes the personal defaults file.
func (s *sandbox) removePersonalDefaults() {
	s.t.Helper()
	if err := os.Remove(filepath.Join(s.home, ".config", "sow", "config.yml")); err != nil {
		s.t.Fatal(err)
	}
}

// write creates the file at root/name, and its directories, with content.
func (s *sandbox) write(root, name, content string) {
	s.t.Helper()
	target := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		s.t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		s.t.Fatal(err)
	}
}

// read returns the contents of a file in the working directory, such as
// "demo/README.md".
func (s *sandbox) read(name string) string {
	s.t.Helper()
	content, err := os.ReadFile(filepath.Join(s.dir, filepath.FromSlash(name)))
	if err != nil {
		s.t.Fatal(err)
	}
	return string(content)
}

// exists reports whether a path in the working directory exists.
func (s *sandbox) exists(name string) bool {
	s.t.Helper()
	_, err := os.Lstat(filepath.Join(s.dir, filepath.FromSlash(name)))
	return err == nil
}

// git runs git in a directory under the working directory and returns its
// trimmed output.
func (s *sandbox) git(dir string, args ...string) string {
	s.t.Helper()
	cmd := exec.CommandContext(s.t.Context(), "git", args...)
	cmd.Dir = filepath.Join(s.dir, dir)
	cmd.Env = s.environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

// wantContains fails the test if text doesn't contain every part.
func wantContains(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("want %q in:\n%s", part, text)
		}
	}
}
