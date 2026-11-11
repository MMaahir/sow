package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/MMaahir/sow/internal/answers"
	"github.com/MMaahir/sow/internal/render"
)

// UpdateOptions describes the update that Update makes.
type UpdateOptions struct {
	// Dir is the root of the project.
	Dir string
	// Answers come from the project's answers file.
	Answers answers.Answers
	// From is the sow release the answers file records.
	From string
	// To is the sow release of Template. The answers file records it after
	// the update.
	To string
	// Template holds the template directories of release To.
	Template fs.FS
	// Repo is the git repo of sow, which has a tag for every release.
	Repo string
	// CacheDir holds the templates of the releases Update fetched before.
	CacheDir string
	// Environ is the environment for the commands Update starts.
	Environ []string
}

// Update brings the project in opts.Dir from the template of release
// opts.From to opts.Template, and records opts.To in the answers file. It
// refuses a project that isn't a git repo or has uncommitted or untracked
// files, so git reset --hard && git clean -fd can always undo the update. For
// the same reason it refuses a project where a file it would read or write,
// or a directory above one, is a symlink. It fetches the old template from
// opts.Repo unless opts.CacheDir already has it, renders the old and the new
// template with the project's answers, merges each file with git merge-file,
// and runs the follow-up steps. It never commits.
//
// Update returns the paths that git merge-file left conflict markers in,
// sorted. When there are any, it skips the follow-up steps, because a
// manifest with conflict markers can't be installed. A failed check, fetch or
// render leaves the project unchanged. A failure while it changes files, or a
// failed follow-up step, leaves the files it changed so far.
func Update(ctx context.Context, opts UpdateOptions) ([]string, error) {
	if err := checkClean(ctx, opts.Environ, opts.Dir); err != nil {
		return nil, err
	}
	oldTemplate, err := fetchTemplate(ctx, opts)
	if err != nil {
		return nil, err
	}
	// The answers file keeps no entry for a component it lacks, but both
	// renders need one.
	renderAnswers := opts.Answers.WithComponentsOff()
	oldRender, err := render.Render(os.DirFS(oldTemplate), renderAnswers.Dirs(), renderAnswers)
	if err != nil {
		return nil, fmt.Errorf("sow %s: %w", opts.From, err)
	}
	newRender, err := render.Render(opts.Template, renderAnswers.Dirs(), renderAnswers)
	if err != nil {
		return nil, err
	}
	answersFile, err := answers.Marshal(opts.To, opts.Answers)
	if err != nil {
		return nil, err
	}

	if err := checkNoSymlinks(opts.Dir, oldRender, newRender); err != nil {
		return nil, err
	}
	changes, err := planUpdate(opts.Dir, oldRender, newRender)
	if err != nil {
		return nil, err
	}
	conflicts, err := applyUpdate(ctx, opts, changes)
	if err != nil {
		return nil, err
	}
	// The merged files are at release To even when some have conflicts.
	if err := os.WriteFile(filepath.Join(opts.Dir, answers.FileName), answersFile, 0o644); err != nil {
		return nil, fmt.Errorf("write the answers file: %w", err)
	}
	if len(conflicts) > 0 {
		return conflicts, nil
	}
	return nil, runFollowUpSteps(ctx, opts.Environ, opts.Dir, true)
}

// checkClean returns an error if dir isn't in a git repo or has uncommitted
// changes or untracked files. git clean -fd deletes untracked files and git
// reset --hard discards uncommitted changes, so undoing an update would lose
// them.
func checkClean(ctx context.Context, environ []string, dir string) error {
	status, err := output(ctx, environ, dir, "git", "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("%s isn't a git repo, or git can't read it: sow update needs git to undo an update, "+
			"so run git init and commit the project, then run sow update again: %w", dir, err)
	}
	if status != "" {
		return fmt.Errorf("%s has uncommitted changes or untracked files: "+
			"commit or stash them, then run sow update again, so git reset --hard && git clean -fd "+
			"can undo the update without losing your work\n%s", dir, strings.TrimRight(status, "\n"))
	}
	return nil
}

// fetchTemplate returns the directory that holds the template of release
// opts.From, fetching it from opts.Repo into opts.CacheDir if it isn't there. A
// release tag never moves, so a cached template never goes stale.
//
// It fetches into a temporary directory and renames the template into place,
// so an interrupted fetch never leaves a partial template in the cache.
func fetchTemplate(ctx context.Context, opts UpdateOptions) (string, error) {
	tag := "v" + opts.From
	templates := filepath.Join(opts.CacheDir, "templates")
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
	err = cacheTemplate(ctx, opts, tag, fetch, cached)
	if removeErr := os.RemoveAll(fetch); removeErr != nil {
		err = errors.Join(err, fmt.Errorf("remove the fetch: %w", removeErr))
	}
	if err != nil {
		return "", fmt.Errorf("fetch the template of sow %s from %s: %w", opts.From, opts.Repo, err)
	}
	return cached, nil
}

// cacheTemplate fetches the tag from opts.Repo into a bare repo in the
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
func cacheTemplate(ctx context.Context, opts UpdateOptions, tag, fetch, cached string) error {
	repo := filepath.Join(fetch, "repo")
	ref := "refs/tags/" + tag
	if err := run(ctx, opts.Environ, opts.Dir, "git", "init", "--quiet", "--bare", repo); err != nil {
		return err
	}
	err := run(ctx, opts.Environ, opts.Dir, "git", "--git-dir", repo,
		"fetch", "--quiet", "--depth", "1", "--no-tags", "--", opts.Repo, "+"+ref+":"+ref)
	if err != nil {
		return fmt.Errorf("check that the tag %s exists and the repo is reachable, "+
			"or set SOW_REPO to a clone of sow that has the tag: %w", tag, err)
	}
	listing, err := output(ctx, opts.Environ, opts.Dir, "git", "--git-dir", repo,
		"ls-tree", "-r", "-z", ref+":template")
	if err != nil {
		return fmt.Errorf("list the template of %s: %w", tag, err)
	}

	template := filepath.Join(fetch, "template")
	for _, entry := range strings.Split(listing, "\x00") {
		// Each entry is "<mode> <type> <object>\t<path>".
		info, name, ok := strings.Cut(entry, "\t")
		fields := strings.Fields(info)
		if !ok || len(fields) != 3 || fields[1] != "blob" {
			continue
		}
		content, err := output(ctx, opts.Environ, opts.Dir, "git", "--git-dir", repo, "cat-file", "blob", fields[2])
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

// checkNoSymlinks returns an error that names every symlink in the project
// in dir that Update would read or write through: a path of either render,
// the answers file, or a directory above one of them. Writing through a
// symlink would change a file outside the project, which git reset --hard
// can't restore.
func checkNoSymlinks(dir string, oldRender, newRender []render.File) error {
	names := []string{answers.FileName}
	for _, file := range slices.Concat(oldRender, newRender) {
		names = append(names, file.Path)
	}
	var links []string
	for _, name := range names {
		link, err := symlinkIn(dir, name)
		if err != nil {
			return err
		}
		if link != "" && !slices.Contains(links, link) {
			links = append(links, link)
		}
	}
	slices.Sort(links)
	var problems []error
	for _, link := range links {
		problems = append(problems, fmt.Errorf("%s is a symbolic link, and sow update won't write through it, "+
			"because git reset --hard can't restore the file it points to: "+
			"replace the link with the file or directory it points to, commit it, then run sow update again", link))
	}
	return errors.Join(problems...)
}

// symlinkIn returns the first part of name, a path relative to root with
// forward slashes, that is a symlink in root, such as "docs" for
// "docs/guide.md". It returns "" when no part is a symlink, including when a
// part doesn't exist.
func symlinkIn(root, name string) (string, error) {
	parts := strings.Split(name, "/")
	for i := range parts {
		part := path.Join(parts[:i+1]...)
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(part)))
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return part, nil
		}
	}
	return "", nil
}

// changeAction is what Update does to one file of the project.
type changeAction int

const (
	// changeWrite replaces the file with the new render's content.
	changeWrite changeAction = iota
	// changeMerge merges the new render into the file with git merge-file.
	changeMerge
	// changeDelete deletes the file.
	changeDelete
)

// fileChange is one change Update makes to the project.
type fileChange struct {
	action changeAction
	// path is relative to the project root and uses forward slashes.
	path string
	// oldContent is the old render's file. It's empty when the old render
	// doesn't have the path.
	oldContent []byte
	// newContent is the new render's file.
	newContent []byte
}

// planUpdate reads the project in dir and returns the changes that bring it
// from oldRender to newRender. It changes nothing on disk.
//
// A path that only the project has isn't in either render, so the plan never
// touches it. A file the new render dropped is deleted only when the project's
// copy equals the old render, so an update never deletes an edit.
func planUpdate(dir string, oldRender, newRender []render.File) ([]fileChange, error) {
	oldFiles := map[string][]byte{}
	for _, file := range oldRender {
		oldFiles[file.Path] = file.Content
	}
	var changes []fileChange

	for _, file := range newRender {
		oldContent, inOld := oldFiles[file.Path]
		delete(oldFiles, file.Path)
		change := fileChange{path: file.Path, oldContent: oldContent, newContent: file.Content}

		current, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(file.Path)))
		switch {
		case errors.Is(err, fs.ErrNotExist) && inOld:
			// The project deleted a file the old render had. It stays deleted.
			continue
		case errors.Is(err, fs.ErrNotExist):
			change.action = changeWrite
		case err != nil:
			return nil, err
		case bytes.Equal(current, file.Content):
			continue
		case inOld && bytes.Equal(current, oldContent):
			change.action = changeWrite
		case inOld && bytes.Equal(oldContent, file.Content):
			// The template didn't change the file, so the project's edit stays.
			continue
		default:
			// Without an old render, git merge-file merges against an empty
			// file, so a file the project already had at a new path is kept
			// and differences show as conflicts.
			change.action = changeMerge
		}
		changes = append(changes, change)
	}

	for _, file := range oldRender {
		if _, dropped := oldFiles[file.Path]; !dropped {
			continue
		}
		current, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(file.Path)))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if bytes.Equal(current, file.Content) {
			changes = append(changes, fileChange{action: changeDelete, path: file.Path})
		}
	}
	return changes, nil
}

// applyUpdate makes changes in opts.Dir and returns the paths that git
// merge-file left conflict markers in.
func applyUpdate(ctx context.Context, opts UpdateOptions, changes []fileChange) ([]string, error) {
	var conflicts []string
	for _, change := range changes {
		switch change.action {
		case changeWrite:
			if err := render.Write(opts.Dir, []render.File{{Path: change.path, Content: change.newContent}}); err != nil {
				return nil, fmt.Errorf("write %s: %w", change.path, err)
			}
		case changeDelete:
			if err := os.Remove(filepath.Join(opts.Dir, filepath.FromSlash(change.path))); err != nil {
				return nil, err
			}
			if err := removeEmptyParents(opts.Dir, change.path); err != nil {
				return nil, err
			}
		case changeMerge:
			conflicted, err := mergeFile(ctx, opts, change)
			if err != nil {
				return nil, err
			}
			if conflicted {
				conflicts = append(conflicts, change.path)
			}
		}
	}
	return conflicts, nil
}

// mergeFile merges the change from the old render to the new render into the
// project's file with git merge-file, which writes the result, conflict
// markers included, into the file. It reports whether the result has
// conflicts.
func mergeFile(ctx context.Context, opts UpdateOptions, change fileChange) (bool, error) {
	inputs, err := os.MkdirTemp(opts.CacheDir, "merge-")
	if err != nil {
		return false, fmt.Errorf("merge %s: %w", change.path, err)
	}
	conflicted, err := mergeFileWith(ctx, opts, change, inputs)
	if removeErr := os.RemoveAll(inputs); removeErr != nil {
		err = errors.Join(err, removeErr)
	}
	return conflicted, err
}

// mergeFileWith runs git merge-file with the old and the new render written
// to files in the directory inputs.
func mergeFileWith(ctx context.Context, opts UpdateOptions, change fileChange, inputs string) (bool, error) {
	oldFile := filepath.Join(inputs, "old")
	newFile := filepath.Join(inputs, "new")
	if err := os.WriteFile(oldFile, change.oldContent, 0o644); err != nil {
		return false, err
	}
	if err := os.WriteFile(newFile, change.newContent, 0o644); err != nil {
		return false, err
	}

	err := run(ctx, opts.Environ, opts.Dir, "git", "merge-file",
		"-L", "yours", "-L", "sow "+opts.From, "-L", "sow "+opts.To,
		"--", filepath.FromSlash(change.path), oldFile, newFile)
	// git merge-file exits with the number of conflicts, at most 127, and
	// with a negative status on an error, which the shell sees as 255.
	var exit *exec.ExitError
	switch {
	case err == nil:
		return false, nil
	case errors.As(err, &exit) && exit.ExitCode() >= 1 && exit.ExitCode() <= 127:
		return true, nil
	default:
		return false, fmt.Errorf("merge %s: %w", change.path, err)
	}
}

// removeEmptyParents removes each directory above name, a path relative to
// root, that is now empty, up to root. git doesn't track directories, so an
// empty one would only be left behind on disk.
func removeEmptyParents(root, name string) error {
	for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
		dir := filepath.Join(root, filepath.FromSlash(parent))
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			return nil
		}
		if err := os.Remove(dir); err != nil {
			return err
		}
	}
	return nil
}
