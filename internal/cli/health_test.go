package cli_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"maps"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MMaahir/sow/internal/answers"
)

// newHealthSandbox returns a sandbox for a generated project health test. Its
// runs inherit the real machine's environment, because the real tools need
// their own caches and settings. It skips the test in short mode, and when one
// of the tools isn't installed.
func newHealthSandbox(t *testing.T, tools ...string) *sandbox {
	t.Helper()
	if testing.Short() {
		t.Skip("generated project health tests are slow: run mise run test-health")
	}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			// mise run test-health sets SOW_HEALTH_STRICT, so a missing tool
			// fails the run there instead of skipping a check silently.
			if os.Getenv("SOW_HEALTH_STRICT") != "" {
				t.Fatalf("%s isn't installed, and SOW_HEALTH_STRICT is set: install it, "+
					"or run go test ./... to skip the checks that need it", tool)
			}
			t.Skipf("%s isn't installed: install it to run this test", tool)
		}
	}
	s := newSandbox(t)
	s.realMachine = true
	return s
}

// newProject runs sow new for a project called demo. It passes the personal
// answers as flags, so the real machine's personal defaults file isn't needed.
func (s *sandbox) newProject(args ...string) {
	s.t.Helper()
	s.mustRun(append([]string{
		"new", "demo", "--no-input",
		"--user-name", "Test User", "--user-email", "test@example.com", "--github-user", "test-user",
	}, args...)...)
}

// in returns a copy of the sandbox that reports to a subtest. A subtest calls
// it first, because a failure must be reported to the test that is running.
func (s *sandbox) in(t *testing.T) *sandbox {
	t.Helper()
	sub := *s
	sub.t = t
	return &sub
}

// tool runs a real tool in a directory of the demo project, such as "." or
// "backend", and returns its output and whether it succeeded.
func (s *sandbox) tool(dir, name string, args ...string) (string, error) {
	s.t.Helper()
	cmd := exec.CommandContext(s.t.Context(), name, args...)
	cmd.Dir = filepath.Join(s.dir, "demo", dir)
	cmd.Env = s.environ()
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// mustTool runs a real tool in the demo project and fails the test if the
// tool fails.
func (s *sandbox) mustTool(dir, name string, args ...string) string {
	s.t.Helper()
	output, err := s.tool(dir, name, args...)
	if err != nil {
		s.t.Fatalf("%s %s failed in %s: %v\n%s", name, strings.Join(args, " "), dir, err, output)
	}
	return output
}

func TestNewMinimalProjectHealth(t *testing.T) {
	s := newHealthSandbox(t, "mise")

	s.newProject("--preset", "minimal")

	s.mustTool(".", "mise", "install")
	s.mustTool(".", "mise", "run", "check")
}

func TestNewBackendTSProjectHealth(t *testing.T) {
	s := newHealthSandbox(t, "mise", "vp", "node")
	// Where vp runs as pnpm exec vp, pnpm installs an edited manifest before
	// the pin check can read it. The global vp doesn't.
	s.extraEnv = append(s.extraEnv, "pnpm_config_verify_deps_before_run=false")
	s.newProject("--backend", "ts")
	s.mustTool(".", "mise", "install")

	t.Run("the root check task passes", func(t *testing.T) {
		s := s.in(t)
		s.mustTool(".", "mise", "run", "check")
	})

	t.Run("vp check passes at the root", func(t *testing.T) {
		s := s.in(t)
		s.mustTool(".", "vp", "check")
	})

	t.Run("lint, test and build pass in the backend", func(t *testing.T) {
		s := s.in(t)
		for _, task := range []string{"lint", "test", "build"} {
			s.mustTool("backend", "mise", "run", task)
		}
	})

	t.Run("the built server answers on both health endpoints", func(t *testing.T) {
		s := s.in(t)
		port := freePort(t)
		server := exec.CommandContext(t.Context(), "node", "dist/index.mjs")
		server.Dir = filepath.Join(s.dir, "demo", "backend")
		server.Env = append(s.environ(), fmt.Sprintf("PORT=%d", port))
		if err := server.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = server.Process.Kill()
			_ = server.Wait()
		})

		for _, endpoint := range []string{"/healthz", "/readyz"} {
			if got := statusOf(t, fmt.Sprintf("http://127.0.0.1:%d%s", port, endpoint)); got != http.StatusOK {
				t.Errorf("GET %s = %d, want 200", endpoint, got)
			}
		}
	})

	t.Run("the committed openapi.json equals a fresh export", func(t *testing.T) {
		s := s.in(t)
		s.mustTool("backend", "mise", "run", "openapi")

		if changed := s.git("status", "--porcelain", "backend/openapi.json"); changed != "" {
			t.Errorf("mise run openapi changed the committed file:\n%s", s.git("diff", "backend/openapi.json"))
		}
	})

	t.Run("the Vite+ pin check passes", func(t *testing.T) {
		s := s.in(t)
		s.mustTool(".", "mise", "run", "check-vite-plus-pins")
	})

	t.Run("a vite alias at another version fails the pin check", func(t *testing.T) {
		s := s.in(t)
		alias := regexp.MustCompile(`(?m)^  vite: npm:@voidzero-dev/vite-plus-core@.*$`)
		s.edit("demo/pnpm-workspace.yaml", func(content string) string {
			return alias.ReplaceAllString(content, "  vite: npm:@voidzero-dev/vite-plus-core@0.0.1")
		})

		output, err := s.tool(".", "mise", "run", "check-vite-plus-pins")

		if err == nil {
			t.Fatalf("the pin check passed with the vite alias at 0.0.1:\n%s", output)
		}
		wantContains(t, output,
			"vite: the catalog pins npm:@voidzero-dev/vite-plus-core@0.0.1",
			"mise run upgrade-vite-plus")
	})

	for name, override := range map[string]string{
		"a missing vite override fails the pin check":       "",
		"a vite override to plain Vite fails the pin check": "  vite@*: npm:vite@8.3.7\n",
	} {
		t.Run(name, func(t *testing.T) {
			s := s.in(t)
			s.edit("demo/pnpm-workspace.yaml", func(content string) string {
				return strings.Replace(content, "  vite@*: \"catalog:\"\n", override, 1)
			})

			output, err := s.tool(".", "mise", "run", "check-vite-plus-pins")

			if err == nil {
				t.Fatalf("the pin check passed with the vite override changed:\n%s", output)
			}
			wantContains(t, output, "vite@* override", "mise run upgrade-vite-plus")
		})
	}

	t.Run("a vitest peer dependency with no override fails the pin check", func(t *testing.T) {
		s := s.in(t)
		s.edit("demo/backend/package.json", func(content string) string {
			return strings.Replace(content, `"devDependencies": {`, `"peerDependencies": {
    "@vitest/coverage-v8": ">=5"
  },
  "devDependencies": {`, 1)
		})

		output, err := s.tool(".", "mise", "run", "check-vite-plus-pins")

		if err == nil {
			t.Fatalf("the pin check passed with a Vitest peer dependency and no override:\n%s", output)
		}
		wantContains(t, output,
			"vitest: backend/package.json depends on Vitest directly, but no vitest@* override pins Vitest")
	})

	t.Run("a direct vitest dependency with no override fails the pin check", func(t *testing.T) {
		s := s.in(t)
		s.edit("demo/backend/package.json", func(content string) string {
			return strings.Replace(content, `"vite-plus": "catalog:"`, `"vite-plus": "catalog:",
    "vitest": ">=5"`, 1)
		})

		output, err := s.tool(".", "mise", "run", "check-vite-plus-pins")

		if err == nil {
			t.Fatalf("the pin check passed with a direct vitest dependency:\n%s", output)
		}
		wantContains(t, output,
			"vitest: backend/package.json pins >=5, but vp toolchain reports Vitest",
			"vitest: backend/package.json depends on Vitest directly, but no vitest@* override pins Vitest")
	})

	t.Run("a vitest override at another version fails the pin check", func(t *testing.T) {
		s := s.in(t)
		s.edit("demo/pnpm-workspace.yaml", func(content string) string {
			return strings.Replace(content, "overrides:\n", "overrides:\n  vitest@*: 0.0.1\n", 1)
		})

		output, err := s.tool(".", "mise", "run", "check-vite-plus-pins")

		if err == nil {
			t.Fatalf("the pin check passed with the vitest override at 0.0.1:\n%s", output)
		}
		wantContains(t, output, "vitest: the vitest@* override pins 0.0.1, but vp toolchain reports Vitest")
	})

	t.Run("upgrade-vite-plus leaves a project that is already pinned unchanged", func(t *testing.T) {
		s := s.in(t)
		s.mustTool(".", "mise", "run", "upgrade-vite-plus")

		if changed := s.git("status", "--porcelain"); changed != "" {
			t.Errorf("mise run upgrade-vite-plus changed the project:\n%s", changed)
		}
		s.mustTool(".", "mise", "run", "check-vite-plus-pins")
	})

	t.Run("the working tree is clean after every task", func(t *testing.T) {
		s := s.in(t)
		if changed := s.git("status", "--porcelain"); changed != "" {
			t.Errorf("the tasks left uncommitted files:\n%s", changed)
		}
	})

	typeError := "export const wrong: number = \"text\";\n"
	for name, file := range map[string]string{
		"a type error in a source file fails lint": "backend/src/wrong.ts",
		"a type error in a test file fails lint":   "backend/src/wrong.test.ts",
	} {
		t.Run(name, func(t *testing.T) {
			s := s.in(t)
			s.write(s.dir, "demo/"+file, typeError)
			t.Cleanup(func() {
				if err := os.Remove(filepath.Join(s.dir, "demo", file)); err != nil {
					t.Error(err)
				}
			})

			output, err := s.tool("backend", "mise", "run", "lint")

			if err == nil {
				t.Fatalf("lint passed despite a type error in %s:\n%s", file, output)
			}
			wantContains(t, output, filepath.Base(file))
		})
	}

	t.Run("the backend image builds", func(t *testing.T) {
		s := s.in(t)
		if !s.exists("demo/backend/Dockerfile") {
			t.Skip("backend/Dockerfile doesn't exist yet: the author writes it")
		}
		if _, err := exec.LookPath("docker"); err != nil {
			t.Skip("docker isn't installed: install it to run this test")
		}

		// The build context is the project root, because the package's
		// manifest reads its Vite+ version from the root pnpm catalog.
		s.mustTool(".", "docker", "build", "--file", "backend/Dockerfile", ".")
	})
}

func TestNewFrontendReactProjectHealth(t *testing.T) {
	s := newHealthSandbox(t, "mise", "vp", "node")
	s.newProject("--backend", "ts", "--frontend", "react")
	s.mustTool(".", "mise", "install")

	t.Run("the root check task passes", func(t *testing.T) {
		s := s.in(t)
		s.mustTool(".", "mise", "run", "check")
	})

	t.Run("lint, test and build pass in the frontend", func(t *testing.T) {
		s := s.in(t)
		for _, task := range []string{"lint", "build"} {
			s.mustTool("frontend", "mise", "run", task)
		}
		if !s.exists("demo/frontend/dist/index.html") {
			t.Error("mise run build wrote no frontend/dist/index.html")
		}

		output := s.mustTool("frontend", "mise", "run", "test", "--reporter=verbose")

		wantContains(t, output,
			"the starter route shows the status from GET /api/healthz",
			"a route that doesn't exist shows the not-found view")
	})

	t.Run("e2e fails within a minute and names the address when nothing listens there", func(t *testing.T) {
		s := s.in(t)
		address := fmt.Sprintf("http://127.0.0.1:%d", freePort(t))
		s.edit("demo/.env", func(content string) string {
			return strings.Replace(content, "E2E_BASE_URL=http://localhost:8080", "E2E_BASE_URL="+address, 1)
		})
		start := time.Now()

		output, err := s.tool(".", "mise", "run", "e2e")

		if err == nil {
			t.Fatalf("mise run e2e passed with nothing listening at %s:\n%s", address, output)
		}
		if elapsed := time.Since(start); elapsed > time.Minute {
			t.Errorf("mise run e2e took %s to fail, want under a minute", elapsed)
		}
		wantContains(t, output, "nothing answers at "+address)
	})

	t.Run("the smoke tests pass against the Compose stack", func(t *testing.T) {
		s := s.in(t)
		if !s.exists("demo/compose.yaml") {
			t.Skip("compose.yaml doesn't exist yet: the author writes it")
		}
		if _, err := exec.LookPath("docker"); err != nil {
			t.Skip("docker isn't installed: install it to run this test")
		}
		if !s.chromiumInstalled() {
			t.Skip("Playwright's Chromium isn't installed: run pnpm exec playwright install --only-shell chromium in frontend/")
		}
		s.composeUp()

		output := s.mustTool(".", "mise", "run", "e2e")

		wantContains(t, output, "2 passed")

		t.Run("the home page smoke test fails with the backend stopped", func(t *testing.T) {
			s := s.in(t)
			s.mustTool(".", "docker", s.compose("stop", "backend")...)

			output, err := s.tool(".", "mise", "run", "e2e")

			if err == nil {
				t.Fatalf("mise run e2e passed with the backend stopped:\n%s", output)
			}
			_, failures, _ := strings.Cut(output, "1 failed")
			wantContains(t, failures, "the home page shows the backend's health status")
		})
	})

	t.Run("generate-client leaves a fresh project unchanged", func(t *testing.T) {
		s := s.in(t)
		s.mustTool("frontend", "mise", "run", "generate-client")

		if changed := s.git("status", "--porcelain"); changed != "" {
			t.Errorf("mise run generate-client changed the project:\n%s", changed)
		}
	})

	t.Run("a new backend route changes the client and fails the freshness check", func(t *testing.T) {
		s := s.in(t)
		s.edit("demo/backend/src/app.ts", func(content string) string {
			return strings.Replace(content,
				`  .openapi(readyz, (c) => c.json({ status: "ok" as const }, 200));`,
				`  .openapi(readyz, (c) => c.json({ status: "ok" as const }, 200))
  .openapi(
    createRoute({ method: "get", path: "/version", summary: "Version.", responses: statusResponse }),
    (c) => c.json({ status: "ok" as const }, 200),
  );`, 1)
		})
		// Editing with no change makes the sandbox put the generated files back when the test ends.
		unchanged := func(content string) string { return content }
		s.edit("demo/backend/openapi.json", unchanged)
		s.edit("demo/frontend/src/client.gen.ts", unchanged)
		s.mustTool("backend", "mise", "run", "openapi")

		s.mustTool("frontend", "mise", "run", "generate-client")

		if output, err := s.tool(".", "git", "diff", "--exit-code", "--", "frontend/src/client.gen.ts"); err == nil {
			t.Errorf("git diff --exit-code passed after the backend gained a route:\n%s", output)
		}
		wantContains(t, s.read("demo/frontend/src/client.gen.ts"),
			"export const getVersion = async", "export function useGetVersion", "`/api/version`")
	})

	t.Run("the root check regenerates the client before it type-checks a new endpoint's caller", func(t *testing.T) {
		s := s.in(t)
		s.edit("demo/backend/src/app.ts", func(content string) string {
			return strings.Replace(content,
				`  .openapi(readyz, (c) => c.json({ status: "ok" as const }, 200));`,
				`  .openapi(readyz, (c) => c.json({ status: "ok" as const }, 200))
  .openapi(
    createRoute({
      method: "get",
      path: "/version",
      summary: "Version.",
      responses: statusResponse,
    }),
    (c) => c.json({ status: "ok" as const }, 200),
  );`, 1)
		})
		// A real change updates the backend's test of its route list too.
		s.edit("demo/backend/src/app.test.ts", func(content string) string {
			return strings.Replace(content, `toEqual(["/healthz", "/readyz"])`, `toEqual(["/healthz", "/readyz", "/version"])`, 1)
		})
		unchanged := func(content string) string { return content }
		s.edit("demo/backend/openapi.json", unchanged)
		s.edit("demo/frontend/src/client.gen.ts", unchanged)
		s.mustTool("backend", "mise", "run", "openapi")
		// The starter route calls the new endpoint while the committed client doesn't have it yet.
		s.edit("demo/frontend/src/routes/index.tsx", func(content string) string {
			content = strings.Replace(content, `import { useGetHealthz } from "../client.gen";`,
				`import { useGetHealthz, useGetVersion } from "../client.gen";`, 1)
			return strings.Replace(content, "  const { data, isError } = useGetHealthz();",
				"  const { data, isError } = useGetHealthz();\n  useGetVersion();", 1)
		})

		// The edits are already formatted, so the pre-commit hooks change no file.
		s.mustTool(".", "mise", "run", "check")
	})

	t.Run("the Vite+ pin check passes", func(t *testing.T) {
		s := s.in(t)
		s.mustTool(".", "mise", "run", "check-vite-plus-pins")
	})

	t.Run("the working tree is clean after every task", func(t *testing.T) {
		s := s.in(t)
		if changed := s.git("status", "--porcelain"); changed != "" {
			t.Errorf("the tasks left uncommitted files:\n%s", changed)
		}
	})

	typeError := "export const wrong: number = \"text\";\n"
	conditionalHook := `import { useState } from "react";

export function Wrong({ on }: { on: boolean }) {
  if (on) {
    const [count] = useState(0);
    return <p>{count}</p>;
  }
  return <p>off</p>;
}
`
	for name, c := range map[string]struct{ file, content, want string }{
		"a type error in a source file fails lint": {
			file: "frontend/src/wrong.tsx", content: typeError, want: "TS2322",
		},
		"a type error in a test file fails lint": {
			file: "frontend/src/wrong.test.tsx", content: typeError, want: "TS2322",
		},
		"a hook called conditionally fails lint": {
			file: "frontend/src/wrong.tsx", content: conditionalHook, want: "rules-of-hooks",
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := s.in(t)
			s.write(s.dir, "demo/"+c.file, c.content)
			t.Cleanup(func() {
				if err := os.Remove(filepath.Join(s.dir, "demo", c.file)); err != nil {
					t.Error(err)
				}
			})

			output, err := s.tool("frontend", "mise", "run", "lint")

			if err == nil {
				t.Fatalf("lint passed despite the problem in %s:\n%s", c.file, output)
			}
			wantContains(t, output, filepath.Base(c.file), c.want)
		})
	}

	t.Run("the frontend image builds", func(t *testing.T) {
		s := s.in(t)
		if !s.exists("demo/frontend/Dockerfile") {
			t.Skip("frontend/Dockerfile doesn't exist yet: the author writes it")
		}
		if _, err := exec.LookPath("docker"); err != nil {
			t.Skip("docker isn't installed: install it to run this test")
		}

		// The build context is the project root, because the package's
		// manifest reads its Vite+ version from the root pnpm catalog.
		s.mustTool(".", "docker", "build", "--file", "frontend/Dockerfile", ".")
	})
}

func TestNewWebTSProjectHealth(t *testing.T) {
	s := newHealthSandbox(t, "mise", "vp", "node")
	s.newProject("--preset", "web-ts")
	s.mustTool(".", "mise", "install")

	t.Run("the root check task passes", func(t *testing.T) {
		s := s.in(t)
		s.mustTool(".", "mise", "run", "check")
	})

	t.Run("the Vite+ pin check passes", func(t *testing.T) {
		s := s.in(t)
		s.mustTool(".", "mise", "run", "check-vite-plus-pins")
	})

	t.Run("the working tree is clean after every task", func(t *testing.T) {
		s := s.in(t)
		if changed := s.git("status", "--porcelain"); changed != "" {
			t.Errorf("the tasks left uncommitted files:\n%s", changed)
		}
	})
}

func TestNewEveryCombinationProjectHealth(t *testing.T) {
	if os.Getenv("SOW_TEST_ALL_COMBINATIONS") != "1" {
		t.Skip("every combination of answers is slow to test: set SOW_TEST_ALL_COMBINATIONS=1 to run it")
	}

	for _, combination := range everyCombination() {
		t.Run(combination.name, func(t *testing.T) {
			tools := []string{"mise"}
			if combination.answers.TypeScriptOn() {
				tools = append(tools, "vp", "node")
			}
			s := newHealthSandbox(t, tools...)
			s.newProject(combination.flags...)
			s.mustTool(".", "mise", "install")

			t.Run("the root check task passes", func(t *testing.T) {
				s := s.in(t)
				s.mustTool(".", "mise", "run", "check")
			})

			for _, component := range combination.on {
				t.Run("lint, test and build pass in the "+component, func(t *testing.T) {
					s := s.in(t)
					for _, task := range []string{"lint", "test", "build"} {
						s.mustTool(component, "mise", "run", task)
					}
				})
			}

			if combination.answers.TypeScriptOn() {
				t.Run("the Vite+ pin check passes", func(t *testing.T) {
					s := s.in(t)
					s.mustTool(".", "mise", "run", "check-vite-plus-pins")
				})
			}

			t.Run("the working tree is clean after every task", func(t *testing.T) {
				s := s.in(t)
				if changed := s.git("status", "--porcelain"); changed != "" {
					t.Errorf("the tasks left uncommitted files:\n%s", changed)
				}
			})
		})
	}
}

func TestEveryCombinationListsEachCombinationValidationAccepts(t *testing.T) {
	var got []string

	for _, combination := range everyCombination() {
		got = append(got, combination.name)
	}

	want := []string{
		"backend=none,frontend=none",
		"backend=ts,frontend=none",
		"backend=ts,frontend=react",
	}
	if !slices.Equal(got, want) {
		t.Errorf("combinations = %q, want %q", got, want)
	}
}

// combination is one set of component answers for a generated project.
type combination struct {
	// name lists each answer as key=value, such as
	// "backend=ts,frontend=react". It names the subtest.
	name string
	// answers holds the component answers.
	answers answers.Answers
	// flags passes the answers to sow new, such as "--backend", "ts".
	flags []string
	// on lists the components that are on. Each one renders into a
	// directory of the same name.
	on []string
}

// everyCombination returns every combination of component answers that
// validation accepts, in the order of the questions and their choices. It
// reads the questions, so a new component or stack joins the list with no
// change here, and a combination a question's Needs rejects is left out.
func everyCombination() []combination {
	all := []answers.Answers{{}}
	for _, q := range answers.Questions() {
		if !q.Component {
			continue
		}
		var next []answers.Answers
		for _, partial := range all {
			for _, choice := range q.Choices {
				extended := maps.Clone(partial)
				extended[q.Key] = choice
				next = append(next, extended)
			}
		}
		all = next
	}

	var combinations []combination
	for _, a := range all {
		rejected := slices.ContainsFunc(answers.Settle(a).Problems, func(p answers.Problem) bool {
			return p.Kind != answers.MissingAnswer
		})
		if rejected {
			continue
		}
		c := combination{answers: a}
		var names []string
		for _, q := range answers.Questions() {
			value, ok := a[q.Key]
			if !ok {
				continue
			}
			names = append(names, q.Key+"="+value)
			c.flags = append(c.flags, "--"+strings.ReplaceAll(q.Key, "_", "-"), value)
			if value != answers.None {
				c.on = append(c.on, q.Key)
			}
		}
		c.name = strings.Join(names, ",")
		combinations = append(combinations, c)
	}
	return combinations
}

// freePort returns a TCP port nothing is listening on.
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port
}

// statusOf returns the status code of a GET request. It retries for ten
// seconds while the server starts.
func statusOf(t *testing.T, url string) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		response, err := http.DefaultClient.Do(request)
		if err == nil {
			_ = response.Body.Close()
			return response.StatusCode
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET %s: %v", url, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// installLocation matches where Playwright installs its headless Chromium in
// the output of playwright install --dry-run.
var installLocation = regexp.MustCompile(`(?m)chromium-headless-shell.*\n\s*Install location:\s*(.+)$`)

// chromiumInstalled reports whether the headless Chromium that the demo
// project's Playwright runs is on this machine. Playwright prints where the
// browser belongs without downloading it.
func (s *sandbox) chromiumInstalled() bool {
	s.t.Helper()
	output := s.mustTool("frontend", "pnpm", "exec", "playwright", "install", "--dry-run", "--only-shell", "chromium")
	match := installLocation.FindStringSubmatch(output)
	if match == nil {
		s.t.Fatalf("playwright install --dry-run printed no install location for chromium-headless-shell:\n%s", output)
	}
	_, err := os.Stat(strings.TrimSpace(match[1]))
	return err == nil
}

// composeUp builds and starts the demo project's Compose stack under the
// sandbox's own project name, and waits until its services are healthy. It
// removes the stack when the test ends, even when the test or the start fails.
func (s *sandbox) composeUp() {
	s.t.Helper()
	s.t.Cleanup(func() {
		// The test's context is cancelled before cleanup runs, so down needs one that isn't.
		down := exec.CommandContext(context.WithoutCancel(s.t.Context()), "docker", s.compose("down")...)
		down.Dir = filepath.Join(s.dir, "demo")
		down.Env = s.environ()
		if output, err := down.CombinedOutput(); err != nil {
			s.t.Errorf("docker compose down: %v\n%s", err, output)
		}
	})
	s.mustTool(".", "docker", s.compose("up", "--build", "--wait")...)
}

// compose returns the arguments to docker for a docker compose command on
// this sandbox's own Compose project. Every sandbox generates a directory
// called demo, and a health sandbox inherits COMPOSE_PROJECT_NAME from the
// real machine, so either default could name another stack. Compose takes -p
// over both. The name comes from the sandbox's temporary directory, which is
// unique to the test run, so a subtest's copy of the sandbox gets the same name.
func (s *sandbox) compose(args ...string) []string {
	sum := sha256.Sum256([]byte(s.dir))
	return append([]string{"compose", "-p", fmt.Sprintf("sow-health-%x", sum[:6])}, args...)
}
