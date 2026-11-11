package project

import (
	"context"
	"fmt"
	"strings"
)

// githubStep is one thing sow does to put a project on GitHub, such as
// creating the repo or protecting main.
type githubStep struct {
	// name says what the step does. It prefixes the error when the step fails.
	name string
	// command and args are the command the step starts in the project
	// directory.
	command string
	args    []string
}

// githubSteps returns the steps that put the project in dir on GitHub as
// owner/name, with origin at the URL remote, in the order they run.
//
// Push protection goes on before the push, so the first push is covered.
// Branch protection goes on after the push, because GitHub's branch
// protection API needs the branch to exist. The protection matches sow's own
// main: ci-ok is the only required check and no approving review is needed,
// because agents merge their own pull requests.
func githubSteps(owner, name, remote string) []githubStep {
	repo := owner + "/" + name
	return []githubStep{
		{
			name:    "create the GitHub repo",
			command: "gh",
			args:    []string{"repo", "create", repo, "--public"},
		},
		{
			name:    "add the GitHub repo as origin",
			command: "git",
			args:    []string{"remote", "add", "origin", remote},
		},
		{
			// The squash commit takes the pull request's title. GitHub's
			// default takes the commit title for a single-commit pull
			// request, which would skip the Conventional Commits check on
			// titles.
			name:    "allow only squash merges",
			command: "gh",
			args: []string{
				"api", "--method", "PATCH", "repos/" + repo,
				"-F", "allow_squash_merge=true",
				"-F", "allow_merge_commit=false",
				"-F", "allow_rebase_merge=false",
				"-f", "squash_merge_commit_title=PR_TITLE",
				"-f", "squash_merge_commit_message=PR_BODY",
			},
		},
		{
			name:    "turn on secret scanning and push protection",
			command: "gh",
			args: []string{
				"api", "--method", "PATCH", "repos/" + repo,
				"-f", "security_and_analysis[secret_scanning][status]=enabled",
				"-f", "security_and_analysis[secret_scanning_push_protection][status]=enabled",
			},
		},
		{
			name:    "push main",
			command: "git",
			args:    []string{"push", "--set-upstream", "origin", "main"},
		},
		{
			// strict makes a pull request's branch be up to date with main
			// before it merges. enforce_admins stays off so the author can
			// still merge with gh pr merge --admin before the project's CI
			// reports ci-ok.
			name:    "protect main",
			command: "gh",
			args: []string{
				"api", "--method", "PUT", "repos/" + repo + "/branches/main/protection",
				"-F", "required_status_checks[strict]=true",
				"-f", "required_status_checks[checks][][context]=ci-ok",
				"-F", "enforce_admins=false",
				"-F", "required_pull_request_reviews[required_approving_review_count]=0",
				"-F", "restrictions=null",
			},
		},
	}
}

// checkGitHub checks that gh is on PATH and logged in to github.com, so sow
// refuses before it writes anything, and returns the URL of the repo
// owner/name in the git protocol gh is set up for. The error says how to
// install gh or how to log in.
//
// gh login sets up git credentials for one protocol, so a push to a URL in
// the other protocol could fail after the repo exists.
func checkGitHub(ctx context.Context, environ []string, dir, owner, name string) (string, error) {
	if _, err := lookPath(environ, "gh"); err != nil {
		return "", fmt.Errorf("%w: --github needs the GitHub CLI. Install it from https://cli.github.com", err)
	}
	if err := run(ctx, environ, dir, "gh", "auth", "status", "--hostname", "github.com"); err != nil {
		return "", fmt.Errorf("--github needs gh logged in to github.com: run gh auth login, then try again: %w", err)
	}
	protocol, err := output(ctx, environ, dir, "gh", "config", "get", "git_protocol", "--host", "github.com")
	if err != nil {
		return "", fmt.Errorf("read the git protocol gh uses for github.com: %w", err)
	}
	if strings.TrimSpace(protocol) == "ssh" {
		return "git@github.com:" + owner + "/" + name + ".git", nil
	}
	return "https://github.com/" + owner + "/" + name + ".git", nil
}

// publish runs the GitHub steps in dir in order, with origin at the URL
// remote, and stops at the first error.
// The error names the failed step and its command, and lists the steps that
// didn't finish. The project and its first commit stay on disk.
func publish(ctx context.Context, environ []string, dir, owner, name, remote string) error {
	steps := githubSteps(owner, name, remote)
	for i, step := range steps {
		if err := run(ctx, environ, dir, step.command, step.args...); err != nil {
			var left []string
			for _, s := range steps[i:] {
				left = append(left, s.name)
			}
			return fmt.Errorf("%s: %w\nsow kept the project and its first commit in %s. "+
				"These steps didn't finish: %s",
				step.name, err, dir, strings.Join(left, ", "))
		}
	}
	return nil
}
