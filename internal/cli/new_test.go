package cli_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewRendersTheProjectAndCommitsIt(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input")

	if got, want := s.read("demo/README.md"), "# demo\n\n## Getting started\n\nInstall the tools:\n\n    mise install\n\nCheck the project:\n\n    mise run check\n"; got != want {
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

func TestNewRendersTheAlwaysOnFiles(t *testing.T) {
	tests := map[string]struct {
		file string
		want []string
	}{
		"AGENTS.md tells agents to use the mise tasks": {
			file: "AGENTS.md",
			want: []string{
				"Use the mise tasks in `mise.toml`",
				"Never call a tool directly when a mise task exists",
				"mise run check",
			},
		},
		".claude/settings.json declares the skills marketplace": {
			file: ".claude/settings.json",
			want: []string{
				"extraKnownMarketplaces",
				"mattpocock/skills",
				"enabledPlugins",
				"mattpocock-skills@mattpocock",
			},
		},
		"mise.toml loads the env file and gives the check task": {
			file: "mise.toml",
			want: []string{".env", "pre-commit run --all-files"},
		},
		".env.example explains what it is for": {
			file: ".env.example",
			want: []string{"# Local settings and secrets for this project"},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s := newSandbox(t)

			s.mustRun("new", "demo", "--no-input", "--preset", "minimal")

			wantContains(t, s.read("demo/"+test.file), test.want...)
		})
	}
}

func TestNewRendersClaudeSettingsAsValidJSON(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input", "--preset", "minimal")

	var settings struct {
		ExtraKnownMarketplaces map[string]struct {
			Source struct {
				Source string
				Repo   string
			}
		}
		EnabledPlugins map[string]bool
	}
	if err := json.Unmarshal([]byte(s.read("demo/.claude/settings.json")), &settings); err != nil {
		t.Fatalf("parse .claude/settings.json: %v", err)
	}
	marketplace, ok := settings.ExtraKnownMarketplaces["mattpocock"]
	if !ok {
		t.Fatal("the mattpocock marketplace is missing from extraKnownMarketplaces")
	}
	if marketplace.Source.Source != "github" {
		t.Errorf("the marketplace source is %q, want %q", marketplace.Source.Source, "github")
	}
	if marketplace.Source.Repo != "mattpocock/skills" {
		t.Errorf("the marketplace repo is %q, want %q", marketplace.Source.Repo, "mattpocock/skills")
	}
	if !settings.EnabledPlugins["mattpocock-skills@mattpocock"] {
		t.Error("the mattpocock-skills@mattpocock plugin isn't enabled")
	}
}

func TestNewDoesNotRenderDotEnv(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input", "--preset", "minimal")

	if !s.exists("demo/.env.example") {
		t.Error(".env.example doesn't exist")
	}
	if s.exists("demo/.env") {
		t.Error(".env was rendered, but the template never contains .env")
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
