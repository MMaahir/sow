package cli_test

import (
	"strings"
	"testing"
)

func TestNewFrontendReactRendersTheFrontendInTheWorkspace(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", fakeVP)

	s.mustRun("new", "demo", "--no-input", "--backend", "ts", "--frontend", "react")

	wantContains(t, s.read("demo/.sow-answers.yml"), "backend: ts\nfrontend: react\n")
	for _, name := range []string{
		"package.json",
		"pnpm-workspace.yaml",
		"vite.config.ts",
		"backend/package.json",
		"frontend/package.json",
		"frontend/tsconfig.json",
		"frontend/vite.config.ts",
		"frontend/mise.toml",
		"frontend/index.html",
		"frontend/src/main.tsx",
		"frontend/src/router.ts",
		"frontend/src/routeTree.gen.ts",
		"frontend/src/test-setup.ts",
		"frontend/src/routes/__root.tsx",
		"frontend/src/routes/index.tsx",
		"frontend/src/routes/index.test.tsx",
	} {
		if !s.exists("demo/" + name) {
			t.Errorf("%s is missing", name)
		}
	}
	wantContains(t, s.read("demo/frontend/index.html"), "<title>demo</title>")
	wantContains(t, s.read("demo/.gitignore"), ".tanstack/\n")
	if got := s.git("status", "--porcelain"); got != "" {
		t.Errorf("uncommitted files after sow new:\n%s", got)
	}
}

func TestNewFrontendReactGivesTheStackInterfaceTasks(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", fakeVP)

	s.mustRun("new", "demo", "--no-input", "--backend", "ts", "--frontend", "react")

	want := `[tasks.dev]
description = "Run the frontend's dev server, which sends /api to the backend"
run = "vp dev"

[tasks.test]
description = "Run the frontend tests once"
run = "vp test run"

[tasks.lint]
description = "Check formatting, lint rules and types"
run = "vp check"

[tasks.build]
description = "Build the frontend into dist/"
run = "vp build"
`
	if got := s.read("demo/frontend/mise.toml"); got != want {
		t.Errorf("frontend/mise.toml =\n%s\nwant\n%s", got, want)
	}
}

func TestNewFrontendReactSharedFilesCoverBothComponents(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", fakeVP)

	s.mustRun("new", "demo", "--no-input", "--backend", "ts", "--frontend", "react")

	wantContains(t, s.read("demo/mise.toml"),
		"run = [\n  \"pre-commit run --all-files\",\n  \"mise --cd backend run lint\",\n  \"mise --cd backend run test\",\n"+
			"  \"mise --cd frontend run lint\",\n  \"mise --cd frontend run test\",\n]\n\n[tasks.check-vite-plus-pins]\n")
	wantContains(t, s.read("demo/.pre-commit-config.yaml"), "entry: vp check --fix")
	wantContains(t, s.read("demo/AGENTS.md"),
		"mise run check` before you commit.\n\n## TypeScript\n\nThe backend is in `backend/`.",
		"`mise run build` and `mise run openapi`.\n\nThe frontend is in `frontend/`. Run its tasks from that directory: "+
			"`mise run dev`, `mise run test`, `mise run lint` and `mise run build`.\n\nVite+ is the toolchain.",
		"- Import test functions from `vite-plus/test`.\n- Relative imports in `backend/` end in `.ts`",
		"commit `openapi.json`.\n- Routes are files in `frontend/src/routes/`.",
		"\n- After `vite-plus` changes version, run `mise run upgrade-vite-plus` from the root")
}

func TestNewBackendTSSharedFilesLeaveTheFrontendOut(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", fakeVP)

	s.mustRun("new", "demo", "--no-input", "--backend", "ts")

	wantContains(t, s.read("demo/mise.toml"),
		"  \"mise --cd backend run test\",\n]\n\n[tasks.check-vite-plus-pins]\n")
	wantContains(t, s.read("demo/AGENTS.md"),
		"`mise run build` and `mise run openapi`.\n\nVite+ is the toolchain.",
		"commit `openapi.json`.\n- After `vite-plus` changes version")
	for _, file := range []string{"demo/mise.toml", "demo/AGENTS.md"} {
		if strings.Contains(s.read(file), "frontend") {
			t.Errorf("%s mentions the frontend in a project with no frontend", file)
		}
	}
	if s.exists("demo/frontend") {
		t.Error("--backend ts rendered frontend/")
	}
}
