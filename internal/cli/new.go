package cli

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MMaahir/sow"
	"github.com/MMaahir/sow/internal/answers"
	"github.com/MMaahir/sow/internal/project"
)

func newCommand(env Env) *cobra.Command {
	var (
		preset  string
		noInput bool
	)
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Generate a new project from the template.",
		Args:  cobra.ExactArgs(1),
	}
	cmd.Flags().StringVar(&preset, "preset", "",
		"a named set of answers: "+strings.Join(answers.PresetNames(), ", "))
	cmd.Flags().BoolVar(&noInput, "no-input", false,
		"never ask questions; fail if an answer is missing")
	for _, q := range answers.Questions() {
		if q.Key != answers.ProjectName {
			cmd.Flags().String(flagName(q.Key), "", q.Help)
		}
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Sources are listed from the highest priority to the lowest.
		given := answers.Answers{answers.ProjectName: args[0]}
		for _, q := range answers.Questions() {
			if flag := cmd.Flags().Lookup(flagName(q.Key)); flag != nil && flag.Changed {
				given[q.Key] = flag.Value.String()
			}
		}
		// Every source is checked before any error is returned, so one run
		// reports every problem with the answers.
		var problems []error
		sources := []answers.Answers{given}
		if preset != "" {
			fromPreset, err := answers.Preset(preset)
			problems = append(problems, err)
			sources = append(sources, fromPreset)
		}
		personal, err := answers.ReadPersonalDefaults(personalDefaultsPath(env))
		problems = append(problems, err)
		sources = append(sources, personal)
		// Nothing is asked when a source is already wrong: the author fixes
		// the preset or the personal defaults file first.
		if !noInput && errors.Join(problems...) == nil {
			typed, err := ask(cmd.Context(), env.Prompter, sources)
			if err != nil {
				return err
			}
			sources = append(sources, typed)
		}

		resolved, err := answers.Resolve(sources...)
		problems = append(problems, err)
		if err := errors.Join(problems...); err != nil {
			return err
		}
		return project.New(cmd.Context(), project.Options{
			Dir:      filepath.Join(env.Dir, resolved[answers.ProjectName]),
			Answers:  resolved,
			Version:  env.Version,
			Template: sow.Template(),
			Environ:  env.Environ,
		})
	}
	return cmd
}

// flagName turns an answer key such as "user_name" into its flag, "user-name".
func flagName(key string) string {
	return strings.ReplaceAll(key, "_", "-")
}

// personalDefaultsPath returns $XDG_CONFIG_HOME/sow/config.yml, or
// ~/.config/sow/config.yml when XDG_CONFIG_HOME isn't set.
func personalDefaultsPath(env Env) string {
	base := env.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(env.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "sow", "config.yml")
}
