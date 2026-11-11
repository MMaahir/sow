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
		"frontend/orval.config.ts",
		"frontend/src/client.gen.ts",
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

func TestNewFrontendReactCommitsTheGeneratedClient(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", fakeVP)

	s.mustRun("new", "demo", "--no-input", "--backend", "ts", "--frontend", "react")

	first := s.git("rev-list", "--max-parents=0", "HEAD")
	wantContains(t, s.git("show", first+":frontend/src/client.gen.ts"),
		"export const getHealthz = async", "export function useGetHealthz", "`/api/healthz`")
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

[tasks.generate-client]
description = "Write the API client in src/client.gen.ts from backend/openapi.json"
run = "pnpm exec orval"
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
		"run = [\n  # The client is regenerated first, because the pre-commit hook type-checks code that may use a new endpoint.\n"+
			"  \"mise --cd frontend run generate-client\",\n  \"pre-commit run --all-files\",\n"+
			"  \"mise --cd backend run lint\",\n  \"mise --cd backend run test\",\n  \"mise --cd frontend run lint\",\n  \"mise --cd frontend run test\",\n]\n\n[tasks.check-vite-plus-pins]\n")
	wantContains(t, s.read("demo/.pre-commit-config.yaml"), "entry: vp check --fix")
	wantContains(t, s.read("demo/AGENTS.md"),
		"mise run check` before you commit.\n\n## TypeScript\n\nThe backend is in `backend/`.",
		"`mise run build` and `mise run openapi`.\n\nThe frontend is in `frontend/`. Run its tasks from that directory: "+
			"`mise run dev`, `mise run test`, `mise run lint`, `mise run build` and `mise run generate-client`.\n\nVite+ is the toolchain.",
		"- The Vitest tests, in files that end in `.test.ts` or `.test.tsx`, import their test functions from `vite-plus/test`.\n"+
			"- Relative imports in `backend/` end in `.ts`",
		"commit `openapi.json`.\n- Routes are files in `frontend/src/routes/`.",
		"`.test.tsx`.\n- The frontend calls the backend through the client in `frontend/src/client.gen.ts`, "+
			"which `mise run generate-client` writes from `backend/openapi.json`.",
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
		"`oxfmt` directly.\n- The Vitest tests, in files that end in `.test.ts` or `.test.tsx`, import their test functions from `vite-plus/test`.\n"+
			"- Relative imports in `backend/` end in `.ts`",
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

func TestNewFrontendReactGivesTheRootE2ETask(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", fakeVP)

	s.mustRun("new", "demo", "--no-input", "--backend", "ts", "--frontend", "react")

	wantContains(t, s.read("demo/mise.toml"), `
[tasks.e2e]
description = "Run the Playwright smoke tests against the app running at E2E_BASE_URL"
dir = "{{config_root}}/frontend"
run = "pnpm exec playwright test"
`)
	for _, name := range []string{"frontend/playwright.config.ts", "frontend/e2e/global-setup.ts", "frontend/e2e/smoke.spec.ts"} {
		if !s.exists("demo/" + name) {
			t.Errorf("%s is missing", name)
		}
	}
	wantContains(t, s.read("demo/.gitignore"), "test-results/\n")
	wantContains(t, s.read("demo/AGENTS.md"),
		"- The Playwright smoke tests in `frontend/e2e/` run against the running app. Start it, then run `mise run e2e` from the root. "+
			"The task reads the app's address from `E2E_BASE_URL` in `.env`. "+
			"These tests import their test functions from `@playwright/test`, never from `vite-plus/test`.\n")
}

func TestNewFrontendReactSetsTheE2EBaseURL(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", fakeVP)

	s.mustRun("new", "demo", "--no-input", "--backend", "ts", "--frontend", "react")

	want := `# Local settings and secrets for this project. Copy it to .env and fill it in. mise loads .env.

# The address of the running app, where mise run e2e runs the Playwright smoke tests.
E2E_BASE_URL=http://localhost:8080
`
	if got := s.read("demo/.env.example"); got != want {
		t.Errorf(".env.example =\n%s\nwant\n%s", got, want)
	}
}

func TestNewBackendTSHasNoE2ETask(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", fakeVP)

	s.mustRun("new", "demo", "--no-input", "--backend", "ts")

	if got := s.read("demo/mise.toml"); strings.Contains(got, "[tasks.e2e]") {
		t.Errorf("mise.toml has the e2e task in a project with no frontend:\n%s", got)
	}
	want := "# Local settings and secrets for this project. Copy it to .env and fill it in. mise loads .env.\n"
	if got := s.read("demo/.env.example"); got != want {
		t.Errorf(".env.example = %q, want %q", got, want)
	}
}

func TestUpdateRejectsAnAnswersFileWithAFrontendAndNoBackend(t *testing.T) {
	s := newSandbox(t)
	file := "sow_version: 1.2.3\nproject_name: demo\nuser_name: Test User\n" +
		"user_email: test@example.com\ngithub_user: test-user\nfrontend: react\n"
	s.write(s.dir, "demo/.sow-answers.yml", file)

	got := s.runIn("demo", "update")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1\n%s", got.code, got.stdout)
	}
	wantContains(t, got.stderr, "frontend needs a backend",
		"set backend to ts in .sow-answers.yml", "run sow update again")
	if strings.Contains(got.stderr, "--backend") {
		t.Errorf("stderr gives sow new's advice:\n%s", got.stderr)
	}
	if got := s.read("demo/.sow-answers.yml"); got != file {
		t.Errorf(".sow-answers.yml = %q, want it untouched", got)
	}
}
