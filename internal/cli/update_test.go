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
