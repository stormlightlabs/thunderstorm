// Package thunderstorm carries the workflow source that tstorm reads at
// runtime.
//
// A binary installed with `go install` has no checkout to read, and installing
// the loop into another repository is the ordinary case rather than the rare
// one. So the tree travels with the binary: `tstorm install` renders from what
// is embedded here, and `--source` is for working on the loop itself.
//
// Dispatch reads the same tree. Claude Code provisions a subagent from a role
// definition and Codex spawns an agent; Pi has neither mechanism, so tstorm
// dispatch starts the session and needs the role's prompt and its tool list on
// a machine where no payload carries either.
package thunderstorm

import (
	"embed"
	"io/fs"
)

// source holds workflow/ as the render and dispatch read it. The all: prefix
// keeps files the toolchain would otherwise drop, which is what a skill's
// dot-prefixed reference would be.
//
//go:embed all:workflow
var source embed.FS

// Workflow is the canonical source tree, rooted where manifest.json sits.
func Workflow() fs.FS {
	sub, err := fs.Sub(source, "workflow")
	if err != nil {
		// The embed is a compile-time tree and the directory is its root, so
		// this cannot fail without the binary being built wrong.
		panic(err)
	}
	return sub
}
