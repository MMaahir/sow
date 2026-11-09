package render_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/MMaahir/sow/internal/render"
)

func TestRenderLaysEveryDirectoryOverTheProjectRoot(t *testing.T) {
	tmpl := fstest.MapFS{
		"base/README.md.tmpl":        {Data: []byte("# [[ .project_name ]]\n")},
		"backend-ts/backend/main.ts": {Data: []byte("serve()\n")},
		"frontend-react/skipped.txt": {Data: []byte("not selected\n")},
	}

	files, err := render.Render(tmpl, []string{"base", "backend-ts"}, map[string]string{"project_name": "demo"})
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]string{}
	for _, file := range files {
		got[file.Path] = string(file.Content)
	}
	want := map[string]string{"README.md": "# demo\n", "backend/main.ts": "serve()\n"}
	if len(got) != len(want) {
		t.Errorf("rendered %d files, want %d: %v", len(got), len(want), got)
	}
	for path, content := range want {
		if got[path] != content {
			t.Errorf("%s = %q, want %q", path, got[path], content)
		}
	}
}

func TestRenderCopiesAFileWithoutTheTemplateSuffixByteForByte(t *testing.T) {
	workflow := "run: echo ${{ github.sha }} [[ .project_name ]]\n"
	tmpl := fstest.MapFS{"base/ci.yml": {Data: []byte(workflow)}}

	files, err := render.Render(tmpl, []string{"base"}, map[string]string{"project_name": "demo"})
	if err != nil {
		t.Fatal(err)
	}

	if got := string(files[0].Content); got != workflow {
		t.Errorf("ci.yml = %q, want it unchanged", got)
	}
}

func TestRenderLeavesCurlyBracesAloneInATemplate(t *testing.T) {
	tmpl := fstest.MapFS{
		"base/ci.yml.tmpl": {Data: []byte("name: [[ .project_name ]]\nrun: echo ${{ github.sha }}\n")},
	}

	files, err := render.Render(tmpl, []string{"base"}, map[string]string{"project_name": "demo"})
	if err != nil {
		t.Fatal(err)
	}

	want := "name: demo\nrun: echo ${{ github.sha }}\n"
	if got := string(files[0].Content); got != want {
		t.Errorf("ci.yml = %q, want %q", got, want)
	}
}

func TestRenderFailures(t *testing.T) {
	tests := map[string]struct {
		tmpl fstest.MapFS
		dirs []string
		want string
	}{
		"two directories contain the same path": {
			tmpl: fstest.MapFS{
				"base/mise.toml":       {Data: []byte("a")},
				"backend-ts/mise.toml": {Data: []byte("b")},
			},
			dirs: []string{"base", "backend-ts"},
			want: "mise.toml is in both base and backend-ts",
		},
		"a template and a plain file render to the same path": {
			tmpl: fstest.MapFS{
				"base/mise.toml":            {Data: []byte("a")},
				"backend-ts/mise.toml.tmpl": {Data: []byte("b")},
			},
			dirs: []string{"base", "backend-ts"},
			want: "mise.toml is in both base and backend-ts",
		},
		"a template uses an answer that doesn't exist": {
			tmpl: fstest.MapFS{"base/README.md.tmpl": {Data: []byte("[[ .project_title ]]")}},
			dirs: []string{"base"},
			want: "project_title",
		},
		"a selected directory is missing": {
			tmpl: fstest.MapFS{"base/README.md": {Data: []byte("a")}},
			dirs: []string{"base", "backend-ts"},
			want: "backend-ts",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := render.Render(test.tmpl, test.dirs, map[string]string{"project_name": "demo"})
			if err == nil {
				t.Fatal("Render succeeded, want an error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error = %q, want it to contain %q", err, test.want)
			}
		})
	}
}
