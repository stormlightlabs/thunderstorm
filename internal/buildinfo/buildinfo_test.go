package buildinfo

import "testing"

func TestABuildNamesATagAndTheCommitUnderIt(t *testing.T) {
	for name, tc := range map[string]struct {
		build Build
		want  string
	}{
		"a release":            {Build{Version: "v0.1.0", Released: true}, "v0.1.0"},
		"a release with a sha": {Build{Version: "v0.1.0", Revision: "998f9dabf3285012", Released: true}, "v0.1.0+g998f9da"},
		"a checkout":           {Build{Version: "v0.1.0-rc.1", Revision: "1969674364e7"}, "v0.1.0-rc.1+g1969674"},
		"uncommitted changes":  {Build{Version: "v0.1.0-rc.1", Revision: "1969674364e7", Modified: true}, "v0.1.0-rc.1+g1969674.dirty"},
		"nothing recorded":     {Build{Version: "v0.1.0-rc.1"}, "v0.1.0-rc.1"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := tc.build.String(); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// The module system records a version for every build. Only a tagged one
// answers for the build; a pseudo-version answers for the commit inside it.
func TestOnlyATaggedModuleVersionNamesTheBuild(t *testing.T) {
	for name, tc := range map[string]struct {
		version  string
		tag      string
		revision string
	}{
		"a tag":                               {"v0.1.0", "v0.1.0", ""},
		"a pseudo-version":                    {"v0.1.0-rc.1.0.20260922044137-1969674364e7", "", "1969674364e7"},
		"a dirty main module":                 {"v0.1.0-rc.1.0.20260922044137-1969674364e7+dirty", "", "1969674364e7"},
		"built outside a module":              {"(devel)", "", ""},
		"nothing at all":                      {"", "", ""},
		"a prerelease that is not a revision": {"v0.2.0-beta.1", "v0.2.0-beta.1", ""},
	} {
		t.Run(name, func(t *testing.T) {
			tag, revision := moduleBuild(tc.version)
			if tag != tc.tag || revision != tc.revision {
				t.Errorf("got (%q, %q), want (%q, %q)", tag, revision, tc.tag, tc.revision)
			}
		})
	}
}

// Release reads the binary running the test, which carries no link-time values
// and does come from a checkout.
func TestReleaseNamesTheRunningBuild(t *testing.T) {
	b := Release()
	if b.Version == "" {
		t.Error("the build names no version")
	}
	if b.Released {
		t.Error("a test binary reports itself released")
	}
}
