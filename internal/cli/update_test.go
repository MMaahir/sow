package cli_test

import (
	"maps"
	"os"
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
			if got := s.read("demo/.sow-answers.yml"); got != tt.file {
				t.Errorf(".sow-answers.yml = %q, want it untouched", got)
			}
		})
	}
}

func TestUpdateAcceptsAnAnswersFileWithoutALaterComponent(t *testing.T) {
	s := newSandbox(t)
	s.write(s.dir, "demo/.sow-answers.yml", "sow_version: 1.2.3\nproject_name: demo\n")

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
	wantContains(t, got.stderr, "README.md", "conflict")
	wantContains(t, s.read("demo/README.md"), "<<<<<<<", "line four, mine", "line four, from sow 1.2.3")
	wantContains(t, s.read("demo/.sow-answers.yml"), "sow_version: 1.2.3\n")
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
