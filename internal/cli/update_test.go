package cli_test

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateSaysAProjectAtTheCLIVersionIsAlreadyThere(t *testing.T) {
	s := newSandbox(t)
	s.mustRun("new", "demo", "--no-input")

	got := s.runIn("demo", "update")

	if got.code != 0 {
		t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	wantContains(t, got.stdout, "demo is already at sow 1.2.3")
	if got := s.git("status", "--porcelain"); got != "" {
		t.Errorf("sow update changed the project:\n%s", got)
	}
}

func TestUpdateRefusesADirectoryWithoutAnAnswersFile(t *testing.T) {
	s := newSandbox(t)
	s.write(s.dir, "demo/notes.txt", "mine\n")

	got := s.runIn("demo", "update")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, ".sow-answers.yml", "run sow update from the root of a project sow created")
	if s.exists("demo/.sow-answers.yml") {
		t.Errorf("sow update wrote .sow-answers.yml")
	}
}

func TestUpdateRejectsAnInvalidAnswersFile(t *testing.T) {
	tests := map[string]struct {
		file string
		want []string
		// absent lists text the error must not contain.
		absent []string
	}{
		"an unknown key": {
			file: "sow_version: 1.2.3\nproject_name: demo\ndatabse: postgres\n",
			want: []string{"databse is not an answer sow knows"},
		},
		"no sow_version": {
			file: "project_name: demo\n",
			want: []string{"sow_version is missing"},
		},
		"an invalid answer": {
			file: "sow_version: 1.2.3\nproject_name: demo\nbackend: rails\n",
			want: []string{`backend can't be "rails": choose one of none, ts`},
		},
		"several problems at once": {
			file: "project_name: demo\ndatabse: postgres\nbackend: rails\n",
			want: []string{
				"sow_version is missing",
				"databse is not an answer sow knows",
				`backend can't be "rails": choose one of none, ts`,
			},
		},
		"only sow_version": {
			file: "sow_version: 1.2.3\n",
			want: []string{
				"project_name is missing from",
				"user_name is missing from",
				"user_email is missing from",
				"github_user is missing from",
				"add it with the value the project was created with",
			},
		},
		"an empty answer": {
			file: "sow_version: 1.2.3\nproject_name: demo\nuser_name: \"\"\nuser_email: test@example.com\ngithub_user: test-user\n",
			want: []string{"user_name is missing from", "add it with the value the project was created with"},
			// The advice for sow new doesn't fit an existing project.
			absent: []string{"--user-name", "personal defaults file"},
		},
		"a missing answer and an unknown key": {
			file: "sow_version: 1.2.3\nproject_name: demo\nuser_name: Test User\nuser_email: test@example.com\ndatabse: postgres\n",
			want: []string{"github_user is missing from", "databse is not an answer sow knows"},
		},
		"invalid YAML": {
			file: "sow_version: 1.2.3\nproject_name: [demo\n",
			want: []string{"read the answers file", ".sow-answers.yml"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := newSandbox(t)
			s.write(s.dir, "demo/.sow-answers.yml", tt.file)

			got := s.runIn("demo", "update")

			if got.code != 1 {
				t.Errorf("exit code = %d, want 1", got.code)
			}
			wantContains(t, got.stderr, tt.want...)
			for _, text := range tt.absent {
				if strings.Contains(got.stderr, text) {
					t.Errorf("stderr contains %q:\n%s", text, got.stderr)
				}
			}
			if got := s.read("demo/.sow-answers.yml"); got != tt.file {
				t.Errorf(".sow-answers.yml = %q, want it untouched", got)
			}
		})
	}
}

func TestUpdateAcceptsAnAnswersFileWithoutALaterComponent(t *testing.T) {
	s := newSandbox(t)
	s.write(s.dir, "demo/.sow-answers.yml", "sow_version: 1.2.3\nproject_name: demo\n"+
		"user_name: Test User\nuser_email: test@example.com\ngithub_user: test-user\n")

	got := s.runIn("demo", "update")

	if got.code != 0 {
		t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	wantContains(t, got.stdout, "demo is already at sow 1.2.3")
}

func TestUpdateHelpDescribesTheCommand(t *testing.T) {
	s := newSandbox(t)

	got := s.mustRun("update", "--help")

	wantContains(t, got.stdout, "Update the project in this directory to the template of this sow version.")
}

func TestUpdateRefusesAVersionItCantRender(t *testing.T) {
	tests := map[string]struct {
		recorded string
		cli      string
		want     []string
	}{
		"a development build": {
			recorded: "0.0.0-dev",
			cli:      "1.2.3",
			want:     []string{"0.0.0-dev", "a development build"},
		},
		"a newer version": {
			recorded: "2.0.0",
			cli:      "1.2.3",
			want:     []string{"2.0.0", "newer than this sow", "upgrade sow"},
		},
		"a newer version compared as numbers": {
			recorded: "1.10.0",
			cli:      "1.9.0",
			want:     []string{"1.10.0", "newer than this sow", "upgrade sow"},
		},
		"a released project with a development build of sow": {
			recorded: "1.0.0",
			cli:      "0.0.0-dev",
			want: []string{
				"this sow is a development build",
				"a release build of sow at 1.0.0 or later",
				"go install github.com/MMaahir/sow/cmd/sow@v1.0.0",
			},
		},
		"a word": {
			recorded: "latest",
			cli:      "1.2.3",
			want:     []string{"latest", "isn't a sow release"},
		},
		"two numbers": {
			recorded: "0.1",
			cli:      "1.2.3",
			want:     []string{"0.1", "isn't a sow release"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := newSandbox(t)
			s.projectAt(tt.recorded)
			answersFile := s.read("demo/.sow-answers.yml")
			s.version = tt.cli
			// SOW_REPO points nowhere and git isn't on the PATH, so the
			// message shows sow refused before it fetched anything.
			s.extraEnv = append(s.extraEnv, "SOW_REPO="+filepath.Join(s.dir, "no-such-repo"))
			s.isolatePath()

			got := s.runIn("demo", "update")

			if got.code != 1 {
				t.Errorf("exit code = %d, want 1", got.code)
			}
			wantContains(t, got.stderr, tt.want...)
			if got := s.read("demo/.sow-answers.yml"); got != answersFile {
				t.Errorf(".sow-answers.yml = %q, want it untouched", got)
			}
			if got := s.git("status", "--porcelain"); got != "" {
				t.Errorf("sow update changed the project:\n%s", got)
			}
		})
	}
}

// templateAt100 is the template of the fixture release 1.0.0.
var templateAt100 = map[string]string{
	"base/README.md.tmpl": "# [[ .project_name ]]\n\nline one\nline two\nline three\nline four\nline five\n",
	"base/LICENSE":        "MIT\n",
}

// templateAt123 changes one line of README.md and adds CHANGELOG.md.
var templateAt123 = map[string]string{
	"base/README.md.tmpl": "# [[ .project_name ]]\n\nline one\nline two\nline three\nline four, from sow 1.2.3\nline five\n",
	"base/LICENSE":        "MIT\n",
	"base/CHANGELOG.md":   "# Changelog\n",
}

func TestUpdateBringsAnUneditedProjectToTheNewTemplate(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)
	if got := s.runIn("fresh", "new", "demo", "--no-input"); got.code != 0 {
		t.Fatalf("sow new in fresh exited %d:\n%s", got.code, got.stderr)
	}

	got := s.runIn("demo", "update")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	wantContains(t, s.read("demo/README.md"), "line four, from sow 1.2.3\n")
	if got, want := s.read("demo/CHANGELOG.md"), "# Changelog\n"; got != want {
		t.Errorf("CHANGELOG.md = %q, want %q", got, want)
	}
	if got, want := s.files("demo"), s.files("fresh/demo"); !maps.Equal(got, want) {
		t.Errorf("updated project = %q\nwant what sow new renders: %q", got, want)
	}
}

func TestUpdateRecordsTheCLIVersionInTheAnswersFile(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)

	s.runIn("demo", "update")

	wantContains(t, s.read("demo/.sow-answers.yml"), "sow_version: 1.2.3\n")
}

func TestUpdateEndsByTellingTheAuthorToReviewTheDiff(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)

	got := s.runIn("demo", "update")

	lines := strings.Split(strings.TrimRight(got.stdout, "\n"), "\n")
	wantContains(t, lines[len(lines)-1], "1.0.0", "1.2.3", "git diff")
}

func TestUpdateKeepsAnEditAndAddsTheTemplateChangeToTheSameFile(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)
	s.edit("demo/README.md", func(readme string) string {
		return strings.Replace(readme, "line one\n", "line one, mine\n", 1)
	})
	s.commit()

	got := s.runIn("demo", "update")

	if got.code != 0 {
		t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	want := "# demo\n\nline one, mine\nline two\nline three\nline four, from sow 1.2.3\nline five\n"
	if got := s.read("demo/README.md"); got != want {
		t.Errorf("README.md = %q, want %q", got, want)
	}
}

func TestUpdateLeavesAFileTheProjectAddedUnchanged(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)
	s.write(s.dir, "demo/notes.txt", "my notes\nno trailing newline")
	s.commit()

	s.runIn("demo", "update")

	if got, want := s.read("demo/notes.txt"), "my notes\nno trailing newline"; got != want {
		t.Errorf("notes.txt = %q, want %q", got, want)
	}
}

func TestUpdateDeletesAnUneditedFileTheNewTemplateDroppedAndKeepsAnEditedOne(t *testing.T) {
	s := newSandbox(t)
	withOldFiles := maps.Clone(templateAt100)
	withOldFiles["base/docs/unedited.md"] = "from the template\n"
	withOldFiles["base/edited.md"] = "from the template\n"
	s.releasedProject("1.0.0", withOldFiles, templateAt123)
	s.write(s.dir, "demo/edited.md", "from the template\nand from me\n")
	s.commit()

	got := s.runIn("demo", "update")

	if got.code != 0 {
		t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	if s.exists("demo/docs/unedited.md") {
		t.Errorf("docs/unedited.md still exists, want it deleted")
	}
	if s.exists("demo/docs") {
		t.Errorf("docs still exists, want the empty directory deleted")
	}
	if got, want := s.read("demo/edited.md"), "from the template\nand from me\n"; got != want {
		t.Errorf("edited.md = %q, want the author's edit kept", got)
	}
}

func TestUpdateMarksAClashBetweenAnEditAndATemplateChange(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)
	s.edit("demo/README.md", func(readme string) string {
		return strings.Replace(readme, "line four\n", "line four, mine\n", 1)
	})
	s.commit()

	got := s.runIn("demo", "update")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "README.md", "conflict", "git reset --hard && git clean -fd")
	wantContains(t, s.read("demo/README.md"), "<<<<<<<", "line four, mine", "line four, from sow 1.2.3")
	wantContains(t, s.read("demo/.sow-answers.yml"), "sow_version: 1.2.3\n")
}

func TestUpdateWithAConflictStillUpdatesTheOtherFiles(t *testing.T) {
	s := newSandbox(t)
	newTemplate := maps.Clone(templateAt123)
	newTemplate["base/LICENSE"] = "MIT, from sow 1.2.3\n"
	s.releasedProject("1.0.0", templateAt100, newTemplate)
	s.edit("demo/README.md", func(readme string) string {
		return strings.Replace(readme, "line four\n", "line four, mine\n", 1)
	})
	s.commit()

	got := s.runIn("demo", "update")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	if got, want := s.read("demo/LICENSE"), "MIT, from sow 1.2.3\n"; got != want {
		t.Errorf("LICENSE = %q, want %q", got, want)
	}
	wantContains(t, s.read("demo/.sow-answers.yml"), "sow_version: 1.2.3\n")
}

func TestUpdateRefusesAProjectWithLocalChanges(t *testing.T) {
	tests := map[string]func(s *sandbox){
		"an uncommitted change": func(s *sandbox) {
			s.edit("demo/README.md", func(readme string) string { return readme + "mine\n" })
		},
		"an untracked file": func(s *sandbox) {
			s.write(s.dir, "demo/notes.txt", "mine\n")
		},
		"an untracked file while git status hides untracked files": func(s *sandbox) {
			s.git("config", "status.showUntrackedFiles", "no")
			s.write(s.dir, "demo/notes.txt", "mine\n")
		},
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			s := newSandbox(t)
			s.releasedProject("1.0.0", templateAt100, templateAt123)
			change(s)
			before := s.files("demo")

			got := s.runIn("demo", "update")

			if got.code != 1 {
				t.Errorf("exit code = %d, want 1", got.code)
			}
			wantContains(t, got.stderr, "uncommitted", "commit")
			if after := s.files("demo"); !maps.Equal(after, before) {
				t.Errorf("project = %q\nwant it unchanged: %q", after, before)
			}
		})
	}
}

func TestUpdateRefusesAProjectThatIsNotAGitRepo(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)
	if err := os.RemoveAll(filepath.Join(s.dir, "demo", ".git")); err != nil {
		t.Fatal(err)
	}
	before := s.files("demo")

	got := s.runIn("demo", "update")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "git repo", "git init")
	if after := s.files("demo"); !maps.Equal(after, before) {
		t.Errorf("project = %q\nwant it unchanged: %q", after, before)
	}
}

func TestUpdateKeepsAFileTheProjectDeletedDeleted(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)
	s.git("rm", "--quiet", "README.md")
	s.commit()

	got := s.runIn("demo", "update")

	if got.code != 0 {
		t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	if s.exists("demo/README.md") {
		t.Errorf("README.md exists, want it to stay deleted")
	}
}

func TestUpdateLeavesDotEnvUnchanged(t *testing.T) {
	tests := map[string]struct {
		editedLine string
		wantCode   int
	}{
		"after a successful update": {editedLine: "line one", wantCode: 0},
		"after a conflicted update": {editedLine: "line four", wantCode: 1},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := newSandbox(t)
			s.releasedProject("1.0.0", withEnvExample(templateAt100), withEnvExample(templateAt123))
			s.edit("demo/README.md", func(readme string) string {
				return strings.Replace(readme, tt.editedLine+"\n", tt.editedLine+", mine\n", 1)
			})
			s.commit()
			s.write(s.dir, "demo/.env", "PORT=4000\nSECRET=mine")

			got := s.runIn("demo", "update")

			if got.code != tt.wantCode {
				t.Errorf("exit code = %d, want %d\n%s", got.code, tt.wantCode, got.stderr)
			}
			if got, want := s.read("demo/.env"), "PORT=4000\nSECRET=mine"; got != want {
				t.Errorf(".env = %q, want %q", got, want)
			}
		})
	}
}

func TestUpdateDoesNotCreateDotEnv(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", withEnvExample(templateAt100), withEnvExample(templateAt123))
	if err := os.Remove(filepath.Join(s.dir, "demo", ".env")); err != nil {
		t.Fatal(err)
	}

	got := s.runIn("demo", "update")

	if got.code != 0 {
		t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	if s.exists("demo/.env") {
		t.Errorf("sow update created .env")
	}
}

func TestUpdateRunsVPInstallWhenTheNewTemplateHasAWorkspace(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, withWorkspace(templateAt123))
	s.fakeTool("vp", `echo "$*" >> vp-calls`)

	got := s.runIn("demo", "update")

	if got.code != 0 {
		t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	if got, want := s.read("demo/vp-calls"), "install\n"; got != want {
		t.Errorf("vp calls in the project root = %q, want %q", got, want)
	}
}

func TestUpdateWithAConflictSkipsTheFollowUpSteps(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, withWorkspace(templateAt123))
	s.edit("demo/README.md", func(readme string) string {
		return strings.Replace(readme, "line four\n", "line four, mine\n", 1)
	})
	s.commit()
	s.fakeTool("vp", `echo "$*" >> vp-calls`)

	got := s.runIn("demo", "update")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "follow-up steps", "vp install")
	if s.exists("demo/vp-calls") {
		t.Errorf("vp ran despite the conflict: %q", s.read("demo/vp-calls"))
	}
}

func TestUpdateReportsAFailedFollowUpStepAndKeepsTheMergedFiles(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, withWorkspace(templateAt123))
	s.fakeTool("vp", "echo registry unreachable >&2; exit 1")

	got := s.runIn("demo", "update")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "vp install", filepath.Join(s.dir, "demo"), "registry unreachable")
	wantContains(t, s.read("demo/README.md"), "line four, from sow 1.2.3\n")
	if !s.exists("demo/pnpm-workspace.yaml") {
		t.Errorf("pnpm-workspace.yaml is gone after the failed follow-up step")
	}
}

func TestUpdateLeavesTheChangesUncommitted(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)
	head := s.git("rev-parse", "HEAD")

	got := s.runIn("demo", "update")

	if got.code != 0 {
		t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	if got := s.git("rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD = %s, want %s", got, head)
	}
	want := "M .sow-answers.yml\n M README.md\n?? CHANGELOG.md"
	if got := s.git("status", "--porcelain"); got != want {
		t.Errorf("git status --porcelain = %q, want %q", got, want)
	}
}

func TestUpdateWithAConflictCanBeUndoneWithGit(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)
	s.edit("demo/README.md", func(readme string) string {
		return strings.Replace(readme, "line four\n", "line four, mine\n", 1)
	})
	s.commit()
	before := s.files("demo")
	if got := s.runIn("demo", "update"); got.code != 1 {
		t.Fatalf("sow update exited %d, want a conflict:\n%s", got.code, got.stderr)
	}

	s.git("reset", "--quiet", "--hard")
	s.git("clean", "-fd")

	if after := s.files("demo"); !maps.Equal(after, before) {
		t.Errorf("project = %q\nwant the previous commit: %q", after, before)
	}
}

func TestUpdateRendersATemplateThatReadsAComponentTheAnswersFileLacks(t *testing.T) {
	s := newSandbox(t)
	readsBackend := maps.Clone(templateAt100)
	readsBackend["base/backend.md.tmpl"] = "backend: [[ .backend ]]\n"
	newReadsBackend := maps.Clone(templateAt123)
	newReadsBackend["base/backend.md.tmpl"] = "backend: [[ .backend ]]\n"
	s.releasedProject("1.0.0", readsBackend, newReadsBackend)
	s.edit("demo/.sow-answers.yml", func(file string) string {
		return strings.Replace(file, "backend: none\n", "", 1)
	})
	s.commit()

	got := s.runIn("demo", "update")

	if got.code != 0 {
		t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	if got, want := s.read("demo/backend.md"), "backend: none\n"; got != want {
		t.Errorf("backend.md = %q, want %q", got, want)
	}
	wantContains(t, s.read("demo/README.md"), "line four, from sow 1.2.3\n")
}

func TestUpdateUsesTheCachedTemplateWhenTheRepoIsGone(t *testing.T) {
	s := newSandbox(t)
	s.release("1.0.0", templateAt100)
	s.install("1.0.0", templateAt100)
	s.mustRun("new", "demo", "--no-input")
	s.mustRun("new", "other", "--no-input")
	s.install(testVersion, templateAt123)
	if got := s.runIn("demo", "update"); got.code != 0 {
		t.Fatalf("first update exited %d:\n%s", got.code, got.stderr)
	}
	if err := os.RemoveAll(s.repo); err != nil {
		t.Fatal(err)
	}

	got := s.runIn("other", "update")

	if got.code != 0 {
		t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	wantContains(t, s.read("other/README.md"), "line four, from sow 1.2.3\n")
}

func TestUpdateCachesTheOldTemplate(t *testing.T) {
	tests := map[string]struct {
		xdgCacheHome bool
		want         string
	}{
		"under XDG_CACHE_HOME when it is set": {xdgCacheHome: true, want: "xdg-cache/sow"},
		"under ~/.cache otherwise":            {want: ".cache/sow"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := newSandbox(t)
			if tt.xdgCacheHome {
				s.extraEnv = append(s.extraEnv, "XDG_CACHE_HOME="+filepath.Join(s.home, "xdg-cache"))
			}
			s.releasedProject("1.0.0", templateAt100, templateAt123)

			got := s.runIn("demo", "update")

			if got.code != 0 {
				t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
			}
			cached := filepath.Join(s.home, filepath.FromSlash(tt.want), "templates", "v1.0.0", "base", "README.md.tmpl")
			if _, err := os.Stat(cached); err != nil {
				t.Errorf("the old template isn't cached: %v", err)
			}
		})
	}
}

func TestUpdateRefusesARepoWithoutTheRecordedTag(t *testing.T) {
	s := newSandbox(t)
	s.release("0.9.0", templateAt100)
	s.install("1.0.0", templateAt100)
	s.mustRun("new", "demo", "--no-input")
	s.install(testVersion, templateAt123)

	got := s.runIn("demo", "update")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "v1.0.0", "SOW_REPO")
	if got := s.git("status", "--porcelain"); got != "" {
		t.Errorf("sow update changed the project:\n%s", got)
	}
}

func TestUpdateKeepsAnAnswersFileWithoutALaterComponentReadable(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)
	s.edit("demo/.sow-answers.yml", func(file string) string {
		return strings.Replace(file, "backend: none\n", "", 1)
	})
	s.commit()

	s.runIn("demo", "update")
	got := s.runIn("demo", "update")

	if got.code != 0 {
		t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	wantContains(t, got.stdout, "demo is already at sow 1.2.3")
	if file := s.read("demo/.sow-answers.yml"); strings.Contains(file, "backend") {
		t.Errorf(".sow-answers.yml = %q, want no backend entry", file)
	}
}

func TestUpdateReadsARelativeSOWRepoFromTheProjectDirectory(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)
	relative, err := filepath.Rel(filepath.Join(s.dir, "demo"), s.repo)
	if err != nil {
		t.Fatal(err)
	}
	s.extraEnv = append(s.extraEnv, "SOW_REPO="+relative)

	got := s.runIn("demo", "update")

	if got.code != 0 {
		t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	wantContains(t, s.read("demo/README.md"), "line four, from sow 1.2.3\n")
}

func TestUpdateIgnoresAGitWarningWhenItChecksForLocalChanges(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)
	s.isolatePath()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	s.fakeTool("git", `if [ "$1" = status ]; then echo "warning: unable to access attributes" >&2; fi
exec "`+realGit+`" "$@"`)

	got := s.runIn("demo", "update")

	if got.code != 0 {
		t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	wantContains(t, s.read("demo/README.md"), "line four, from sow 1.2.3\n")
}

func TestUpdateReadsTheOldTemplateWithoutTheAuthorsCheckoutFilters(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)
	s.edit("demo/README.md", func(readme string) string {
		return strings.Replace(readme, "line one\n", "line one, mine\n", 1)
	})
	s.commit()
	// A smudge filter that turns the old template's README into the author's
	// edit. If sow checked the release out, the edit would look unedited.
	s.write(s.home, "attributes", "*.tmpl filter=mine\n")
	s.write(s.home, "gitconfig", "[core]\n\tattributesFile = "+filepath.Join(s.home, "attributes")+
		"\n[filter \"mine\"]\n\tsmudge = sed 's/line one$/line one, mine/'\n\tclean = cat\n")

	got := s.runIn("demo", "update")

	if got.code != 0 {
		t.Errorf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	want := "# demo\n\nline one, mine\nline two\nline three\nline four, from sow 1.2.3\nline five\n"
	if got := s.read("demo/README.md"); got != want {
		t.Errorf("README.md = %q, want %q", got, want)
	}
}

func TestUpdateRefusesABranchNamedLikeTheRecordedTag(t *testing.T) {
	s := newSandbox(t)
	s.releasedProject("1.0.0", templateAt100, templateAt123)
	s.gitIn(s.repo, "tag", "--delete", "v1.0.0")
	s.gitIn(s.repo, "branch", "v1.0.0")

	got := s.runIn("demo", "update")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "v1.0.0", "SOW_REPO")
	if got := s.git("status", "--porcelain"); got != "" {
		t.Errorf("sow update changed the project:\n%s", got)
	}
}

func TestUpdateRefusesToWriteThroughASymlink(t *testing.T) {
	tests := map[string]struct {
		link   string
		target string
		want   string
	}{
		"a file":             {link: "README.md", target: "outside/README.md", want: "README.md is a symbolic link"},
		"a parent directory": {link: "docs", target: "outside", want: "docs is a symbolic link"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := newSandbox(t)
			withDocs := maps.Clone(templateAt100)
			withDocs["base/docs/guide.md"] = "guide\n"
			newWithDocs := maps.Clone(templateAt123)
			newWithDocs["base/docs/guide.md"] = "guide, from sow 1.2.3\n"
			s.releasedProject("1.0.0", withDocs, newWithDocs)
			outside := map[string]string{"README.md": s.read("demo/README.md"), "guide.md": "guide\n"}
			for name, content := range outside {
				s.write(s.home, "outside/"+name, content)
			}
			if err := os.RemoveAll(filepath.Join(s.dir, "demo", tt.link)); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(s.home, tt.target), filepath.Join(s.dir, "demo", tt.link)); err != nil {
				t.Fatal(err)
			}
			s.commit()
			before := s.files("demo")

			got := s.runIn("demo", "update")

			if got.code != 1 {
				t.Errorf("exit code = %d, want 1", got.code)
			}
			wantContains(t, got.stderr, tt.want, "replace the link")
			if got, want := s.files("demo"), before; !maps.Equal(got, want) {
				t.Errorf("project = %q\nwant it unchanged: %q", got, want)
			}
			if got := s.git("status", "--porcelain"); got != "" {
				t.Errorf("sow update changed the project:\n%s", got)
			}
			for name, want := range outside {
				content, err := os.ReadFile(filepath.Join(s.home, "outside", name))
				if err != nil {
					t.Fatal(err)
				}
				if string(content) != want {
					t.Errorf("outside/%s = %q, want it untouched: %q", name, content, want)
				}
			}
		})
	}
}
