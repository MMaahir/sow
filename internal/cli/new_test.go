package cli_test

import (
	"strings"
	"testing"
)

func TestNewRendersTheProjectAndCommitsIt(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input")

	if got, want := s.read("demo/README.md"), "# demo\n"; got != want {
		t.Errorf("README.md = %q, want %q", got, want)
	}
	if got, want := s.git("demo", "log", "--format=%s"), "chore: initial commit from sow"; got != want {
		t.Errorf("commit subjects = %q, want %q", got, want)
	}
	if got, want := s.git("demo", "branch", "--show-current"), "main"; got != want {
		t.Errorf("branch = %q, want %q", got, want)
	}
	if got := s.git("demo", "status", "--porcelain"); got != "" {
		t.Errorf("uncommitted files after sow new:\n%s", got)
	}
}

func TestNewWritesTheAnswersFile(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input")

	want := `# Written by sow. Don't edit it by hand.
sow_version: 1.2.3
project_name: demo
user_name: Test User
user_email: test@example.com
github_user: test-user
backend: none
`
	if got := s.read("demo/.sow-answers.yml"); got != want {
		t.Errorf(".sow-answers.yml =\n%s\nwant\n%s", got, want)
	}
}

func TestNewRejectsAnInvalidProjectName(t *testing.T) {
	names := map[string]string{
		"an uppercase letter":     "Demo",
		"a leading digit":         "1demo",
		"a trailing hyphen":       "demo-",
		"an underscore":           "my_demo",
		"more than 53 characters": strings.Repeat("a", 54),
	}
	for reason, name := range names {
		t.Run(reason, func(t *testing.T) {
			s := newSandbox(t)

			got := s.run("new", name, "--no-input")

			if got.code != 1 {
				t.Errorf("exit code = %d, want 1", got.code)
			}
			wantContains(t, got.stderr, "project_name", "lowercase letters")
			if s.exists(name) {
				t.Errorf("sow new created %s despite the invalid name", name)
			}
		})
	}
}

func TestNewAcceptsTheLongestProjectName(t *testing.T) {
	s := newSandbox(t)
	name := strings.Repeat("a", 53)

	s.mustRun("new", name, "--no-input")

	if !s.exists(name + "/README.md") {
		t.Errorf("sow new didn't create %s", name)
	}
}

func TestNewRefusesAnExistingDirectory(t *testing.T) {
	s := newSandbox(t)
	s.write(s.dir, "demo/notes.txt", "mine\n")

	got := s.run("new", "demo", "--no-input")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "demo already exists")
	if got, want := s.read("demo/notes.txt"), "mine\n"; got != want {
		t.Errorf("notes.txt = %q, want it untouched", got)
	}
}

func TestNewPersonalFlagOverridesThePersonalDefaultsFile(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input",
		"--user-name", "Ada Lovelace", "--user-email", "ada@example.com", "--github-user", "ada")

	wantContains(t, s.read("demo/.sow-answers.yml"),
		"user_name: Ada Lovelace\n", "user_email: ada@example.com\n", "github_user: ada\n")
}

func TestNewReportsEveryMissingPersonalAnswer(t *testing.T) {
	s := newSandbox(t)
	s.writePersonalDefaults("")

	got := s.run("new", "demo", "--no-input")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "--user-name", "--user-email", "--github-user")
	if s.exists("demo") {
		t.Error("sow new created demo despite the missing answers")
	}
}

func TestNewReadsPersonalDefaultsFromXDGConfigHome(t *testing.T) {
	s := newSandbox(t)
	s.writePersonalDefaults("")
	s.write(s.home, "xdg/sow/config.yml",
		"user_name: Grace Hopper\nuser_email: grace@example.com\ngithub_user: grace\n")
	s.extraEnv = []string{"XDG_CONFIG_HOME=" + s.home + "/xdg"}

	s.mustRun("new", "demo", "--no-input")

	wantContains(t, s.read("demo/.sow-answers.yml"), "user_name: Grace Hopper\n")
}

func TestNewRejectsAnUnknownKeyInThePersonalDefaultsFile(t *testing.T) {
	s := newSandbox(t)
	s.writePersonalDefaults("nickname: Test User\n")

	got := s.run("new", "demo", "--no-input")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "nickname", "config.yml")
}

func TestNewRejectsAnUnknownBackendAndListsTheChoices(t *testing.T) {
	s := newSandbox(t)

	got := s.run("new", "demo", "--no-input", "--backend", "cobol")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "backend", "cobol", "none")
	if s.exists("demo") {
		t.Error("sow new created demo despite the unknown backend")
	}
}

func TestNewMinimalPresetTurnsNoComponentsOn(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input", "--preset", "minimal")

	wantContains(t, s.read("demo/.sow-answers.yml"), "backend: none\n")
	if s.exists("demo/backend") {
		t.Error("the minimal preset rendered a backend directory")
	}
}

func TestNewRejectsAnUnknownPresetAndListsThePresets(t *testing.T) {
	s := newSandbox(t)

	got := s.run("new", "demo", "--no-input", "--preset", "everything")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "everything", "minimal")
	if s.exists("demo") {
		t.Error("sow new created demo despite the unknown preset")
	}
}

func TestVersionFlagPrintsTheVersion(t *testing.T) {
	s := newSandbox(t)

	got := s.mustRun("--version")

	wantContains(t, got.stdout, "1.2.3")
}
