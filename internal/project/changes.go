package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/MMaahir/sow/internal/answers"
	"github.com/MMaahir/sow/internal/render"
)

// checkNoSymlinks returns an error that names every symlink in the project
// in dir that the pipeline would read or write through: a path of either render,
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

// changeAction is what the pipeline does to one file of the project.
type changeAction int

const (
	// changeWrite replaces the file with the new render's content.
	changeWrite changeAction = iota
	// changeMerge merges the new render into the file with git merge-file.
	changeMerge
	// changeDelete deletes the file.
	changeDelete
)

// fileChange is one change the pipeline makes to the project.
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

// planChanges reads the project in dir and returns the changes that bring it
// from oldRender to newRender. It changes nothing on disk. For a new project,
// dir doesn't exist and oldRender is nil, so every file is written.
//
// A path that only the project has isn't in either render, so the plan never
// touches it. A file the new render dropped is deleted only when the project's
// copy equals the old render, so an update never deletes an edit.
func planChanges(dir string, oldRender, newRender []render.File) ([]fileChange, error) {
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

// applyChanges makes changes in p.dir and returns the paths that git
// merge-file left conflict markers in.
func (p pipeline) applyChanges(ctx context.Context, project *tools, changes []fileChange) ([]string, error) {
	var conflicts []string
	for _, change := range changes {
		switch change.action {
		case changeWrite:
			if err := render.Write(p.dir, []render.File{{Path: change.path, Content: change.newContent}}); err != nil {
				return nil, fmt.Errorf("write %s: %w", change.path, err)
			}
		case changeDelete:
			if err := os.Remove(filepath.Join(p.dir, filepath.FromSlash(change.path))); err != nil {
				return nil, err
			}
			if err := removeEmptyParents(p.dir, change.path); err != nil {
				return nil, err
			}
		case changeMerge:
			conflicted, err := p.mergeFile(ctx, project, change)
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
func (p pipeline) mergeFile(ctx context.Context, project *tools, change fileChange) (bool, error) {
	inputs, err := os.MkdirTemp(p.cacheDir, "merge-")
	if err != nil {
		return false, fmt.Errorf("merge %s: %w", change.path, err)
	}
	conflicted, err := p.mergeFileWith(ctx, project, change, inputs)
	if removeErr := os.RemoveAll(inputs); removeErr != nil {
		err = errors.Join(err, removeErr)
	}
	return conflicted, err
}

// mergeFileWith runs git merge-file with the old and the new render written
// to files in the directory inputs.
func (p pipeline) mergeFileWith(ctx context.Context, project *tools, change fileChange, inputs string) (bool, error) {
	oldFile := filepath.Join(inputs, "old")
	newFile := filepath.Join(inputs, "new")
	if err := os.WriteFile(oldFile, change.oldContent, 0o644); err != nil {
		return false, err
	}
	if err := os.WriteFile(newFile, change.newContent, 0o644); err != nil {
		return false, err
	}

	// git merge-file exits with the number of conflicts, at most 127, and
	// with a negative status on an error, which the shell sees as 255.
	conflicts, err := project.exitCode(ctx, 127, "git", "merge-file",
		"-L", "yours", "-L", "sow "+p.from, "-L", "sow "+p.to,
		"--", filepath.FromSlash(change.path), oldFile, newFile)
	if err != nil {
		return false, fmt.Errorf("merge %s: %w", change.path, err)
	}
	return conflicts > 0, nil
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
