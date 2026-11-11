// Package answers defines the questions sow asks about a project and settles
// flags, presets, personal defaults and the answers file into one checked set
// of answers.
package answers

import (
	"errors"
	"fmt"
	"maps"
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
	// typeScript lists the choices that are TypeScript stacks. A project with
	// one of them gets the shared "typescript" template directory and needs
	// the global vp CLI from Vite+ (ADR 0011).
	typeScript []string
	// Needs is the key of a component answer that must be on for this one to
	// be on. Empty means this answer depends on no other. Settle reports a
	// NeedsOff problem when the rule is broken, and leaves the question out
	// of Unanswered while the needed component is off.
	//
	// It works for a component answer whose default is None and whose
	// needed question comes earlier in Questions.
	Needs string
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
			Key:        "backend",
			Help:       "Backend stack.",
			Choices:    []string{None, "ts"},
			Default:    None,
			Component:  true,
			typeScript: []string{"ts"},
		},
		{
			Key:        "frontend",
			Help:       "Frontend stack.",
			Choices:    []string{None, "react"},
			Default:    None,
			Component:  true,
			typeScript: []string{"react"},
			Needs:      "backend",
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

// ProblemKind names what is wrong with an answer.
type ProblemKind int

// The kinds of Problem that Settle reports.
const (
	// UnknownKey is a key that no question has.
	UnknownKey ProblemKind = iota + 1
	// EmptyAnswer is a source that gives an empty value. sow treats the
	// question as answered, so it never asks it.
	EmptyAnswer
	// NotAChoice is a value outside its question's choices.
	NotAChoice
	// FailsCheck is a value that fails its question's own check.
	FailsCheck
	// MissingAnswer is a required question that no source answers.
	MissingAnswer
	// NeedsOff is a component that is on while the component it needs is
	// off.
	NeedsOff
)

// Problem is one thing wrong with a set of answers. It holds the facts a
// command needs to word its advice, and no advice of its own, because the
// fix depends on the command: sow new takes flags, and sow update takes an
// edit to the answers file.
type Problem struct {
	// Key is the key of the answer the problem is about.
	Key string
	// Kind says what is wrong.
	Kind ProblemKind
	// Value is the bad value, for NotAChoice and FailsCheck.
	Value string
	// Choices lists the allowed values, for NotAChoice.
	Choices []string
	// Reason is why the value fails its question's check, for FailsCheck.
	Reason string
	// Needs is the key of the component that must be on, for NeedsOff.
	Needs string
	// NeededStacks lists the stacks that turn the Needs component on, for
	// NeedsOff.
	NeededStacks []string
	// Askable reports that a question no source answers causes the
	// problem, so asking it may fix the problem. That is a MissingAnswer,
	// or a NeedsOff whose needed component took its default.
	Askable bool
}

// Error states the problem without the fix. The one exception is a value
// that isn't allowed: the message names the choices, or the reason its
// question's check gives, because that holds for every command.
func (p Problem) Error() string {
	switch p.Kind {
	case UnknownKey:
		return p.Key + " is not an answer sow knows"
	case EmptyAnswer:
		return p.Key + " can't be empty"
	case NotAChoice:
		return fmt.Sprintf("%s can't be %q: choose one of %s", p.Key, p.Value, strings.Join(p.Choices, ", "))
	case FailsCheck:
		return fmt.Sprintf("%s can't be %q: %s", p.Key, p.Value, p.Reason)
	case MissingAnswer:
		return p.Key + " is missing"
	case NeedsOff:
		return fmt.Sprintf("%s needs a %s", p.Key, p.Needs)
	}
	return p.Key + " has an unknown problem"
}

// Settled is the result of Settle.
type Settled struct {
	// Answers holds an answer for every question. A question no source
	// answers takes its default, or is empty when it has none. The answers
	// are fit to render only when Problems is empty.
	Answers Answers
	// Problems lists every problem, in the order Settle finds them.
	Problems []Problem
	// Unanswered lists the questions that no source answers and that sow
	// may ask, in the order of Questions. A question whose needed component
	// is off isn't one, because only its default fits.
	Unanswered []Question
}

// Settle settles the answers that sources give into one answer per question.
// Sources are listed from the highest priority to the lowest, and the first
// source that has a key wins. A question no source answers takes its
// default.
//
// Settle reports every problem, not only the first: an unknown key and an
// empty or invalid value in any source, even one that a higher source
// overrides, a required question no source answers, and a component that is
// on while the component it needs is off. It is the one place that applies
// these rules, so sow new and sow update can't disagree on them.
func Settle(sources ...Answers) Settled {
	questions := Questions()
	settled := Settled{Answers: Answers{}}
	for _, source := range sources {
		for _, key := range slices.Sorted(maps.Keys(source)) {
			if problem, ok := checkValue(questions, key, source[key]); ok {
				settled.Problems = append(settled.Problems, problem)
			}
		}
	}

	answered := map[string]bool{}
	for _, q := range questions {
		value := q.Default
		for _, source := range sources {
			if given, ok := source[q.Key]; ok {
				value = given
				answered[q.Key] = true
				break
			}
		}
		if !answered[q.Key] && value == "" {
			settled.Problems = append(settled.Problems, Problem{Key: q.Key, Kind: MissingAnswer, Askable: true})
		}
		settled.Answers[q.Key] = value
	}

	for _, q := range questions {
		needsOff := q.Needs != "" && settled.Answers[q.Needs] == None
		if !answered[q.Key] && !needsOff {
			settled.Unanswered = append(settled.Unanswered, q)
		}
		if needsOff && settled.Answers[q.Key] != None && settled.Answers[q.Key] != "" {
			settled.Problems = append(settled.Problems, Problem{
				Key:          q.Key,
				Kind:         NeedsOff,
				Needs:        q.Needs,
				NeededStacks: stacks(questions, q.Needs),
				Askable:      !answered[q.Needs],
			})
		}
	}
	return settled
}

// checkValue returns the problem with value as the answer to key, and false
// when there is none.
func checkValue(questions []Question, key, value string) (Problem, bool) {
	i := slices.IndexFunc(questions, func(q Question) bool { return q.Key == key })
	if i < 0 {
		return Problem{Key: key, Kind: UnknownKey}, true
	}
	q := questions[i]
	if value == "" {
		return Problem{Key: key, Kind: EmptyAnswer}, true
	}
	if len(q.Choices) > 0 && !slices.Contains(q.Choices, value) {
		return Problem{Key: key, Kind: NotAChoice, Value: value, Choices: q.Choices}, true
	}
	if q.check == nil {
		return Problem{}, false
	}
	if err := q.check(value); err != nil {
		return Problem{Key: key, Kind: FailsCheck, Value: value, Reason: err.Error()}, true
	}
	return Problem{}, false
}

// stacks returns the choices of the question with key that turn its
// component on.
func stacks(questions []Question, key string) []string {
	var on []string
	for _, q := range questions {
		if q.Key != key {
			continue
		}
		for _, choice := range q.Choices {
			if choice != None {
				on = append(on, choice)
			}
		}
	}
	return on
}

// Dirs returns the template directories these answers select: "base", then one
// directory per component that is on, named "<component>-<stack>", then
// "typescript" if any of those stacks is a TypeScript stack. ADR 0012 fixes
// these rules.
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
	if a.TypeScriptOn() {
		dirs = append(dirs, "typescript")
	}
	return dirs
}

// WithComponentsOff returns a copy of these answers in which each component
// without an answer is None. An answers file written by an older release has
// no entry for a component added since, and a template that reads that
// component's answer needs a value for it.
func (a Answers) WithComponentsOff() Answers {
	filled := Answers{}
	maps.Copy(filled, a)
	for _, q := range Questions() {
		if _, ok := filled[q.Key]; q.Component && !ok {
			filled[q.Key] = None
		}
	}
	return filled
}

// TypeScriptOn reports whether these answers turn on a component with a
// TypeScript stack, which needs Vite+ (ADR 0011).
func (a Answers) TypeScriptOn() bool {
	for _, q := range Questions() {
		if slices.Contains(q.typeScript, a[q.Key]) {
			return true
		}
	}
	return false
}
