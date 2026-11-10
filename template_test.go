package sow_test

import (
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/MMaahir/sow"
)

// templatePaths returns the path of every file in the embedded template.
func templatePaths(t *testing.T) []string {
	t.Helper()
	var paths []string
	err := fs.WalkDir(sow.Template(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			paths = append(paths, name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestTemplateHasNoDotEnv(t *testing.T) {
	for _, name := range templatePaths(t) {
		if base := path.Base(name); base == ".env" || base == ".env.tmpl" {
			t.Errorf("%s: the template never contains .env, only .env.example", name)
		}
	}
}

func TestTemplateKeepsPackageManifestsPlain(t *testing.T) {
	plain := []string{"package.json", "pnpm-workspace.yaml", "pnpm-lock.yaml"}
	for _, name := range templatePaths(t) {
		if base, isTemplate := strings.CutSuffix(path.Base(name), ".tmpl"); isTemplate && slices.Contains(plain, base) {
			t.Errorf("%s: %s must be a plain file so Renovate can parse it", name, base)
		}
	}
}

func TestTemplateHasNoGoMod(t *testing.T) {
	for _, name := range templatePaths(t) {
		if path.Base(name) == "go.mod" {
			t.Errorf("%s: Go's embed skips a directory that contains go.mod, so name it go.mod.tmpl", name)
		}
	}
}
