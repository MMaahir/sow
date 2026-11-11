package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/MMaahir/sow/internal/answers"
)

func updateCommand(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update the project in this directory to the template of this sow version.",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
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
				return nil
			}
			if _, err := fmt.Fprintf(env.Stdout, "%s is already at sow %s\n",
				recorded[answers.ProjectName], version); err != nil {
				return fmt.Errorf("print the result: %w", err)
			}
			return nil
		},
	}
}
