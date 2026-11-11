package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

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
			repo := env.Getenv("SOW_REPO")
			if repo == "" {
				repo = defaultRepo
			}
			result, err := project.Update(cmd.Context(), project.UpdateOptions{
				Dir:      env.Dir,
				Version:  env.Version,
				Template: env.Template,
				Repo:     repo,
				CacheDir: cacheDir(env),
				Environ:  env.Environ,
			})
			if err != nil {
				return err
			}
			return reportUpdate(env, result)
		},
	}
}

// reportUpdate tells the author what Update did: that the project was
// already at env.Version, which files have conflict markers, or that the
// update needs a review.
func reportUpdate(env Env, result project.UpdateResult) error {
	if len(result.Conflicts) > 0 {
		return fmt.Errorf("the update from sow %s to %s left conflict markers in %s: "+
			"resolve each conflict, run the follow-up steps sow update skipped, such as vp install, "+
			"and review the changes with git diff. To undo the update, run git reset --hard && git clean -fd",
			result.From, env.Version, strings.Join(result.Conflicts, ", "))
	}
	message := fmt.Sprintf("Updated %s from sow %s to %s. Review the changes with git diff.\n",
		result.Name, result.From, env.Version)
	if result.UpToDate {
		message = fmt.Sprintf("%s is already at sow %s\n", result.Name, result.From)
	}
	if _, err := fmt.Fprint(env.Stdout, message); err != nil {
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
