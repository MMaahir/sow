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
// fetches the old template from opts.Repo unless opts.CacheDir already has it,
// renders the old and the new template with the project's answers, and
// merges each file with git merge-file.
//
// Update returns the paths that git merge-file left conflict markers in,
// sorted. A fetch or render failure leaves the project unchanged. A failure
// while it changes files leaves the files it changed so far.
func Update(ctx context.Context, opts UpdateOptions) ([]string, error) {
	oldTemplate, err := fetchTemplate(ctx, opts)
	if err != nil {
		return nil, err
	}
	oldRender, err := render.Render(os.DirFS(oldTemplate), opts.Answers.Dirs(), opts.Answers)
	if err != nil {
		return nil, fmt.Errorf("sow %s: %w", opts.From, err)
	}
	newRender, err := render.Render(opts.Template, opts.Answers.Dirs(), opts.Answers)
	if err != nil {
		return nil, err
	}
	answersFile, err := answers.Marshal(opts.To, opts.Answers)
	if err != nil {
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
	return conflicts, nil
}

// fetchTemplate returns the directory that holds the template of release
// opts.From, fetching it from opts.Repo into opts.CacheDir if it isn't there. A
// release tag never moves, so a cached template never goes stale.
//
// It clones only the tagged commit, with no history, and keeps only its
// template/ directory. It clones into a temporary directory and renames the
// template into place, so an interrupted fetch never leaves a partial
// template in the cache.
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
	clone, err := os.MkdirTemp(templates, "fetch-")
	if err != nil {
		return "", fmt.Errorf("create the cache: %w", err)
	}
	err = cacheTemplate(ctx, opts, tag, clone, cached)
	if removeErr := os.RemoveAll(clone); removeErr != nil {
		err = errors.Join(err, fmt.Errorf("remove the clone: %w", removeErr))
	}
	if err != nil {
		return "", fmt.Errorf("fetch the template of sow %s from %s: %w", opts.From, opts.Repo, err)
	}
	return cached, nil
}

// cacheTemplate clones the commit that tag names from opts.Repo into clone
// and moves its template/ directory to cached. git runs in the project
// root, where the author runs sow update, so a relative SOW_REPO resolves
// from there.
//
// "git clone --depth 1 --branch <tag>" asks the server for that one commit
// and its files, with no history and no other branches or tags, in one
// command. The fetch is the size of one checkout of sow.
func cacheTemplate(ctx context.Context, opts UpdateOptions, tag, clone, cached string) error {
	err := run(ctx, opts.Environ, opts.Dir, "git", "clone", "--quiet", "--depth", "1", "--branch", tag,
		"--", opts.Repo, clone)
	if err != nil {
		return fmt.Errorf("check that the tag %s exists and the repo is reachable, "+
			"or set SOW_REPO to a clone of sow that has the tag: %w", tag, err)
	}
	if err := os.Rename(filepath.Join(clone, "template"), cached); err != nil {
		return fmt.Errorf("cache the template: %w", err)
	}
	return nil
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
