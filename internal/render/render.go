package render

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// marker names the file every payload carries: the target on the first line,
// then every path the render wrote. A later render replaces a directory only
// when the marker names its target and the directory holds nothing the marker
// does not. Naming the target alone was not enough, because the committed
// payload carries a marker and travels with any copy of it.
const marker = ".tstorm-payload"

// The source names two directories it cannot spell itself. rootToken is the
// harness's own directory inside the repository being worked on, which is where
// a worktree and a .gitignore live. pluginToken is where the installed payload
// landed, which is where the scripts and the agent definitions live. They were
// one token until a plugin install put the payload outside the repository.
const (
	rootToken   = "{{ROOT}}"
	pluginToken = "{{PLUGIN}}"
)

// leftoverToken catches a token the target has no value for, which would
// otherwise reach a reader as literal braces.
var leftoverToken = regexp.MustCompile(`{{[A-Z_]+}}`)

// File is one file of a payload, at a slash-separated path relative to the
// payload root.
type File struct {
	Path string
	Body []byte
	Mode fs.FileMode
	// Seed is written where the destination has no such file and left alone
	// where it does. settings.json is the only one: what it needs from the
	// payload is merged into whatever the repository already keeps there.
	Seed bool
}

// Payload is a rendered harness payload held in memory. Nothing reaches the
// filesystem until Write, so a render that fails halfway leaves no half-payload
// behind.
type Payload struct {
	Target *Target
	Files  []File

	// Version is the manifest version this payload was built from, recorded
	// in the marker so an installed payload can say what it is and update
	// can say what it moved from.
	Version string
	// Limits is what this harness does not get, where the render went ahead
	// anyway: an unmet artifact stops a render, a missing permission
	// mechanism does not. The render report is where an operator meets it.
	Limits []string

	counts map[Kind]int
}

// Unmet reports artifacts a target cannot carry. It is the loud failure: the
// render writes nothing, and the message names every gap at once rather than
// stopping at the first.
type Unmet struct {
	Target string
	Gaps   []Gap
}

// Gap is one artifact's unmet requirements.
type Gap struct {
	Artifact string
	Kind     Kind
	Reasons  []string
}

// Error groups the gaps by reason rather than by artifact, because one missing
// capability usually stops a dozen artifacts and the operator needs the reason
// once, with the list of what it costs.
func (u *Unmet) Error() string {
	var reasons []string
	blocked := map[string][]string{}
	for _, g := range u.Gaps {
		for _, r := range g.Reasons {
			if _, seen := blocked[r]; !seen {
				reasons = append(reasons, r)
			}
			blocked[r] = append(blocked[r], fmt.Sprintf("%s %s", g.Kind, g.Artifact))
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s cannot carry %d of the workflow's artifacts:", u.Target, len(u.Gaps))
	for _, r := range reasons {
		fmt.Fprintf(&b, "\n\n  %s\n    %s", r, strings.Join(blocked[r], ", "))
	}
	b.WriteString("\n\nNothing was written. A payload that installs and then skips part of the")
	b.WriteString("\nworkflow is worse than no payload, so the gaps close before the target does.")
	return b.String()
}

// Plan builds the payload for one target from a source tree. It returns an
// *Unmet when the target cannot carry every artifact.
func Plan(m Manifest, t *Target, src Source) (*Payload, error) {
	p := &Payload{Target: t, Version: m.Version, counts: map[Kind]int{}}

	// Every gap is collected before anything is rendered, so a target that
	// cannot carry the workflow says so once, in full, rather than failing on
	// whichever artifact happened to come first.
	var gaps []Gap
	var present []Artifact
	for _, a := range m.Artifacts {
		if reasons := t.unmet(a); len(reasons) > 0 {
			gaps = append(gaps, Gap{Artifact: a.Name, Kind: a.Kind, Reasons: reasons})
			continue
		}
		present = append(present, a)
	}
	if len(gaps) > 0 {
		return nil, &Unmet{Target: t.Name, Gaps: gaps}
	}

	for _, a := range present {
		files, err := p.render(a, t, src)
		if err != nil {
			return nil, err
		}
		p.Files = append(p.Files, files...)
		p.counts[a.Kind]++
	}

	if t.extras != nil {
		extras, err := t.extras(m, present)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t.Name, err)
		}
		p.Files = append(p.Files, extras...)
	}
	if t.limits != nil {
		p.Limits = append(p.Limits, t.limits(present)...)
	}
	if !t.provides[CapPermissions] {
		p.Limits = append(p.Limits, fmt.Sprintf("nothing stops the denied commands here: %s",
			t.reason(CapPermissions)))
	}
	// The marker lists one path per line, so a path carrying a newline would
	// split into two entries and every later render would refuse the payload.
	for _, f := range p.Files {
		if strings.ContainsAny(f.Path, "\n\r") {
			return nil, fmt.Errorf("payload path %q carries a newline", f.Path)
		}
	}
	slices.SortFunc(p.Files, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	p.Files = append(p.Files, File{Path: marker, Body: p.markerBody(nil)})
	slices.SortFunc(p.Files, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	return p, nil
}

// versionLine prefixes the version in the marker's header, so a reader can
// tell it from the paths under it and an older one can ignore it.
const versionLine = "version "

// markerBody is the target, the version that wrote the payload, and every file
// the render writes, one per line. Paths in skip were not written and are left
// out.
func (p *Payload) markerBody(skip map[string]bool) []byte {
	var b strings.Builder
	b.WriteString(p.Target.Name)
	b.WriteString("\n")
	if p.Version != "" {
		b.WriteString(versionLine)
		b.WriteString(p.Version)
		b.WriteString("\n")
	}
	for _, f := range p.Files {
		if f.Path != marker && !skip[f.Path] {
			b.WriteString(f.Path)
			b.WriteString("\n")
		}
	}
	return []byte(b.String())
}

// render copies one artifact into payload files, substituting the resource
// root in prose.
func (p *Payload) render(a Artifact, t *Target, src Source) ([]File, error) {
	dir := t.dirs[a.Kind]
	sources, err := a.files(src)
	if err != nil {
		return nil, err
	}

	var out []File
	for _, name := range sources {
		body, err := fs.ReadFile(src.FS, name)
		if err != nil {
			return nil, fmt.Errorf("artifact %q: %w", a.Name, err)
		}
		if strings.HasSuffix(name, ".md") {
			body = bytes.ReplaceAll(body, []byte(rootToken), []byte(t.Root))
			if pluginRoot := t.pluginRoot(a); pluginRoot != "" {
				body = bytes.ReplaceAll(body, []byte(pluginToken), []byte(pluginRoot))
			}
			if found := leftoverToken.Find(body); found != nil {
				return nil, fmt.Errorf("artifact %q: %s names %s, which %s has no value for",
					a.Name, name, found, t.Name)
			}
		}
		if t.transform != nil {
			body, err = t.transform(a, body)
			if err != nil {
				return nil, err
			}
		}
		out = append(out, File{Path: p.destination(a, dir, name), Body: body, Mode: a.mode()})
	}
	return out, nil
}

// destination places one source file in the payload. A skill keeps its
// directory and everything under it; everything else becomes one file named
// for the command, agent, hook, or script that the harness will look up.
func (p *Payload) destination(a Artifact, dir, src string) string {
	if a.Kind == KindSkill {
		rel := strings.TrimPrefix(src, a.Source+"/")
		return path.Join(dir, a.Name, rel)
	}
	if a.Kind == KindCommand {
		return path.Join(dir, a.Name+".md")
	}
	if a.Kind == KindAgent {
		ext := ".md"
		if p.Target.Name == "codex" {
			ext = ".toml"
		}
		return path.Join(dir, a.Name+ext)
	}
	return path.Join(dir, a.Name)
}

// executable reports whether a mode carries any execute bit, which is the
// whole of what git stores and therefore the whole of what a payload promises.
// A zero mode is a file the renderer generated, which is never executable.
func executable(mode fs.FileMode) bool { return mode&0o111 != 0 }

// DropSeeds removes the files the payload would only seed, for a caller that
// writes them itself.
//
// install merges the deny rules and the hook registrations into the settings
// file rather than seeding one, and a payload that also wrote that file would
// claim it: the marker would list a path the merge then edits, and every later
// check would report it stale.
func (p *Payload) DropSeeds() {
	kept := p.Files[:0]
	for _, f := range p.Files {
		if !f.Seed {
			kept = append(kept, f)
		}
	}
	p.Files = kept
}

// Write puts the payload at dir and reports the files it left alone and the
// ones replace overwrote.
//
// It builds the whole tree beside dir and swaps it in, so a render either
// replaces the payload or leaves the previous one untouched. Nothing is
// deleted file by file: an earlier version of this wrote each file in place
// and then removed whatever it had not written, which deleted a git
// repository that happened to hold a marker file.
func (p *Payload) Write(dir string, replace bool) (kept, replaced []string, err error) {
	dir, kept, replaced, err = p.claim(dir, replace)
	if err != nil {
		return nil, nil, err
	}

	// A sibling of the destination, so the rename that swaps it in stays on
	// one filesystem. A render killed outright leaves one behind; it is named
	// so it is recognisable, and no later render reads it.
	held := map[string]bool{}
	for _, rel := range kept {
		held[rel] = true
	}

	staging, err := os.MkdirTemp(filepath.Dir(dir), "."+filepath.Base(dir)+".tstorm-staging-")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(staging)

	skipped := map[string]bool{}
	for _, f := range p.Files {
		if f.Seed && held[f.Path] {
			skipped[f.Path] = true
		}
	}
	for _, f := range p.Files {
		if skipped[f.Path] {
			continue
		}
		if f.Path == marker {
			f.Body = p.markerBody(skipped)
		}
		dest := filepath.Join(staging, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, nil, err
		}
		mode := f.Mode
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(dest, f.Body, mode); err != nil {
			return nil, nil, err
		}
		// WriteFile applies the mode only when it creates the file, and a
		// staged file is always new, but an inherited umask still narrows it.
		if err := os.Chmod(dest, mode); err != nil {
			return nil, nil, err
		}
	}

	previous, err := os.MkdirTemp(filepath.Dir(dir), "."+filepath.Base(dir)+".tstorm-previous-")
	if err != nil {
		return nil, nil, err
	}
	// MkdirTemp made the directory; rename needs the name free.
	if err := os.Remove(previous); err != nil {
		return nil, nil, err
	}
	swapped := false
	if err := os.Rename(dir, previous); err == nil {
		swapped = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, err
	}
	if err := os.Rename(staging, dir); err != nil {
		if swapped {
			os.Rename(previous, dir)
		}
		return nil, nil, err
	}
	if err := carry(previous, dir, kept); err != nil {
		return nil, nil, fmt.Errorf("%w; the files this render did not write are in %s", err, previous)
	}
	return kept, replaced, os.RemoveAll(previous)
}

// carry moves the files the render does not own from the replaced directory
// into the new one. Moved, not copied: a .claude can hold worktrees.
func carry(from, to string, paths []string) error {
	for _, rel := range paths {
		src := filepath.Join(from, filepath.FromSlash(rel))
		dest := filepath.Join(to, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.Rename(src, dest); err != nil {
			return err
		}
	}
	return nil
}

// claim decides whether dir may be replaced, and returns the path to
// replace, the files in it the payload does not write, and the ones a
// collision lets replace overwrite.
//
// A marker in dir says which files a previous render of this payload wrote;
// every other file there is the repository's, whether or not a marker is
// present at all. A path the payload wants to write that dir already holds,
// and the marker does not own, is a collision: refused unless replace is
// set, in which case it is treated like the marker's own files and
// overwritten.
func (p *Payload) claim(dir string, replace bool) (string, []string, []string, error) {
	resolved, err := filepath.EvalSymlinks(dir)
	if err == nil {
		dir = resolved
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", nil, nil, err
	}

	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return dir, nil, nil, os.MkdirAll(dir, 0o755)
	case err != nil:
		return "", nil, nil, err
	case len(entries) == 0:
		return dir, nil, nil, nil
	}

	writing := map[string]bool{}
	seeds := map[string]bool{}
	// dirs holds every path a payload file is nested under, so a symlink or a
	// plain file sitting at one of those paths can be told apart from a real
	// directory before anything is staged.
	dirs := map[string]bool{}
	for _, f := range p.Files {
		writing[f.Path] = true
		if f.Seed {
			seeds[f.Path] = true
		}
		parts := strings.Split(f.Path, "/")
		for i := 1; i < len(parts); i++ {
			dirs[strings.Join(parts[:i], "/")] = true
		}
	}

	owned := map[string]bool{}
	held, err := os.ReadFile(filepath.Join(dir, marker))
	switch {
	case err == nil:
		name, _, paths := parseMarker(held)
		if name != p.Target.Name {
			return "", nil, nil, fmt.Errorf("%s holds the %s payload, not %s; write this one somewhere else", dir, name, p.Target.Name)
		}
		owned[marker] = true
		for _, path := range paths {
			owned[path] = true
		}
	case !errors.Is(err, fs.ErrNotExist):
		return "", nil, nil, err
	}

	var kept, collisions []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(rel)
		if d.IsDir() {
			if owned[slash] {
				return nil
			}
			// A directory sitting where the payload writes a plain file
			// blocks that write the same way a plain file below blocks a
			// directory: caught here, or carry finds out only after the
			// swap, trying to rename a kept path onto what the new payload
			// just created there. Its contents are never carried: they sit
			// under a path the payload is about to replace outright.
			if writing[slash] {
				collisions = append(collisions, slash)
				return fs.SkipDir
			}
			return nil
		}
		if owned[slash] || slash == marker {
			return nil
		}
		switch {
		// A symlink or a plain file where the payload needs a directory to
		// hold nested paths is the mirror case: WalkDir does not descend
		// into it, so it would otherwise be carried back whole, onto a real
		// directory the swap just put in its place.
		case dirs[slash]:
			collisions = append(collisions, slash)
		case writing[slash] && !seeds[slash]:
			collisions = append(collisions, slash)
		default:
			kept = append(kept, slash)
		}
		return nil
	})
	if err != nil {
		return "", nil, nil, err
	}
	slices.Sort(collisions)
	if len(collisions) > 0 && !replace {
		return "", nil, nil, claimError(dir, collisions)
	}
	slices.Sort(kept)
	return dir, kept, collisions, nil
}

// claimError names every path the payload writes that dir already holds
// without owning, capped so a very long list still fits a terminal.
func claimError(dir string, paths []string) error {
	const shown = 10
	list, more := paths, 0
	if len(list) > shown {
		list, more = list[:shown], len(list)-shown
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s holds %d %s this payload also writes:\n", dir, len(paths), plural("path", len(paths)))
	for _, path := range list {
		fmt.Fprintf(&b, "  %s\n", path)
	}
	if more > 0 {
		fmt.Fprintf(&b, "  … and %d more\n", more)
	}
	b.WriteString("\nRun again with --replace to overwrite them, or move them aside.")
	return errors.New(b.String())
}

// Diff reports how the payload at dir differs from this one, so a check can
// tell whether the committed output still matches the source. An empty result
// means they agree.
//
// A file the marker does not list is the repository's, and is reported in
// left rather than as a difference.
func (p *Payload) Diff(dir string) (diff, left []string, err error) {
	var out []string
	owned, err := rendered(dir)
	if err != nil {
		return nil, nil, err
	}
	seen := map[string]bool{}
	// A seed the marker does not list is the repository's, edits and all.
	skipped := map[string]bool{}
	for _, f := range p.Files {
		if f.Seed && owned != nil && !owned[f.Path] {
			skipped[f.Path] = true
		}
	}
	for _, f := range p.Files {
		if skipped[f.Path] {
			continue
		}
		if f.Path == marker {
			f.Body = p.markerBody(skipped)
		}
		seen[f.Path] = true
		full := filepath.Join(dir, filepath.FromSlash(f.Path))
		body, err := os.ReadFile(full)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			out = append(out, "missing: "+f.Path)
			continue
		case err != nil:
			return nil, nil, err
		case !bytes.Equal(body, f.Body):
			out = append(out, "stale: "+f.Path)
		}
		// A check that reads only bytes certifies a script whose executable
		// bit a checkout dropped, and the skills invoke those by path.
		info, err := os.Stat(full)
		if err != nil {
			return nil, nil, err
		}
		// Only the executable bit. Git records nothing else, so a clone made
		// under a umask of 027 hands every file 0640 and a check comparing
		// full permissions fails on a payload nobody touched.
		if executable(info.Mode()) != executable(f.Mode) {
			want := "not executable"
			if executable(f.Mode) {
				want = "executable"
			}
			out = append(out, fmt.Sprintf("mode %04o, want %s: %s", info.Mode().Perm(), want, f.Path))
		}
	}
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(rel)
		if seen[slash] {
			return nil
		}
		if owned != nil && !owned[slash] {
			left = append(left, slash)
			return nil
		}
		out = append(out, "unexpected: "+slash)
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, err
	}
	slices.Sort(out)
	slices.Sort(left)
	return out, left, nil
}

// rendered reads the marker at dir and reports the paths it lists, the marker
// included, or nil where there is no marker.
func rendered(dir string) (map[string]bool, error) {
	held, err := os.ReadFile(filepath.Join(dir, marker))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	owned := map[string]bool{marker: true}
	_, _, paths := parseMarker(held)
	for _, path := range paths {
		owned[path] = true
	}
	return owned, nil
}

// parseMarker reads a marker into the target that wrote it, the version it
// wrote, and the paths it owns.
//
// A marker written before the version line carries none, and reads as an
// unknown version rather than an error: every payload installed until now is
// one of those.
func parseMarker(body []byte) (target, version string, paths []string) {
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	target = strings.TrimSpace(lines[0])
	rest := lines[1:]
	if len(rest) > 0 && strings.HasPrefix(rest[0], versionLine) {
		version = strings.TrimSpace(strings.TrimPrefix(rest[0], versionLine))
		rest = rest[1:]
	}
	for _, line := range rest {
		if path := strings.TrimSpace(line); path != "" {
			paths = append(paths, path)
		}
	}
	return target, version, paths
}

// Installed is the payload a directory holds.
type Installed struct {
	Target  string
	Version string
	Files   []string
}

// Remove deletes the files the payload at dir owns, and reports what it
// removed.
//
// The marker is the record of ownership, so a file the repository put in the
// payload directory is left where it is, and a directory is removed only once
// nothing is left in it. A payload file the repository has since edited is
// still the payload's, and goes: what a repository wants to keep belongs
// outside a directory a render owns.
func Remove(dir string) ([]string, error) {
	held, ok, err := Read(dir)
	if err != nil || !ok {
		return nil, err
	}

	var removed []string
	for _, rel := range held.Files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		switch err := os.Remove(path); {
		case err == nil:
			removed = append(removed, rel)
		case errors.Is(err, fs.ErrNotExist):
		default:
			return removed, err
		}
	}
	if err := os.Remove(filepath.Join(dir, marker)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return removed, err
	}
	slices.Sort(removed)
	return removed, pruneEmpty(dir)
}

// pruneEmpty removes every directory under dir that the removal emptied, and
// dir itself when nothing is left there. A repository's own file anywhere
// inside keeps its directory, and the ones above it.
func pruneEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			if err := pruneEmpty(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	left, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(left) == 0 {
		return os.Remove(dir)
	}
	return nil
}

// Read reports what the payload at dir says about itself. A directory holding
// no marker is not a payload, and reports so through ok.
func Read(dir string) (in Installed, ok bool, err error) {
	held, err := os.ReadFile(filepath.Join(dir, marker))
	if errors.Is(err, fs.ErrNotExist) {
		return in, false, nil
	}
	if err != nil {
		return in, false, err
	}
	in.Target, in.Version, in.Files = parseMarker(held)
	return in, true, nil
}

func plural(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// Summary describes what a render produced, one line per kind.
func (p *Payload) Summary() string {
	var parts []string
	for _, k := range sortedKinds(p.counts) {
		parts = append(parts, fmt.Sprintf("%d %s", p.counts[k], plural(string(k), p.counts[k])))
	}
	return fmt.Sprintf("%s: %s in %d files", p.Target.Name, strings.Join(parts, ", "), len(p.Files))
}
