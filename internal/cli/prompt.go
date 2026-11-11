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

// ask returns the answers the author types for the questions that
// answers.Settle lists as unanswered. It settles again after each answer, so
// a question whose needed component the author turned off isn't asked.
// Validation happens afterwards in answers.Settle, as for a flag.
func ask(ctx context.Context, prompter Prompter, sources []answers.Answers) (answers.Answers, error) {
	typed := answers.Answers{}
	known := append(slices.Clone(sources), typed)
	for {
		unanswered := answers.Settle(known...).Unanswered
		if len(unanswered) == 0 {
			return typed, nil
		}
		if prompter == nil {
			var keys []string
			for _, q := range unanswered {
				keys = append(keys, q.Key)
			}
			return nil, fmt.Errorf("stdin isn't a terminal, so sow can't ask for %s: "+
				"pass --no-input and give answers with flags, a preset or the personal defaults file",
				strings.Join(keys, ", "))
		}
		q := unanswered[0]
		value, err := prompter.Ask(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("ask for %s: %w", q.Key, err)
		}
		typed[q.Key] = value
	}
}
