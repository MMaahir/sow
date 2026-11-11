package cli

import (
	"fmt"
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
			if err != nil {
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
