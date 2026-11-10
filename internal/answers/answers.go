// Package answers defines the questions sow asks about a project and turns
// flags, presets and personal defaults into one validated set of answers.
package answers

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// ProjectName is the key of the answer that names the project.
const ProjectName = "project_name"

// None is the value of a component answer when the component is off.
const None = "none"

// Answers maps each question's key to its answer. The keys are the ones in the
// answers file, and templates read answers by the same keys.
type Answers map[string]string

// Question describes one answer. To add an answer, add a Question to the list
// in Questions. The flag, the validation, the answers file entry and the
// template value all come from that one entry.
type Question struct {
	// Key names the answer in the answers file and in templates. The flag is
	// the key with hyphens: "user_name" becomes --user-name.
	Key string
	// Help is the flag description and the prompt.
	Help string
	// Choices lists the allowed values. Empty means any text.
	Choices []string
	// Default is used when no source gives the answer. Empty means the answer
	// is required.
	Default string
	// Personal marks an answer the personal defaults file may set.
	Personal bool
	// Component marks an answer that turns a component on. Its value is the
	// stack, or None.
	Component bool
	// check reports why a value is invalid, or nil. It may be nil.
	check func(value string) error
}

// Questions returns every question in the order the answers file lists them.
func Questions() []Question {
	return []Question{
		{
			Key:   ProjectName,
			Help:  "Project name, used as the directory name.",
			check: checkProjectName,
		},
		{Key: "user_name", Help: "Your name, used as the project's author.", Personal: true},
		{Key: "user_email", Help: "Your email, used as the project's author email.", Personal: true},
		{Key: "github_user", Help: "Your GitHub username.", Personal: true},
		{
			Key:       "backend",
			Help:      "Backend stack.",
			Choices:   []string{None, "ts"},
			Default:   None,
			Component: true,
		},
	}
}

var projectNamePattern = regexp.MustCompile(`^[a-z]([a-z0-9-]{0,51}[a-z0-9])?$`)

// checkProjectName enforces the rule in docs/roadmap.md. The name becomes a
// Kubernetes namespace and a Helm release name, and Helm allows 53 characters.
func checkProjectName(value string) error {
	if !projectNamePattern.MatchString(value) {
		return errors.New("use lowercase letters, digits and hyphens, start with a letter, " +
			"end with a letter or digit, and use at most 53 characters")
	}
	return nil
}

// Resolve merges sources into one complete set of answers. Sources are listed
// from the highest priority to the lowest, and the first one that has a key
// wins. A question no source answers takes its default.
//
// The returned error lists every problem, not only the first: an unknown key,
// a missing required answer, and a value its question doesn't allow.
func Resolve(sources ...Answers) (Answers, error) {
	questions := Questions()
	resolved := Answers{}
	var problems []error

	for _, source := range sources {
		for key := range source {
			if !slices.ContainsFunc(questions, func(q Question) bool { return q.Key == key }) {
				problems = append(problems, fmt.Errorf("%s is not an answer sow knows", key))
			}
		}
	}

	for _, q := range questions {
		value := q.Default
		for _, source := range sources {
			if given, ok := source[q.Key]; ok {
				value = given
				break
			}
		}
		switch {
		case value == "" && q.Personal:
			problems = append(problems, fmt.Errorf(
				"%s is missing: pass --%s or set %s in the personal defaults file",
				q.Key, strings.ReplaceAll(q.Key, "_", "-"), q.Key))
		case value == "":
			problems = append(problems, fmt.Errorf(
				"%s is missing: pass --%s", q.Key, strings.ReplaceAll(q.Key, "_", "-")))
		case len(q.Choices) > 0 && !slices.Contains(q.Choices, value):
			problems = append(problems, fmt.Errorf(
				"%s can't be %q: choose one of %s", q.Key, value, strings.Join(q.Choices, ", ")))
		case q.check != nil && q.check(value) != nil:
			problems = append(problems, fmt.Errorf("%s can't be %q: %w", q.Key, value, q.check(value)))
		}
		resolved[q.Key] = value
	}

	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return resolved, nil
}

// Dirs returns the template directories these answers select: "base", then one
// directory per component that is on, named "<component>-<stack>". ADR 0012
// fixes this naming rule.
//
// A component with no answer is off. An answers file written by an older
// release has no entry for a component added since, and the old render must
// not pick that component up.
func (a Answers) Dirs() []string {
	dirs := []string{"base"}
	for _, q := range Questions() {
		stack := a[q.Key]
		if q.Component && stack != "" && stack != None {
			dirs = append(dirs, q.Key+"-"+stack)
		}
	}
	return dirs
}
