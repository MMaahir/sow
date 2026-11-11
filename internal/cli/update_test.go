package cli_test

import (
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
