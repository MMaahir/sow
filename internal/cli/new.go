package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MMaahir/sow/internal/answers"
	"github.com/MMaahir/sow/internal/project"
)

func newCommand(env Env) *cobra.Command {
	var (
		preset  string
		noInput bool
		github  bool
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
	cmd.Flags().BoolVar(&github, "github", false,
		"create a public GitHub repo with gh, protect main and push the first commit")
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
		// Nothing is asked when a source is already wrong, when an answer
		// that was given is invalid, or when the target directory exists:
		// the author fixes the cause first. project.New repeats the
		// directory check and reports it.
		settled := answers.Settle(sources...)
		givenWrong := slices.ContainsFunc(settled.Problems, func(p answers.Problem) bool { return !p.Askable })
		if !noInput && errors.Join(problems...) == nil && !givenWrong &&
			project.CheckTarget(filepath.Join(env.Dir, args[0])) == nil {
			typed, err := ask(cmd.Context(), env.Prompter, sources)
			if err != nil {
				return err
			}
			settled = answers.Settle(append(sources, typed)...)
		}

		for _, problem := range settled.Problems {
			problems = append(problems, adviseNew(problem))
		}
		if err := errors.Join(problems...); err != nil {
			return err
		}
		return project.New(cmd.Context(), project.Options{
			Dir:      filepath.Join(env.Dir, settled.Answers[answers.ProjectName]),
			Answers:  settled.Answers,
			Version:  env.Version,
			Template: env.Template,
			Environ:  env.Environ,
			GitHub:   github,
		})
	}
	return cmd
}

// flagName turns an answer key such as "user_name" into its flag, "user-name".
func flagName(key string) string {
	return strings.ReplaceAll(key, "_", "-")
}

// adviseNew words the fix for a problem with the answers sow new was given or
// asked for: the flag to pass, or the personal defaults file for a personal
// answer.
func adviseNew(p answers.Problem) error {
	switch p.Kind {
	case answers.EmptyAnswer, answers.MissingAnswer:
		personal := slices.ContainsFunc(answers.Questions(), func(q answers.Question) bool {
			return q.Key == p.Key && q.Personal
		})
		if personal {
			return fmt.Errorf("%w: pass --%s or set %s in the personal defaults file", p, flagName(p.Key), p.Key)
		}
		return fmt.Errorf("%w: pass --%s", p, flagName(p.Key))
	case answers.NeedsOff:
		var flags []string
		for _, stack := range p.NeededStacks {
			flags = append(flags, "--"+flagName(p.Needs)+" "+stack)
		}
		return fmt.Errorf("%w: pass %s", p, strings.Join(flags, " or "))
	}
	return p
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
