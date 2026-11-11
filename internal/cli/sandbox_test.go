package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/MMaahir/sow"
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
	// version is the sow version the sandbox runs as. It's testVersion when
	// empty.
	version string
	// template is the template sow renders. It's the real one unless a test
	// calls install.
	template fs.FS
	// repo is the fixture git repo of sow that release creates, or "".
	repo string
	// ghProtocol is the git protocol the fake gh reports, "https" or "ssh".
	// It's "https" when empty, gh's default.
	ghProtocol string
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
	s := &sandbox{t: t, dir: t.TempDir(), home: t.TempDir(), template: sow.Template()}
	s.writePersonalDefaults("user_name: Test User\nuser_email: test@example.com\ngithub_user: test-user\n")
	return s
}

// run executes sow in the sandbox's working directory.
func (s *sandbox) run(args ...string) result {
	s.t.Helper()
	return s.runIn(".", args...)
}

// runIn executes sow in dir, a directory inside the working directory such as
// "demo".
func (s *sandbox) runIn(dir string, args ...string) result {
	s.t.Helper()
	var stdout, stderr bytes.Buffer
	env := cli.Env{
		Stdin:    strings.NewReader(""),
		Stdout:   &stdout,
		Stderr:   &stderr,
		Dir:      filepath.Join(s.dir, filepath.FromSlash(dir)),
		Environ:  s.environ(),
		Version:  testVersion,
		Template: s.template,
	}
	if s.version != "" {
		env.Version = s.version
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
		// A repo that doesn't exist, so sow update never reaches the network.
		// release points SOW_REPO at a fixture repo.
		"SOW_REPO=" + filepath.Join(s.home, "no-sow-repo"),
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

// projectAt creates the project demo and commits it with version recorded as
// its sow_version, as if a sow at that version had created it.
func (s *sandbox) projectAt(version string) {
	s.t.Helper()
	s.mustRun("new", "demo", "--no-input")
	s.write(s.dir, "demo/.sow-answers.yml", strings.Replace(
		s.read("demo/.sow-answers.yml"), "sow_version: "+testVersion+"\n", "sow_version: "+version+"\n", 1))
	s.git("commit", "--all", "--message", "record sow "+version)
}

// install makes the sandbox run as sow version with files as its template.
// Each key of files is a path inside the template, such as
// "base/README.md.tmpl".
func (s *sandbox) install(version string, files map[string]string) {
	s.t.Helper()
	template := fstest.MapFS{}
	for name, content := range files {
		template[name] = &fstest.MapFile{Data: []byte(content)}
	}
	s.version = version
	s.template = template
}

// release commits files under template/ in a fixture git repo of sow, tags
// the commit v<version>, and points SOW_REPO at the repo. Each release
// replaces the whole template/ directory. The repo is local, so sow update
// fetches it without the network.
func (s *sandbox) release(version string, files map[string]string) {
	s.t.Helper()
	if s.repo == "" {
		s.repo = s.t.TempDir()
		s.gitIn(s.repo, "init", "--initial-branch", "main")
		s.extraEnv = append(s.extraEnv, "SOW_REPO="+s.repo)
	}
	if err := os.RemoveAll(filepath.Join(s.repo, "template")); err != nil {
		s.t.Fatal(err)
	}
	for name, content := range files {
		s.write(s.repo, "template/"+name, content)
	}
	s.gitIn(s.repo, "add", "--all")
	s.gitIn(s.repo, "commit", "--message", "release "+version)
	s.gitIn(s.repo, "tag", "v"+version)
}

// releasedProject creates the project demo with sow version and files as its
// template, and releases that template in the fixture repo. Afterwards the
// sandbox runs as sow testVersion with newFiles as its template.
func (s *sandbox) releasedProject(version string, files, newFiles map[string]string) {
	s.t.Helper()
	s.release(version, files)
	s.install(version, files)
	s.mustRun("new", "demo", "--no-input")
	s.install(testVersion, newFiles)
}

// commit commits every change in the project directory.
func (s *sandbox) commit() {
	s.t.Helper()
	s.git("add", "--all")
	s.git("commit", "--message", "change the project")
}

// files returns every file under dir in the working directory, except those
// in .git, by path relative to dir. A symlink's entry is "symlink to " and its
// target, and files doesn't follow it.
func (s *sandbox) files(dir string) map[string]string {
	s.t.Helper()
	root := filepath.Join(s.dir, filepath.FromSlash(dir))
	files := map[string]string{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(name)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(relative)] = "symlink to " + target
			return nil
		}
		content, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = string(content)
		return nil
	})
	if err != nil {
		s.t.Fatal(err)
	}
	return files
}

// git runs git in the project directory and returns its trimmed output.
func (s *sandbox) git(args ...string) string {
	s.t.Helper()
	return s.gitIn(filepath.Join(s.dir, "demo"), args...)
}

// gitIn runs git in dir, an absolute path, and returns its trimmed output.
func (s *sandbox) gitIn(dir string, args ...string) string {
	s.t.Helper()
	cmd := exec.CommandContext(s.t.Context(), "git", args...)
	cmd.Dir = dir
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
// bare repo. "auth status" passes, "config get git_protocol" prints
// s.ghProtocol, and every other call succeeds unless its arguments contain
// failOn, when it prints an error and exits 1. An empty failOn never fails.
//
// The sandbox's git config sends https://github.com/ and git@github.com: to
// the sandbox, so sow adds the real URL as the remote and the push still
// stays on the machine.
func (s *sandbox) fakeGH(failOn string) string {
	s.t.Helper()
	protocol := s.ghProtocol
	if protocol == "" {
		protocol = "https"
	}
	remotes := filepath.Join(s.home, "github")
	bare := filepath.Join(remotes, "test-user", "demo.git")
	s.write(s.home, "gitconfig", "[url \""+remotes+"/\"]\n"+
		"\tinsteadOf = https://github.com/\n\tinsteadOf = git@github.com:\n")
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
"config get git_protocol"*) echo "`+protocol+`"; exit 0 ;;
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

// withEnvExample returns files with a .env.example and a .gitignore that
// ignores .env, as the real template has.
func withEnvExample(files map[string]string) map[string]string {
	files = maps.Clone(files)
	files["base/.env.example"] = "PORT=3000\n"
	files["base/.gitignore"] = ".env\n"
	return files
}

// withWorkspace returns files with a pnpm-workspace.yaml, which turns on the
// follow-up step that runs vp install.
func withWorkspace(files map[string]string) map[string]string {
	files = maps.Clone(files)
	files["base/pnpm-workspace.yaml"] = "packages: []\n"
	return files
}

// withBackendTS returns files with the directories the ts backend selects,
// backend-ts and typescript, so a project whose answers file records
// backend: ts renders.
func withBackendTS(files map[string]string) map[string]string {
	files = maps.Clone(files)
	files["backend-ts/backend/index.ts"] = "export {};\n"
	files["typescript/package.json"] = "{}\n"
	return files
}
