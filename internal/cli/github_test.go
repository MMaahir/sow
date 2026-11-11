package cli_test

import (
	"slices"
	"strings"
	"testing"
)

func TestNewGitHubCreatesAndProtectsTheRepoInOrder(t *testing.T) {
	s := newSandbox(t)
	s.fakeGH("")

	s.mustRun("new", "demo", "--no-input", "--github")

	want := []string{
		"before push: auth status --hostname github.com",
		"before push: repo create test-user/demo --public",
		"before push: api --method PATCH repos/test-user/demo" +
			" -F allow_squash_merge=true -F allow_merge_commit=false -F allow_rebase_merge=false" +
			" -f squash_merge_commit_title=PR_TITLE -f squash_merge_commit_message=PR_BODY",
		"before push: api --method PATCH repos/test-user/demo" +
			" -f security_and_analysis[secret_scanning][status]=enabled" +
			" -f security_and_analysis[secret_scanning_push_protection][status]=enabled",
		"after push: api --method PUT repos/test-user/demo/branches/main/protection" +
			" -F required_status_checks[strict]=true -f required_status_checks[checks][][context]=ci-ok" +
			" -F enforce_admins=false" +
			" -F required_pull_request_reviews[required_approving_review_count]=0" +
			" -F restrictions=null",
	}
	if got := s.ghCalls(); !slices.Equal(got, want) {
		t.Errorf("gh calls:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestNewGitHubAllowsOnlySquashMergesWithThePRTitle(t *testing.T) {
	s := newSandbox(t)
	s.fakeGH("")

	s.mustRun("new", "demo", "--no-input", "--github")

	call := ghCall(t, s.ghCalls(), "allow_squash_merge")
	wantContains(t, call,
		" -F allow_squash_merge=true",
		" -F allow_merge_commit=false",
		" -F allow_rebase_merge=false",
		" -f squash_merge_commit_title=PR_TITLE")
}

func TestNewGitHubRequiresCIOKAndNoApprovingReview(t *testing.T) {
	s := newSandbox(t)
	s.fakeGH("")

	s.mustRun("new", "demo", "--no-input", "--github")

	call := ghCall(t, s.ghCalls(), "branches/main/protection")
	wantContains(t, call,
		" -f required_status_checks[checks][][context]=ci-ok",
		" -F required_pull_request_reviews[required_approving_review_count]=0")
	if strings.Count(call, "[context]=") != 1 {
		t.Errorf("want ci-ok as the only required check in:\n%s", call)
	}
}

func TestNewGitHubTurnsOnPushProtectionBeforeThePush(t *testing.T) {
	s := newSandbox(t)
	s.fakeGH("")

	s.mustRun("new", "demo", "--no-input", "--github")

	calls := s.ghCalls()
	wantContains(t, ghCall(t, calls, "secret_scanning_push_protection"), "before push: ")
	wantContains(t, ghCall(t, calls, "branches/main/protection"), "after push: ")
}

func TestNewGitHubPushesMainAtTheFirstCommit(t *testing.T) {
	s := newSandbox(t)
	bare := s.fakeGH("")

	s.mustRun("new", "demo", "--no-input", "--github")

	if got, want := s.git("--git-dir", bare, "rev-parse", "main"), s.git("rev-parse", "HEAD"); got != want {
		t.Errorf("main on the remote = %s, want the first commit %s", got, want)
	}
	if got := s.git("config", "remote.origin.url"); got != "https://github.com/test-user/demo.git" {
		t.Errorf("origin = %q, want https://github.com/test-user/demo.git", got)
	}
	if got := s.git("rev-parse", "--abbrev-ref", "main@{upstream}"); got != "origin/main" {
		t.Errorf("main's upstream = %q, want origin/main", got)
	}
}

func TestNewWithoutGitHubDoesNotCallGH(t *testing.T) {
	s := newSandbox(t)
	s.fakeGH("")

	s.mustRun("new", "demo", "--no-input")

	if got := s.ghCalls(); len(got) != 0 {
		t.Errorf("sow new without --github called gh:\n%s", strings.Join(got, "\n"))
	}
}

func TestNewGitHubWithoutGHCreatesNothing(t *testing.T) {
	s := newSandbox(t)
	s.isolatePath()

	got := s.run("new", "demo", "--no-input", "--github")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "gh isn't installed", "https://cli.github.com")
	if s.exists("demo") {
		t.Error("sow new created demo without gh")
	}
}

func TestNewGitHubLoggedOutCreatesNothing(t *testing.T) {
	s := newSandbox(t)
	s.fakeTool("gh", `echo "You are not logged into any GitHub hosts." >&2; exit 1`)

	got := s.run("new", "demo", "--no-input", "--github")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr, "gh auth login", "You are not logged into any GitHub hosts.")
	if s.exists("demo") {
		t.Error("sow new created demo while gh was logged out")
	}
}

func TestNewGitHubReportsAFailedStepAndKeepsTheCommit(t *testing.T) {
	s := newSandbox(t)
	s.fakeGH("branches/main/protection")

	got := s.run("new", "demo", "--no-input", "--github")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr,
		"protect main",
		"gh api --method PUT repos/test-user/demo/branches/main/protection",
		"HTTP 403: Resource not accessible",
		"sow kept the project and its first commit",
		"These steps didn't finish: protect main")
	if got := s.git("log", "--format=%s"); got != "chore: initial commit from sow" {
		t.Errorf("git log = %q, want the first commit only", got)
	}
}

func TestNewGitHubDoesNotPushWhenPushProtectionIsRefused(t *testing.T) {
	s := newSandbox(t)
	bare := s.fakeGH("security_and_analysis")

	got := s.run("new", "demo", "--no-input", "--github")

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	wantContains(t, got.stderr,
		"turn on secret scanning and push protection",
		"These steps didn't finish: turn on secret scanning and push protection, push main, protect main")
	if got := s.git("--git-dir", bare, "branch", "--list"); got != "" {
		t.Errorf("the remote has branches %q, want none", got)
	}
	if got := s.ghCalls(); strings.Contains(strings.Join(got, "\n"), "branches/main/protection") {
		t.Errorf("sow protected main after the refusal:\n%s", strings.Join(got, "\n"))
	}
}

// ghCall returns the one recorded gh call that contains part.
func ghCall(t *testing.T, calls []string, part string) string {
	t.Helper()
	var found []string
	for _, call := range calls {
		if strings.Contains(call, part) {
			found = append(found, call)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want one gh call with %q, got %d in:\n%s", part, len(found), strings.Join(calls, "\n"))
	}
	return found[0]
}
