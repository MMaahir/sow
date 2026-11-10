package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// fakeVP stands in for Vite+'s CLI. Like the real "vp install", it writes the
// lockfile.
const fakeVP = `[ "$1" = install ] && echo "lockfileVersion: '9.0'" > pnpm-lock.yaml`

func TestNewBackendTSRendersTheBackendAndTheWorkspace(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", fakeVP)

	s.mustRun("new", "demo", "--no-input", "--backend", "ts")

	wantContains(t, s.read("demo/.sow-answers.yml"), "backend: ts\n")
	for _, name := range []string{
		"package.json",
		"pnpm-workspace.yaml",
		"vite.config.ts",
		".node-version",
		".vscode/extensions.json",
		".vscode/settings.json",
		"backend/package.json",
		"backend/tsconfig.json",
		"backend/vite.config.ts",
		"backend/mise.toml",
		"backend/openapi.json",
		"backend/src/app.ts",
		"backend/src/app.test.ts",
		"backend/src/index.ts",
		"backend/scripts/export-openapi.ts",
	} {
		if !s.exists("demo/" + name) {
			t.Errorf("%s is missing", name)
		}
	}
	if got := s.git("status", "--porcelain"); got != "" {
		t.Errorf("uncommitted files after sow new:\n%s", got)
	}
}

func TestNewBackendTSSharedFilesCoverTheBackend(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", fakeVP)

	s.mustRun("new", "demo", "--no-input", "--backend", "ts")

	wantContains(t, s.read("demo/mise.toml"),
		"  \"pre-commit run --all-files\",\n  \"mise --cd backend run lint\",\n  \"mise --cd backend run test\",\n]")
	wantContains(t, s.read("demo/.pre-commit-config.yaml"), "entry: vp check --fix")
	wantContains(t, s.read("demo/AGENTS.md"), "## TypeScript", "Never call `vite`, `vitest`, `oxlint` or `oxfmt` directly.")
	wantContains(t, s.read("demo/.gitignore"), "node_modules/\n", "dist/\n")
}

func TestNewMinimalSharedFilesLeaveTheBackendOut(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input", "--preset", "minimal")

	wantContains(t, s.read("demo/mise.toml"), "run = [\n  \"pre-commit run --all-files\",\n]\n")
	for file, absent := range map[string]string{
		"demo/mise.toml":               "backend",
		"demo/.pre-commit-config.yaml": "vp",
		"demo/AGENTS.md":               "TypeScript",
	} {
		if strings.Contains(s.read(file), absent) {
			t.Errorf("%s mentions %q in a project with no backend", file, absent)
		}
	}
	for _, name := range []string{"backend", "package.json", "pnpm-workspace.yaml", "pnpm-lock.yaml"} {
		if s.exists("demo/" + name) {
			t.Errorf("the minimal preset rendered %s", name)
		}
	}
}

func TestNewBackendTSInstallsDependenciesAndCommitsTheLockfile(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", fakeVP)

	s.mustRun("new", "demo", "--no-input", "--backend", "ts")

	wantContains(t, s.git("ls-tree", "-r", "--name-only", "HEAD"), "pnpm-lock.yaml")
}

func TestNewMinimalDoesNotRunVP(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", "exit 1")

	s.mustRun("new", "demo", "--no-input", "--preset", "minimal")
}

func TestNewReportsAFailedFollowUpStepAndKeepsTheFiles(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", "echo registry unreachable >&2; exit 3")

	got := s.run("new", "demo", "--no-input", "--backend", "ts")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr,
		"install the dependencies",
		"vp install failed in "+filepath.Join(s.dir, "demo"),
		"registry unreachable")
	if !s.exists("demo/backend/package.json") {
		t.Error("the rendered files are gone after a failed follow-up step")
	}
	if s.exists("demo/.git") {
		t.Error("sow new made a commit despite the failed follow-up step")
	}
}
