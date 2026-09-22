package render

import (
	"io/fs"
	"os"
)

// Source is a workflow tree a render reads, with the name to use when a
// message has to say where something came from.
//
// A checkout names a directory. An installed binary carries the tree with no
// directory to name, which is why the name is a field rather than a path the
// caller can always reconstruct.
type Source struct {
	FS   fs.FS
	Name string
}

// Dir reads the workflow from a directory, which is what --source names.
func Dir(path string) Source { return Source{FS: os.DirFS(path), Name: path} }

// Embedded reads a workflow the binary was built with, so a render needs no
// checkout to run in.
func Embedded(fsys fs.FS) Source {
	return Source{FS: fsys, Name: "the workflow this binary carries"}
}
