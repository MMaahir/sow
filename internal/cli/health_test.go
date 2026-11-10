package cli_test

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	s := newHealthSandbox(t, "mise", "pre-commit")

	s.newProject("--preset", "minimal")

	s.mustTool(".", "mise", "run", "check")
}

func TestNewBackendTSProjectHealth(t *testing.T) {
	s := newHealthSandbox(t, "mise", "pre-commit", "vp", "node")
	s.newProject("--backend", "ts")

	t.Run("the root check task passes", func(t *testing.T) {
		s := s.in(t)
		s.mustTool(".", "mise", "run", "check")
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

		s.mustTool("backend", "docker", "build", ".")
	})
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
