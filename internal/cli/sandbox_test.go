package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MMaahir/sow/internal/answers"
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
	// prompter answers questions as the author would at a terminal. When it's
	// nil, stdin isn't a terminal.
	prompter *scriptedPrompter
	// realMachine makes runs inherit the real environment, so the real tools
	// and their caches work. Only health tests set it.
	realMachine bool
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
	env := cli.Env{
		Stdin:   strings.NewReader(""),
		Stdout:  &stdout,
		Stderr:  &stderr,
		Dir:     s.dir,
		Environ: s.environ(),
		Version: testVersion,
	}
	if s.prompter != nil {
		env.Prompter = s.prompter
	}
	code := cli.Run(s.t.Context(), args, env)
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
	base := []string{
		"PATH=" + filepath.Join(s.home, "bin") + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + s.home,
	}
	if s.realMachine {
		// mise refuses to read a mise.toml it wasn't told to trust.
		base = append(os.Environ(), "MISE_TRUSTED_CONFIG_PATHS="+s.dir)
	}
	return append(append(base,
		"GIT_CONFIG_GLOBAL="+filepath.Join(s.home, "gitconfig"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	), s.extraEnv...)
}

// fakeTool puts an executable shell script called name first on the sandbox's
// PATH, so sow starts it in place of the real tool.
func (s *sandbox) fakeTool(name, script string) {
	s.t.Helper()
	target := filepath.Join(s.home, "bin", name)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		s.t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		s.t.Fatal(err)
	}
}

// isolatePath makes the sandbox's bin directory the only directory on its
// PATH, so a tool the test didn't fake isn't installed. For each name in
// realTools, it adds a fake tool that runs the real one.
func (s *sandbox) isolatePath(realTools ...string) {
	s.t.Helper()
	for _, name := range realTools {
		executable, err := exec.LookPath(name)
		if err != nil {
			s.t.Fatal(err)
		}
		s.fakeTool(name, `exec "`+executable+`" "$@"`)
	}
	s.extraEnv = append(s.extraEnv, "PATH="+filepath.Join(s.home, "bin"))
}

// terminal makes stdin a terminal where the author types the typed answers,
// by key, and returns the prompter that records what sow asked.
func (s *sandbox) terminal(typed map[string]string) *scriptedPrompter {
	s.prompter = &scriptedPrompter{typed: typed}
	return s.prompter
}

// errCtrlC is what a scriptedPrompter returns when the author presses Ctrl+C.
var errCtrlC = errors.New("user aborted")

// scriptedPrompter is the author at a terminal. A question the test didn't
// script behaves as if the author pressed Ctrl+C.
type scriptedPrompter struct {
	typed map[string]string
	// asked lists the questions sow asked, in order.
	asked []answers.Question
}

// Ask records q and returns the scripted answer for it.
func (p *scriptedPrompter) Ask(_ context.Context, q answers.Question) (string, error) {
	p.asked = append(p.asked, q)
	answer, ok := p.typed[q.Key]
	if !ok {
		return "", errCtrlC
	}
	return answer, nil
}

// askedKeys returns the keys of the questions sow asked, in order.
func (p *scriptedPrompter) askedKeys() []string {
	keys := []string{}
	for _, q := range p.asked {
		keys = append(keys, q.Key)
	}
	return keys
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

// edit replaces a file in the working directory, such as "demo/package.json",
// with what change returns for its contents. It puts the original back when
// the test ends.
func (s *sandbox) edit(name string, change func(string) string) {
	s.t.Helper()
	original := s.read(name)
	s.write(s.dir, name, change(original))
	s.t.Cleanup(func() {
		s.write(s.dir, name, original)
	})
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

// git runs git in the project directory and returns its trimmed output.
func (s *sandbox) git(args ...string) string {
	s.t.Helper()
	cmd := exec.CommandContext(s.t.Context(), "git", args...)
	cmd.Dir = filepath.Join(s.dir, "demo")
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

// fakeGH puts a fake gh CLI on the sandbox's PATH and a bare repo in the
// sandbox where git pushes in place of GitHub. It returns the bare repo's
// path.
//
// The fake appends each call's arguments to a file that ghCalls reads, and
// prefixes each with "before push" or "after push", which it reads from the
// bare repo. "auth status" passes, and every other call succeeds unless its
// arguments contain failOn, when it prints an error and exits 1. An empty
// failOn never fails.
//
// The sandbox's git config sends https://github.com/ to the sandbox, so sow
// adds the real URL as the remote and the push still stays on the machine.
func (s *sandbox) fakeGH(failOn string) string {
	s.t.Helper()
	remotes := filepath.Join(s.home, "github")
	bare := filepath.Join(remotes, "test-user", "demo.git")
	s.write(s.home, "gitconfig", "[url \""+remotes+"/\"]\n\tinsteadOf = https://github.com/\n")
	cmd := exec.CommandContext(s.t.Context(), "git", "init", "--bare", "--quiet", "--initial-branch", "main", bare)
	cmd.Env = s.environ()
	if output, err := cmd.CombinedOutput(); err != nil {
		s.t.Fatalf("git init --bare: %v\n%s", err, output)
	}
	s.fakeTool("gh", `when="before push"
git --git-dir "`+bare+`" rev-parse --verify --quiet refs/heads/main >/dev/null && when="after push"
echo "$when: $*" >> "`+s.ghCallsPath()+`"
case "$*" in
"auth status"*) exit 0 ;;
esac
if [ -n "`+failOn+`" ]; then
  case "$*" in
  *"`+failOn+`"*) echo "HTTP 403: Resource not accessible" >&2; exit 1 ;;
  esac
fi
exit 0`)
	return bare
}

// ghCallsPath is the file where the fake gh records its calls.
func (s *sandbox) ghCallsPath() string {
	return filepath.Join(s.home, "gh-calls")
}

// ghCalls returns the calls the fake gh recorded, one per line, or nothing if
// gh never ran.
func (s *sandbox) ghCalls() []string {
	s.t.Helper()
	content, err := os.ReadFile(s.ghCallsPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		s.t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
}
