package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"charm.land/huh/v2"

	"github.com/MMaahir/sow/internal/answers"
)

// Prompter asks the author one question and returns the answer they gave.
// TerminalPrompter is the real one, and the tests supply a scripted one
// through Env.
type Prompter interface {
	Ask(ctx context.Context, q answers.Question) (string, error)
}

// TerminalPrompter asks questions with huh forms. In must be a terminal.
type TerminalPrompter struct {
	// In is where huh reads keys.
	In io.Reader
	// Out is where huh draws the form.
	Out io.Writer
}

// Ask shows q's help as the title. A question with choices is a select list
// and any other question is a text field. Either one starts at q's default.
// Ask returns huh.ErrUserAborted when the author presses Ctrl+C.
func (p TerminalPrompter) Ask(ctx context.Context, q answers.Question) (string, error) {
	value := q.Default
	var field huh.Field = huh.NewInput().Title(q.Help).Value(&value)
	if len(q.Choices) > 0 {
		field = huh.NewSelect[string]().Title(q.Help).Options(huh.NewOptions(q.Choices...)...).Value(&value)
	}
	err := huh.NewForm(huh.NewGroup(field)).WithInput(p.In).WithOutput(p.Out).RunWithContext(ctx)
	if err != nil {
		return "", err
	}
	return value, nil
}

// ask returns the answers the author types for every question that no source
// answers. It doesn't ask a question whose needed component is off, because
// that question can only be answered with its default. Validation happens
// later in answers.Resolve, as for a flag.
func ask(ctx context.Context, prompter Prompter, sources []answers.Answers) (answers.Answers, error) {
	typed := answers.Answers{}
	known := append(slices.Clone(sources), typed)
	// given returns the answer for key from the sources or from what the
	// author has typed so far.
	given := func(key string) (string, bool) {
		for _, source := range known {
			if value, ok := source[key]; ok {
				return value, true
			}
		}
		return "", false
	}
	var unasked []string
	for _, q := range answers.Questions() {
		if _, ok := given(q.Key); ok {
			continue
		}
		if needed, _ := given(q.Needs); needed == answers.None {
			continue
		}
		if prompter == nil {
			unasked = append(unasked, q.Key)
			continue
		}
		value, err := prompter.Ask(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("ask for %s: %w", q.Key, err)
		}
		typed[q.Key] = value
	}
	if len(unasked) > 0 {
		return nil, fmt.Errorf("stdin isn't a terminal, so sow can't ask for %s: "+
			"pass --no-input and give answers with flags, a preset or the personal defaults file",
			strings.Join(unasked, ", "))
	}
	return typed, nil
}
