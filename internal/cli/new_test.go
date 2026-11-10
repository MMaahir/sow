package cli_test

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRendersTheProjectAndCommitsIt(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input")

	if got, want := s.read("demo/README.md"), "# demo\n\n## Getting started\n\nInstall the tools:\n\n    mise install\n\nCheck the project:\n\n    mise run check\n"; got != want {
		t.Errorf("README.md = %q, want %q", got, want)
	}
	if got, want := s.git("log", "--format=%s"), "chore: initial commit from sow"; got != want {
		t.Errorf("commit subjects = %q, want %q", got, want)
	}
	if got, want := s.git("branch", "--show-current"), "main"; got != want {
		t.Errorf("branch = %q, want %q", got, want)
	}
	if got := s.git("status", "--porcelain"); got != "" {
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
		".pre-commit-config.yaml pins the hooks to a release tag": {
			file: ".pre-commit-config.yaml",
			want: []string{
				"https://github.com/pre-commit/pre-commit-hooks",
				"rev: v6.0.0",
				"end-of-file-fixer",
				"trailing-whitespace",
				"check-yaml",
				"check-merge-conflict",
			},
		},
		".editorconfig keeps every file clean": {
			file: ".editorconfig",
			want: []string{
				"root = true",
				"charset = utf-8",
				"end_of_line = lf",
				"insert_final_newline = true",
				"trim_trailing_whitespace = true",
				"indent_style = space",
				"indent_size = 2",
			},
		},
		".gitignore ignores the secrets and the machine's files": {
			file: ".gitignore",
			want: []string{".DS_Store", ".env"},
		},
		"LICENSE is the MIT licence with the personal name as the holder": {
			file: "LICENSE",
			want: []string{
				"Copyright (c) 2026 Test User",
				"Permission is hereby granted, free of charge",
			},
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

func TestNewIgnoresDotEnv(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input", "--preset", "minimal")

	if !s.exists("demo/.env.example") {
		t.Error(".env.example doesn't exist")
	}
	if got := s.git("check-ignore", ".env"); got != ".env" {
		t.Errorf("git check-ignore .env = %q, want it ignored", got)
	}
}

func TestNewCopiesDotEnvExampleToDotEnv(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input")

	want := "# Local settings and secrets for this project. Copy it to .env and fill it in. mise loads .env.\n"
	if got := s.read("demo/.env"); got != want {
		t.Errorf(".env = %q, want %q", got, want)
	}
}

func TestNewLeavesDotEnvOutOfTheFirstCommit(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input")

	files := s.git("ls-tree", "--name-only", "-r", "HEAD")
	for _, name := range strings.Split(files, "\n") {
		if name == ".env" {
			t.Error(".env is in the first commit, but .gitignore ignores it")
		}
	}
	if !s.exists("demo/.env") {
		t.Error(".env doesn't exist next to the first commit")
	}
	if got := s.git("status", "--porcelain"); got != "" {
		t.Errorf("uncommitted files after sow new:\n%s", got)
	}
}

func TestNewRendersTheLicenceWithTheNameOfPersonalDefaults(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input", "--user-name", "Ada Lovelace")

	wantContains(t, s.read("demo/LICENSE"), "Copyright (c) 2026 Ada Lovelace")
}

func TestNewCommitsTheAlwaysOnFilesInTheFirstCommit(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input", "--preset", "minimal")

	files := s.git("ls-tree", "--name-only", "-r", "HEAD")
	for _, name := range []string{
		".claude/settings.json",
		".editorconfig",
		".env.example",
		".gitignore",
		".pre-commit-config.yaml",
		"AGENTS.md",
		"LICENSE",
		"README.md",
		"mise.toml",
	} {
		if !strings.Contains(files, name) {
			t.Errorf("%s isn't in the first commit", name)
		}
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

func TestNewReportsEveryMissingPersonalAnswerWithoutAPersonalDefaultsFile(t *testing.T) {
	s := newSandbox(t)
	s.removePersonalDefaults()

	got := s.run("new", "demo", "--no-input")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "--user-name", "--user-email", "--github-user")
	if s.exists("demo") {
		t.Error("sow new created demo despite the missing answers")
	}
}

func TestNewReportsProblemsFromEverySourceInOneRun(t *testing.T) {
	s := newSandbox(t)
	s.writePersonalDefaults("nickname: Tester\nshoe_size: 9\n")

	got := s.run("new", "Demo", "--no-input", "--preset", "everything", "--backend", "cobol")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr,
		`there is no preset "everything"`,
		"sets nickname",
		"sets shoe_size",
		`project_name can't be "Demo"`,
		`backend can't be "cobol"`,
		"--user-name")
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

func TestNewMinimalProjectHealthPreCommitRunsClean(t *testing.T) {
	if testing.Short() {
		t.Skip("generated project health tests are slow: run mise run test-health")
	}
	if _, err := exec.LookPath("pre-commit"); err != nil {
		t.Skip("pre-commit isn't installed: install it to run this test")
	}
	s := newSandbox(t)

	s.mustRun("new", "demo", "--no-input", "--preset", "minimal")

	cmd := exec.CommandContext(s.t.Context(), "pre-commit", "run", "--all-files")
	cmd.Dir = filepath.Join(s.dir, "demo")
	cmd.Env = s.environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pre-commit run --all-files failed in %s: %v\n%s", cmd.Dir, err, output)
	}
}
