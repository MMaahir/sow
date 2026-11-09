// Package render turns template directories and answers into the files of a
// project. It follows the template format in ADR 0012, which is a
// compatibility contract: sow update renders old releases with this code.
package render

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// templateSuffix marks a file that Render executes as a template. Render
// copies every other file byte for byte.
const templateSuffix = ".tmpl"

// File is one file of a rendered project.
type File struct {
	// Path is relative to the project root and uses forward slashes.
	Path    string
	Content []byte
}

// Render returns the project files that dirs produce, sorted by path. Each
// name in dirs is a directory at the root of tmpl, and Render lays them all
// over the same project root. A template reads an answer as [[ .key ]].
//
// Render fails if a directory is missing, if two directories contain the same
// path, or if a template uses an answer that data doesn't have.
func Render(tmpl fs.FS, dirs []string, data map[string]string) ([]File, error) {
	owner := map[string]string{}
	var files []File

	for _, dir := range dirs {
		err := fs.WalkDir(tmpl, dir, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			content, err := fs.ReadFile(tmpl, name)
			if err != nil {
				return err
			}
			target := strings.TrimPrefix(name, dir+"/")
			if strings.HasSuffix(target, templateSuffix) {
				target = strings.TrimSuffix(target, templateSuffix)
				if content, err = execute(name, content, data); err != nil {
					return err
				}
			}
			if other, taken := owner[target]; taken {
				return fmt.Errorf("%s is in both %s and %s", target, other, dir)
			}
			owner[target] = dir
			files = append(files, File{Path: target, Content: content})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("render the template: %w", err)
		}
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func execute(name string, content []byte, data map[string]string) ([]byte, error) {
	parsed, err := template.New(path.Base(name)).
		Delims("[[", "]]").
		Option("missingkey=error").
		Parse(string(content))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := parsed.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// Write writes files under root, creating directories as needed.
func Write(root string, files []File) error {
	for _, file := range files {
		target := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, file.Content, 0o644); err != nil {
			return err
		}
	}
	return nil
}
