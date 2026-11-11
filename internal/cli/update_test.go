package cli_test

import (
	"path/filepath"
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
