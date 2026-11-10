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
// from the given sow version. Entries follow the order of Questions.
func Marshal(version string, a Answers) ([]byte, error) {
	entries := &yaml.Node{Kind: yaml.MappingNode}
	add := func(key, value string) {
		entries.Content = append(entries.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Value: value})
	}
	add(versionKey, version)
	for _, q := range Questions() {
		add(q.Key, a[q.Key])
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
