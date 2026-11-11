package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MMaahir/sow/internal/answers"
	"github.com/MMaahir/sow/internal/project"
)

// defaultRepo is the git repo sow update fetches old templates from when
// SOW_REPO isn't set.
const defaultRepo = "https://github.com/MMaahir/sow.git"

func updateCommand(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update the project in this directory to the template of this sow version.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			version, recorded, err := answers.Read(filepath.Join(env.Dir, answers.FileName))
			if errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("%s has no %s: run sow update from the root of a project sow created",
					env.Dir, answers.FileName)
			}
			if err != nil {
				return err
			}
			if err := checkRenderable(recorded[answers.ProjectName], version, env.Version); err != nil {
				return err
			}
			if version != env.Version {
				return update(cmd.Context(), env, version, recorded)
			}
			if _, err := fmt.Fprintf(env.Stdout, "%s is already at sow %s\n",
				recorded[answers.ProjectName], version); err != nil {
				return fmt.Errorf("print the result: %w", err)
			}
			return nil
		},
	}
}

// update brings the project from sow version to env.Version and tells the
// author to review the result.
func update(ctx context.Context, env Env, version string, recorded answers.Answers) error {
	repo := env.Getenv("SOW_REPO")
	if repo == "" {
		repo = defaultRepo
	}
	conflicts, err := project.Update(ctx, project.UpdateOptions{
		Dir:      env.Dir,
		Answers:  recorded,
		From:     version,
		To:       env.Version,
		Template: env.Template,
		Repo:     repo,
		CacheDir: cacheDir(env),
		Environ:  env.Environ,
	})
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("the update from sow %s to %s left conflict markers in %s: "+
			"resolve each conflict, then review the changes with git diff",
			version, env.Version, strings.Join(conflicts, ", "))
	}
	if _, err := fmt.Fprintf(env.Stdout, "Updated %s from sow %s to %s. Review the changes with git diff.\n",
		recorded[answers.ProjectName], version, env.Version); err != nil {
		return fmt.Errorf("print the result: %w", err)
	}
	return nil
}

// cacheDir returns $XDG_CACHE_HOME/sow, or ~/.cache/sow when XDG_CACHE_HOME
// isn't set, the way personalDefaultsPath finds the personal defaults file.
func cacheDir(env Env) string {
	base := env.Getenv("XDG_CACHE_HOME")
	if base == "" {
		base = filepath.Join(env.Getenv("HOME"), ".cache")
	}
	return filepath.Join(base, "sow")
}
