package answers

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// FileName is the name of the answers file at the root of a generated project.
const FileName = ".sow-answers.yml"

// versionKey is the answers file entry that records the sow version.
const versionKey = "sow_version"

// Marshal returns the contents of the answers file for a project rendered
// from the given sow version. Entries follow the order of Questions. A
// question a doesn't answer has no entry, so an answers file that an older
// release wrote keeps its meaning when sow update rewrites it.
func Marshal(version string, a Answers) ([]byte, error) {
	entries := &yaml.Node{Kind: yaml.MappingNode}
	add := func(key, value string) {
		entries.Content = append(entries.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Value: value})
	}
	add(versionKey, version)
	for _, q := range Questions() {
		if value, ok := a[q.Key]; ok {
			add(q.Key, value)
		}
	}

	var out bytes.Buffer
	out.WriteString("# Written by sow. Don't edit it by hand.\n")
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(entries); err != nil {
		return nil, fmt.Errorf("encode the answers file: %w", err)
	}
	return out.Bytes(), nil
}

// Read reads the answers file at path and returns the sow version it records
// and its answers. It settles the answers with Settle, the function sow new
// uses, and words each problem's fix for sow update. The returned error wraps
// fs.ErrNotExist when the file is missing and names the file when it isn't
// valid YAML. Otherwise it lists every problem with the contents: a missing
// sow_version, an unknown key, an invalid answer, a missing answer that isn't
// a component, and a component that is on while the component it needs is
// off.
//
// A component the file doesn't answer is off, because an answers file
// written by an older release has no entry for a component added since (ADR
// 0012). Read returns the answers as the file records them, without that
// component, so sow update writes the file back with the same entries.
func Read(path string) (string, Answers, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("read the answers file: %w", err)
	}
	recorded := Answers{}
	if err := yaml.Unmarshal(content, &recorded); err != nil {
		return "", nil, fmt.Errorf("read the answers file %s: %w", path, err)
	}
	version := recorded[versionKey]
	delete(recorded, versionKey)
	var problems []error
	if version == "" {
		problems = append(problems, fmt.Errorf(
			"%s is missing: set it to the sow version that created the project", versionKey))
	}
	for _, problem := range Settle(recorded.WithComponentsOff()).Problems {
		problems = append(problems, adviseUpdate(problem))
	}
	if err := errors.Join(problems...); err != nil {
		return "", nil, fmt.Errorf("check the answers file %s: %w", path, err)
	}
	return version, recorded, nil
}

// adviseUpdate words the fix for a problem in the answers file. The author
// fixes the file by hand, commits it and runs sow update again, so the
// advice never names a flag.
func adviseUpdate(p Problem) error {
	const again = "commit the file, then run sow update again"
	switch p.Kind {
	case EmptyAnswer, MissingAnswer:
		return fmt.Errorf("%s is missing from %s: add it with the value the project was created with, %s",
			p.Key, FileName, again)
	case NeedsOff:
		return fmt.Errorf("%w: set %s to %s in %s, %s",
			p, p.Needs, strings.Join(p.NeededStacks, " or "), FileName, again)
	}
	return p
}

// ReadPersonalDefaults reads the personal defaults file at path. A missing
// file gives no answers and no error. A key that isn't a personal default is
// an error, so a typo doesn't go unnoticed. The error lists every such key.
func ReadPersonalDefaults(path string) (Answers, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Answers{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the personal defaults file: %w", err)
	}

	defaults := Answers{}
	if err := yaml.Unmarshal(content, &defaults); err != nil {
		return nil, fmt.Errorf("read the personal defaults file %s: %w", path, err)
	}
	var personal []string
	for _, q := range Questions() {
		if q.Personal {
			personal = append(personal, q.Key)
		}
	}
	var problems []error
	for _, key := range slices.Sorted(maps.Keys(defaults)) {
		if !slices.Contains(personal, key) {
			problems = append(problems, fmt.Errorf(
				"the personal defaults file %s sets %s, but it can only set %s",
				path, key, strings.Join(personal, ", ")))
		}
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return defaults, nil
}
