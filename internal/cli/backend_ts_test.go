package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// fakeVP stands in for Vite+'s CLI. "vp env doctor" passes, and like the real
// "vp install", it writes the lockfile. Any other command fails.
const fakeVP = `case "$*" in
"env doctor") ;;
install) echo "lockfileVersion: '9.0'" > pnpm-lock.yaml ;;
*) exit 1 ;;
esac`

func TestNewBackendTSRendersTheBackendAndTheWorkspace(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", fakeVP)

	s.mustRun("new", "demo", "--no-input", "--backend", "ts")

	wantContains(t, s.read("demo/.sow-answers.yml"), "backend: ts\n")
	for _, name := range []string{
		"package.json",
		"pnpm-workspace.yaml",
		"vite.config.ts",
		"tsconfig.json",
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
	// Oxfmt lays out mise.toml differently from the template, so the root config leaves it out.
	wantContains(t, s.read("demo/vite.config.ts"), `"mise.toml"`)
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

func TestNewBackendTSGivesTheVitePlusPinTasks(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", fakeVP)

	s.mustRun("new", "demo", "--no-input", "--backend", "ts")

	wantContains(t, s.read("demo/mise.toml"),
		"[tasks.check-vite-plus-pins]\n",
		"run = \"node scripts/check-vite-plus-pins.ts\"\n",
		"[tasks.upgrade-vite-plus]\n",
		"run = \"pnpm exec vp migrate --no-interactive\"\n")
	if !s.exists("demo/scripts/check-vite-plus-pins.ts") {
		t.Error("scripts/check-vite-plus-pins.ts is missing")
	}
}

func TestNewMinimalHasNoVitePlusPinTasks(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input", "--preset", "minimal")

	for _, task := range []string{"check-vite-plus-pins", "upgrade-vite-plus"} {
		if strings.Contains(s.read("demo/mise.toml"), task) {
			t.Errorf("mise.toml has the %s task in a project with no TypeScript", task)
		}
	}
	if s.exists("demo/scripts") {
		t.Error("the minimal preset rendered scripts/")
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
	s.fakeTool("vp", `[ "$*" = "env doctor" ] && exit 0; echo registry unreachable >&2; exit 3`)

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

func TestNewBackendTSWithoutVPPrintsTheInstallCommand(t *testing.T) {
	s := newSandbox(t)
	s.isolatePath()

	got := s.run("new", "demo", "--no-input", "--backend", "ts")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "vp isn't installed", "curl -fsSL https://vite.plus | bash")
	if s.exists("demo") {
		t.Error("sow new created demo without vp")
	}
}

func TestNewBackendTSReportsAFailingVPEnvDoctor(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", `[ "$*" = "env doctor" ] && { echo "pnpm shim isn't on PATH"; exit 1; }
echo "lockfileVersion: '9.0'" > pnpm-lock.yaml`)

	got := s.run("new", "demo", "--no-input", "--backend", "ts")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr,
		"fix what vp env doctor reports, then try again",
		"vp env doctor failed",
		"pnpm shim isn't on PATH")
	if s.exists("demo") {
		t.Error("sow new created demo despite the failing vp env doctor")
	}
}

func TestNewMinimalWithoutVPSucceeds(t *testing.T) {
	s := newSandbox(t)
	s.isolatePath("git")

	s.mustRun("new", "demo", "--no-input", "--preset", "minimal")

	if !s.exists("demo/.git") {
		t.Error("sow new made no first commit")
	}
}
