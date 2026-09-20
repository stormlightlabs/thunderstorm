// Package buildinfo reports which build of tstorm is running.
package buildinfo

import "runtime/debug"

// Version is the released version, set at link time by the release build. A
// build without it falls back to what the module system recorded, so a binary
// from `go install` still names itself rather than reporting "dev".
var Version = ""

// Commit is the revision the binary was built from, set at link time.
var Commit = ""

// String returns a one-line description of this build.
func String() string {
	version, commit := Version, Commit
	if info, ok := debug.ReadBuildInfo(); ok {
		if version == "" {
			version = info.Main.Version
		}
		if commit == "" {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					commit = setting.Value
					break
				}
			}
		}
	}
	if version == "" {
		version = "(devel)"
	}
	if commit == "" {
		return version
	}
	if len(commit) > 12 {
		commit = commit[:12]
	}
	return version + " (" + commit + ")"
}
