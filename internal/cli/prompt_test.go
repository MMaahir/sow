package cli_test

import (
	"slices"
	"testing"
)

func TestNewAsksForAMissingAnswerWithItsHelp(t *testing.T) {
	s := newSandbox(t)
	s.writePersonalDefaults("user_email: test@example.com\ngithub_user: test-user\n")
	prompter := s.terminal(map[string]string{"user_name": "Ada Lovelace", "backend": "none"})

	s.mustRun("new", "demo")

	if len(prompter.asked) == 0 || prompter.asked[0].Key != "user_name" {
		t.Fatalf("asked %v, want user_name first", prompter.askedKeys())
	}
	if got, want := prompter.asked[0].Help, "Your name, used as the project's author."; got != want {
		t.Errorf("user_name prompt = %q, want %q", got, want)
	}
	wantContains(t, s.read("demo/.sow-answers.yml"), "user_name: Ada Lovelace\n")
}

func TestNewDoesntAskForAGivenAnswer(t *testing.T) {
	cases := map[string]struct {
		personal string
		args     []string
		want     []string
	}{
		"flags": {
			personal: "",
			args: []string{
				"--user-name", "Ada Lovelace", "--user-email", "ada@example.com",
				"--github-user", "ada", "--backend", "none",
			},
			want: []string{},
		},
		"a preset": {
			personal: "user_name: Test User\nuser_email: test@example.com\ngithub_user: test-user\n",
			args:     []string{"--preset", "minimal"},
			want:     []string{},
		},
		"the personal defaults file": {
			personal: "user_name: Test User\nuser_email: test@example.com\ngithub_user: test-user\n",
			args:     []string{},
			want:     []string{"backend"},
		},
	}
	for source, c := range cases {
		t.Run(source, func(t *testing.T) {
			s := newSandbox(t)
			s.writePersonalDefaults(c.personal)
			prompter := s.terminal(map[string]string{"backend": "none"})

			s.mustRun(append([]string{"new", "demo"}, c.args...)...)

			if got := prompter.askedKeys(); !slices.Equal(got, c.want) {
				t.Errorf("asked %v, want %v", got, c.want)
			}
		})
	}
}

func TestNewAsksForTheBackendAsAChoiceWithNonePreselected(t *testing.T) {
	s := newSandbox(t)
	prompter := s.terminal(map[string]string{"backend": "none"})

	s.mustRun("new", "demo")

	if got, want := prompter.askedKeys(), []string{"backend"}; !slices.Equal(got, want) {
		t.Fatalf("asked %v, want %v", got, want)
	}
	if got := prompter.asked[0].Choices; !slices.Contains(got, "none") {
		t.Errorf("backend choices = %v, want them to include none", got)
	}
	if got, want := prompter.asked[0].Default, "none"; got != want {
		t.Errorf("backend preselected = %q, want %q", got, want)
	}
	wantContains(t, s.read("demo/.sow-answers.yml"), "backend: none\n")
}

func TestNewRejectsAnInvalidTypedAnswer(t *testing.T) {
	s := newSandbox(t)
	s.terminal(map[string]string{"backend": "cobol", "frontend": "none"})

	got := s.run("new", "demo")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "backend", "cobol", "none")
	if s.exists("demo") {
		t.Error("sow new created demo despite the invalid answer")
	}
}

func TestNewRejectsAnEmptyGivenAnswerBeforeAsking(t *testing.T) {
	s := newSandbox(t)
	prompter := s.terminal(map[string]string{"backend": "none"})

	got := s.run("new", "demo", "--user-name", "")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr,
		"user_name can't be empty: pass --user-name or set user_name in the personal defaults file")
	if asked := prompter.askedKeys(); len(asked) != 0 {
		t.Errorf("sow asked for %v despite the empty user_name", asked)
	}
	if s.exists("demo") {
		t.Error("sow new created demo despite the empty user_name")
	}
}

func TestNewWithEveryAnswerGivenNeedsNoTerminal(t *testing.T) {
	s := newSandbox(t)

	s.mustRun("new", "demo", "--backend", "none")

	if !s.exists("demo/README.md") {
		t.Error("sow new didn't create demo")
	}
}

func TestNewRefusesToAskWithoutATerminal(t *testing.T) {
	s := newSandbox(t)

	got := s.run("new", "demo")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	want := "sow: stdin isn't a terminal, so sow can't ask for backend: " +
		"pass --no-input and give answers with flags, a preset or the personal defaults file\n"
	if got.stderr != want {
		t.Errorf("stderr = %q, want %q", got.stderr, want)
	}
	if s.exists("demo") {
		t.Error("sow new created demo without a terminal or --no-input")
	}
}

func TestNewWithNoInputNeverAsks(t *testing.T) {
	s := newSandbox(t)
	s.writePersonalDefaults("")
	prompter := s.terminal(map[string]string{
		"user_name": "Ada Lovelace", "user_email": "ada@example.com", "github_user": "ada", "backend": "none",
	})

	got := s.run("new", "demo", "--no-input")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "user_name is missing", "--user-name")
	if asked := prompter.askedKeys(); len(asked) != 0 {
		t.Errorf("asked %v with --no-input, want nothing", asked)
	}
	if s.exists("demo") {
		t.Error("sow new created demo despite the missing answers")
	}
}

func TestNewRejectsAnInvalidNameBeforeAsking(t *testing.T) {
	s := newSandbox(t)
	prompter := s.terminal(map[string]string{"backend": "none"})

	got := s.run("new", "Demo")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, `project_name can't be "Demo"`, "lowercase letters")
	if asked := prompter.askedKeys(); len(asked) != 0 {
		t.Errorf("asked %v, want nothing", asked)
	}
	if s.exists("Demo") {
		t.Error("sow new created Demo despite the invalid name")
	}
}

func TestNewRejectsAnExistingDirectoryBeforeAsking(t *testing.T) {
	s := newSandbox(t)
	s.write(s.dir, "demo/notes.txt", "mine\n")
	prompter := s.terminal(map[string]string{"backend": "none"})

	got := s.run("new", "demo")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "demo already exists")
	if asked := prompter.askedKeys(); len(asked) != 0 {
		t.Errorf("asked %v, want nothing", asked)
	}
	if got, want := s.read("demo/notes.txt"), "mine\n"; got != want {
		t.Errorf("notes.txt = %q, want it untouched", got)
	}
}

func TestNewRejectsAnInvalidGivenBackendBeforeAsking(t *testing.T) {
	s := newSandbox(t)
	s.removePersonalDefaults()
	prompter := s.terminal(map[string]string{"user_name": "Ada Lovelace"})

	got := s.run("new", "demo", "--backend", "cobol")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, `backend can't be "cobol"`, "none, ts")
	if asked := prompter.askedKeys(); len(asked) != 0 {
		t.Errorf("asked %v, want nothing", asked)
	}
	if s.exists("demo") {
		t.Error("sow new created demo despite the invalid backend")
	}
}

func TestNewRejectsAFrontendWithoutABackendBeforeAsking(t *testing.T) {
	s := newSandbox(t)
	s.removePersonalDefaults()
	prompter := s.terminal(map[string]string{"user_name": "Ada Lovelace"})

	got := s.run("new", "demo", "--backend", "none", "--frontend", "react")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "frontend needs a backend", "--backend ts")
	if asked := prompter.askedKeys(); len(asked) != 0 {
		t.Errorf("asked %v, want nothing", asked)
	}
	if s.exists("demo") {
		t.Error("sow new created demo despite the frontend with no backend")
	}
}

func TestNewRejectsAFrontendWithoutABackendWithoutATerminal(t *testing.T) {
	s := newSandbox(t)
	s.removePersonalDefaults()

	got := s.run("new", "demo", "--backend", "none", "--frontend", "react")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "frontend needs a backend", "--backend ts")
}

func TestNewCancelledPromptCreatesNothing(t *testing.T) {
	s := newSandbox(t)
	s.terminal(map[string]string{})

	got := s.run("new", "demo")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "backend", "user aborted")
	if s.exists("demo") {
		t.Error("sow new created demo after the prompt was cancelled")
	}
}

func TestNewAsksForTheFrontendAfterABackend(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("vp", fakeVP)
	prompter := s.terminal(map[string]string{"backend": "ts", "frontend": "react"})

	s.mustRun("new", "demo")

	if got, want := prompter.askedKeys(), []string{"backend", "frontend"}; !slices.Equal(got, want) {
		t.Fatalf("asked %v, want %v", got, want)
	}
	if got, want := prompter.asked[1].Choices, []string{"none", "react"}; !slices.Equal(got, want) {
		t.Errorf("frontend choices = %v, want %v", got, want)
	}
	if got, want := prompter.asked[1].Default, "none"; got != want {
		t.Errorf("frontend preselected = %q, want %q", got, want)
	}
	wantContains(t, s.read("demo/.sow-answers.yml"), "backend: ts\nfrontend: react\n")
}

func TestNewDoesntAskForAFrontendWithoutABackend(t *testing.T) {
	cases := map[string]struct {
		args []string
		want []string
	}{
		"the author types none for the backend": {args: []string{}, want: []string{"backend"}},
		"a flag turns the backend off":          {args: []string{"--backend", "none"}, want: []string{}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s := newSandbox(t)
			prompter := s.terminal(map[string]string{"backend": "none"})

			s.mustRun(append([]string{"new", "demo"}, c.args...)...)

			if got := prompter.askedKeys(); !slices.Equal(got, c.want) {
				t.Errorf("asked %v, want %v", got, c.want)
			}
			wantContains(t, s.read("demo/.sow-answers.yml"), "frontend: none\n")
		})
	}
}
