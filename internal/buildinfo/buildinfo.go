// Package buildinfo reports which build of tstorm is running.
package buildinfo

import (
	"runtime/debug"
	"strings"
)

// BaseVersion is the tag this tree descends from, and what a build names when
// no release stamped it. It has to move with the tag, so `tstorm check version`
// compares the two and fails a release that left it behind.
const BaseVersion = "v0.1.0-rc.1"

// Version is the released version, set at link time by the release build.
//
// The symbol path is what -X has to match, and a wrong one is silently a
// no-op:
//
//	-X github.com/stormlightlabs/thunderstorm/internal/buildinfo.Version=v0.1.0
//	-X github.com/stormlightlabs/thunderstorm/internal/buildinfo.Commit=$(git rev-parse HEAD)
//
// Check a release build with `tstorm version` before publishing it.
var Version = ""

// Commit is the revision the binary was built from, set at link time. See
// Version for the symbol path.
var Commit = ""

// Build is what a binary can say about itself: which tag it answers to, which
// commit it came from, and whether either was recorded or inferred.
type Build struct {
	Version  string
	Revision string
	Modified bool
	Released bool
}

// Release reads this build's identity from the link-time values, then from
// what the toolchain recorded.
//
// A build from a checkout carries vcs.revision. A build from the module proxy
// carries none, because there was no repository to read, and names the commit
// in its pseudo-version instead.
func Release() Build {
	b := Build{Version: Version, Revision: Commit, Released: Version != ""}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		if b.Version == "" {
			b.Version = BaseVersion
		}
		return b
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if b.Revision == "" {
				b.Revision = setting.Value
			}
		case "vcs.modified":
			b.Modified = setting.Value == "true"
		}
	}
	// A checkout build answers to BaseVersion: the module version it also
	// carries is a pseudo-version naming the same commit at more length.
	if b.Version == "" && b.Revision == "" {
		b.Version, b.Revision = moduleBuild(info.Main.Version)
	}
	if b.Version == "" {
		b.Version = BaseVersion
	}
	return b
}

// String names the build as a tag and the commit under it, so two builds of
// the same tag are told apart:
//
//	v0.1.0                     a release
//	v0.1.0-rc.1+g8591bc0       built from a checkout at that commit
//	v0.1.0-rc.1+g8591bc0.dirty the same, with uncommitted changes
func (b Build) String() string {
	out := b.Version
	if b.Revision != "" {
		out += "+g" + short(b.Revision)
	}
	if b.Modified {
		out += ".dirty"
	}
	return out
}

// String returns a one-line description of this build.
func String() string { return Release().String() }

// moduleBuild reads what the module system recorded. A tagged version is the
// answer itself; a pseudo-version names a commit that no tag covers, so only
// its revision is worth keeping.
func moduleBuild(version string) (tag, revision string) {
	// Build metadata is the toolchain's, not the tag's: it appends +dirty to
	// the version it derived for a main module with uncommitted changes.
	version, _, _ = strings.Cut(version, "+")
	if version == "" || version == "(devel)" {
		return "", ""
	}
	if last := version[strings.LastIndex(version, "-")+1:]; isRevision(last) {
		return "", last
	}
	return version, ""
}

// isRevision reports whether a pseudo-version's last field is the abbreviated
// commit the module system appends, which is always twelve hex digits.
func isRevision(s string) bool {
	if len(s) != 12 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// short abbreviates a revision to what `git describe` prints beside a tag.
func short(revision string) string {
	if len(revision) > 7 {
		return revision[:7]
	}
	return revision
}
