package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MMaahir/sow/internal/answers"
)

// checkRenderable returns an error when sow update can't render the template
// at recorded, the project's sow_version, with a sow at version current. It
// can render only a release that isn't newer than current, because the old
// render comes from the release's tag, and only when current is a release.
// The error names the project by its project_name, project.
func checkRenderable(project, recorded, current string) error {
	if recorded == DevVersion {
		return fmt.Errorf("%s was created by sow %s, a development build, so no release exists to update from: "+
			"set sow_version in %s to the sow release whose template the project matches",
			project, recorded, answers.FileName)
	}
	old, ok := parseRelease(recorded)
	if !ok {
		return fmt.Errorf("%s records sow_version %q in %s, which isn't a sow release: "+
			"set it to the sow release that created the project, such as 0.1.0",
			project, recorded, answers.FileName)
	}
	// A development build of sow has no release, so it has no template to
	// render as the new one and can't compare its version with the project's.
	installed, ok := parseRelease(current)
	if !ok {
		return fmt.Errorf("this sow is a development build (%s), and sow update needs a release build of sow "+
			"at %s or later to update %s: install one with go install github.com/MMaahir/sow/cmd/sow@v%s, "+
			"then run sow update again",
			current, recorded, project, recorded)
	}
	for i := range old {
		if old[i] < installed[i] {
			return nil
		}
		if old[i] > installed[i] {
			return fmt.Errorf("%s is at sow %s, which is newer than this sow (%s): upgrade sow to %s or later and run sow update again",
				project, recorded, current, recorded)
		}
	}
	return nil
}

// parseRelease returns the three numbers of a release version such as 0.10.2,
// and 0.0.0 and false when version isn't three numbers separated by dots.
func parseRelease(version string) ([3]int, bool) {
	var numbers [3]int
	parts := strings.Split(version, ".")
	if len(parts) != len(numbers) {
		return [3]int{}, false
	}
	for i, part := range parts {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			return [3]int{}, false
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return [3]int{}, false
		}
		numbers[i] = n
	}
	return numbers, true
}
