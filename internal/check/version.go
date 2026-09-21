package check

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Release is what a version check was given: where the workflow source and
// the rendered payloads live, and the tag being built where there is one.
type Release struct {
	Source   string
	Payloads string
	Tag      string
}

// Version reports why the payload's version is wrong for the tree it
// describes.
//
// A harness caches an installed plugin by version and reports it current
// while that string is unchanged, so a payload that changed without a bump
// never reaches a machine that already installed it.
func Version(r Release) ([]string, error) {
	version, err := manifestVersion(filepath.Join(r.Source, "manifest.json"))
	if err != nil {
		return nil, err
	}

	var findings []string
	if r.Tag != "" {
		if want := strings.TrimPrefix(r.Tag, "v"); want != version {
			findings = append(findings, fmt.Sprintf(
				"tag %s does not match version %s in %s/manifest.json", r.Tag, version, r.Source))
		}
	}

	tag, ok := lastTag()
	if !ok {
		return findings, nil
	}
	changed, ok := git("diff", "--name-only", tag, "--", r.Source, r.Payloads)
	if !ok || strings.TrimSpace(changed) == "" {
		return findings, nil
	}
	released, ok := taggedVersion(tag, r.Source)
	if !ok || released != version {
		return findings, nil
	}
	return append(findings, fmt.Sprintf(
		"the payload has changed since %s and still says %s; bump version in %s/manifest.json",
		tag, version, r.Source)), nil
}

func lastTag() (string, bool) {
	out, ok := git("describe", "--tags", "--abbrev=0", "--match", "v*")
	if !ok {
		return "", false
	}
	tag := strings.TrimSpace(out)
	return tag, tag != ""
}

func taggedVersion(tag, source string) (string, bool) {
	out, ok := git("show", tag+":"+filepath.ToSlash(filepath.Join(source, "manifest.json")))
	if !ok {
		return "", false
	}
	var m struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		return "", false
	}
	return m.Version, true
}

func manifestVersion(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var m struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if m.Version == "" {
		return "", fmt.Errorf("%s names no version", path)
	}
	return m.Version, nil
}
