package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/MMaahir/sow/internal/answers"
)

// UpdateOptions describes the update that Update makes.
type UpdateOptions struct {
	// Dir is the root of the project.
	Dir string
	// Version is the sow release of Template. The answers file records it
	// after the update.
	Version string
	// Template holds the template directories of release Version.
	Template fs.FS
	// Repo is the git repo of sow, which has a tag for every release.
	Repo string
	// CacheDir holds the templates of the releases Update fetched before.
	CacheDir string
	// Environ is the environment for the commands Update starts.
	Environ []string
}

// UpdateResult is what Update found and did.
type UpdateResult struct {
	// Name is the project's project_name.
	Name string
	// From is the sow release the answers file recorded before the update.
	From string
	// UpToDate reports that the project was already at opts.Version, so
	// Update changed nothing.
	UpToDate bool
	// Conflicts holds the paths that git merge-file left conflict markers in,
	// sorted.
	Conflicts []string
}

// Update brings the project in opts.Dir from the template of the release its
// answers file records to opts.Template, and records opts.Version in the
// answers file. It refuses a directory without an answers file, and a
// recorded release it can't render: a development build, a version that
// isn't a release, or one newer than opts.Version. A project already at
// opts.Version is left alone, with UpToDate set.
//
// It refuses a project that isn't a git repo or has uncommitted or untracked
// files, so git reset --hard && git clean -fd can always undo the update. For
// the same reason it refuses a project where a file it would read or write,
// or a directory above one, is a symlink. When the answers turn on a
// TypeScript component, it checks that Vite+ is installed and set up. It
// fetches the old template from opts.Repo unless opts.CacheDir already has
// it, renders the old and the new template with the project's answers, merges
// each file with git merge-file, and runs the follow-up steps that don't write
// gitignored files. It never commits.
//
// When the merge leaves conflict markers, Update skips the follow-up steps,
// because a manifest with conflict markers can't be installed. A failed check,
// fetch or render leaves the project unchanged. A failure while it changes
// files, or a failed follow-up step, leaves the files it changed so far.
func Update(ctx context.Context, opts UpdateOptions) (UpdateResult, error) {
	from, recorded, err := answers.Read(filepath.Join(opts.Dir, answers.FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return UpdateResult{}, fmt.Errorf("%s has no %s: run sow update from the root of a project sow created",
			opts.Dir, answers.FileName)
	}
	if err != nil {
		return UpdateResult{}, err
	}
	result := UpdateResult{Name: recorded[answers.ProjectName], From: from}
	if err := checkRenderable(result.Name, from, opts.Version); err != nil {
		return UpdateResult{}, err
	}
	if from == opts.Version {
		result.UpToDate = true
		return result, nil
	}
	result.Conflicts, err = pipeline{
		dir:      opts.Dir,
		environ:  opts.Environ,
		from:     from,
		recorded: recorded,
		to:       opts.Version,
		wanted:   recorded,
		template: opts.Template,
		repo:     opts.Repo,
		cacheDir: opts.CacheDir,
	}.run(ctx)
	return result, err
}

// checkClean returns an error if t's directory isn't in a git repo or has uncommitted
// changes or untracked files. git clean -fd deletes untracked files and git
// reset --hard discards uncommitted changes, so undoing an update would lose
// them.
func checkClean(ctx context.Context, t *tools) error {
	status, err := t.output(ctx, "git", "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("%s isn't a git repo, or git can't read it: sow update needs git to undo an update, "+
			"so run git init and commit the project, then run sow update again: %w", t.dir, err)
	}
	if status != "" {
		return fmt.Errorf("%s has uncommitted changes or untracked files: "+
			"commit or stash them, then run sow update again, so git reset --hard && git clean -fd "+
			"can undo the update without losing your work\n%s", t.dir, strings.TrimRight(status, "\n"))
	}
	return nil
}

// fetchTemplate returns the directory that holds the template of release
// p.from, fetching it from p.repo into p.cacheDir if it isn't there. A
// release tag never moves, so a cached template never goes stale.
//
// It fetches into a temporary directory and renames the template into place,
// so an interrupted fetch never leaves a partial template in the cache.
func (p pipeline) fetchTemplate(ctx context.Context, project *tools) (string, error) {
	tag := "v" + p.from
	templates := filepath.Join(p.cacheDir, "templates")
	cached := filepath.Join(templates, tag)
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}

	if err := os.MkdirAll(templates, 0o755); err != nil {
		return "", fmt.Errorf("create the cache: %w", err)
	}
	fetch, err := os.MkdirTemp(templates, "fetch-")
	if err != nil {
		return "", fmt.Errorf("create the cache: %w", err)
	}
	err = p.cacheTemplate(ctx, project, tag, fetch, cached)
	if removeErr := os.RemoveAll(fetch); removeErr != nil {
		err = errors.Join(err, fmt.Errorf("remove the fetch: %w", removeErr))
	}
	if err != nil {
		return "", fmt.Errorf("fetch the template of sow %s from %s: %w", p.from, p.repo, err)
	}
	return cached, nil
}

// cacheTemplate fetches the tag from p.repo into a bare repo in the
// directory fetch, writes each file under the tag's template/ directory to
// fetch, and moves the result to cached. git runs in the project root, where
// the author runs sow update, so a relative SOW_REPO resolves from there.
//
// It fetches refs/tags/<tag> by its full name, so a branch with the tag's
// name never stands in for the release. It fetches one commit with no
// history. It reads each file with git cat-file, which returns the committed
// bytes. A checkout or git archive would apply the author's smudge filters
// and line ending settings, and the old render must match the release
// exactly, or an edit could look like the template's own content.
func (p pipeline) cacheTemplate(ctx context.Context, project *tools, tag, fetch, cached string) error {
	repo := filepath.Join(fetch, "repo")
	ref := "refs/tags/" + tag
	if err := project.run(ctx, "git", "init", "--quiet", "--bare", repo); err != nil {
		return err
	}
	err := project.run(ctx, "git", "--git-dir", repo,
		"fetch", "--quiet", "--depth", "1", "--no-tags", "--", p.repo, "+"+ref+":"+ref)
	if err != nil {
		return fmt.Errorf("check that the tag %s exists and the repo is reachable, "+
			"or set SOW_REPO to a clone of sow that has the tag: %w", tag, err)
	}
	listing, err := project.output(ctx, "git", "--git-dir", repo,
		"ls-tree", "-r", "-z", ref+":template")
	if err != nil {
		return fmt.Errorf("list the template of %s: %w", tag, err)
	}

	// The directory exists even when the listing has no files, so a release
	// with an empty template fails in rendering with a message that names
	// the missing directory.
	template := filepath.Join(fetch, "template")
	if err := os.MkdirAll(template, 0o755); err != nil {
		return fmt.Errorf("cache the template: %w", err)
	}
	for _, entry := range strings.Split(listing, "\x00") {
		// Each entry is "<mode> <type> <object>\t<path>".
		info, name, ok := strings.Cut(entry, "\t")
		fields := strings.Fields(info)
		if !ok || len(fields) != 3 || fields[1] != "blob" {
			continue
		}
		content, err := project.output(ctx, "git", "--git-dir", repo, "cat-file", "blob", fields[2])
		if err != nil {
			return fmt.Errorf("read template/%s at %s: %w", name, tag, err)
		}
		target := filepath.Join(template, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("cache the template: %w", err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return fmt.Errorf("cache the template: %w", err)
		}
	}
	if err := os.Rename(template, cached); err != nil {
		return fmt.Errorf("cache the template: %w", err)
	}
	return nil
}
