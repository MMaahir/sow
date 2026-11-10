// Package sow holds the template that the sow CLI compiles into its binary.
package sow

import (
	"embed"
	"io/fs"
)

//go:embed all:template
var embedded embed.FS

// Template returns the template directories: base and one per component.
func Template() fs.FS {
	sub, err := fs.Sub(embedded, "template")
	if err != nil {
		// fs.Sub fails only on an invalid path, and "template" is a constant.
		panic(err)
	}
	return sub
}
