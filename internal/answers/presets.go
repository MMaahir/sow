package answers

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// presets maps each preset's name to the answers it fills in.
var presets = map[string]Answers{
	"minimal": {"backend": None, "frontend": None},
}

// PresetNames returns the names of every preset, sorted.
func PresetNames() []string {
	return slices.Sorted(maps.Keys(presets))
}

// Preset returns the answers the named preset fills in.
func Preset(name string) (Answers, error) {
	preset, ok := presets[name]
	if !ok {
		return nil, fmt.Errorf("there is no preset %q: choose one of %s",
			name, strings.Join(PresetNames(), ", "))
	}
	return maps.Clone(preset), nil
}
