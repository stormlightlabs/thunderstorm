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
	p := &Payload{Target: t, counts: map[Kind]int{}}

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

// markerBody is the target and everything the render writes, one per line.
// Paths in skip were not written and are left out.
func (p *Payload) markerBody(skip map[string]bool) []byte {
	var b strings.Builder
	b.WriteString(p.Target.Name)
	b.WriteString("\n")
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

// Write puts the payload at dir and reports the files it left alone.
//
// It builds the whole tree beside dir and swaps it in, so a render either
// replaces the payload or leaves the previous one untouched. Nothing is
// deleted file by file: an earlier version of this wrote each file in place
// and then removed whatever it had not written, which deleted a git
// repository that happened to hold a marker file.
func (p *Payload) Write(dir string, adopt bool) ([]string, error) {
	dir, kept, err := p.claim(dir, adopt)
	if err != nil {
		return nil, err
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
		return nil, err
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
			return nil, err
		}
		mode := f.Mode
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(dest, f.Body, mode); err != nil {
			return nil, err
		}
		// WriteFile applies the mode only when it creates the file, and a
		// staged file is always new, but an inherited umask still narrows it.
		if err := os.Chmod(dest, mode); err != nil {
			return nil, err
		}
	}

	previous, err := os.MkdirTemp(filepath.Dir(dir), "."+filepath.Base(dir)+".tstorm-previous-")
	if err != nil {
		return nil, err
	}
	// MkdirTemp made the directory; rename needs the name free.
	if err := os.Remove(previous); err != nil {
		return nil, err
	}
	swapped := false
	if err := os.Rename(dir, previous); err == nil {
		swapped = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err := os.Rename(staging, dir); err != nil {
		if swapped {
			os.Rename(previous, dir)
		}
		return nil, err
	}
	if err := carry(previous, dir, kept); err != nil {
		return nil, fmt.Errorf("%w; the files this render did not write are in %s", err, previous)
	}
	return kept, os.RemoveAll(previous)
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

// claim decides whether dir may be replaced, and returns the path to replace
// along with the files in it this render does not own.
//
// The marker decides ownership: a file it lists is tstorm's to replace and to
// delete once the source stops producing it, and every other file is the
// repository's. A directory carrying no marker is refused unless adopt is
// set, which takes over the paths this payload writes and leaves the rest.
func (p *Payload) claim(dir string, adopt bool) (string, []string, error) {
	resolved, err := filepath.EvalSymlinks(dir)
	if err == nil {
		dir = resolved
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", nil, err
	}

	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return dir, nil, os.MkdirAll(dir, 0o755)
	case err != nil:
		return "", nil, err
	case len(entries) == 0:
		return dir, nil, nil
	}

	writing := map[string]bool{}
	seeds := map[string]bool{}
	for _, f := range p.Files {
		writing[f.Path] = true
		if f.Seed {
			seeds[f.Path] = true
		}
	}

	var owned map[string]bool
	held, err := os.ReadFile(filepath.Join(dir, marker))
	switch {
	case err != nil && adopt:
		// An unmarked directory has no record of its payload, so the paths
		// this one writes are what adoption takes over. Seeds stay the
		// repository's.
		owned = map[string]bool{}
		for path := range writing {
			owned[path] = !seeds[path]
		}
	case err != nil:
		return "", nil, fmt.Errorf("%s holds files tstorm did not render; render it with --adopt, or choose another --out", dir)
	default:
		lines := strings.Split(strings.TrimSpace(string(held)), "\n")
		if name := strings.TrimSpace(lines[0]); name != p.Target.Name {
			return "", nil, fmt.Errorf("%s holds the %s payload, not %s; choose another --out", dir, name, p.Target.Name)
		}
		owned = map[string]bool{marker: true}
		for _, path := range lines[1:] {
			owned[strings.TrimSpace(path)] = true
		}
	}

	var kept []string
	var collision string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(rel)
		if owned[slash] || slash == marker {
			return nil
		}
		if writing[slash] && !seeds[slash] && collision == "" {
			collision = slash
		}
		kept = append(kept, slash)
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	if collision != "" {
		return "", nil, fmt.Errorf("%s holds %s, which this render also writes; move it aside or choose another --out",
			dir, collision)
	}
	slices.Sort(kept)
	return dir, kept, nil
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

// Adoption is what a render would do to a directory carrying no marker.
// Leave is where an older payload's files show up: a render removes only what
// it wrote, so they survive it and are the operator's to delete.
type Adoption struct {
	Replace []string
	Leave   []string
}

// Adopt reports what adopting dir would do, and writes nothing.
func (p *Payload) Adopt(dir string) (*Adoption, error) {
	writing := map[string]bool{}
	seeds := map[string]bool{}
	for _, f := range p.Files {
		writing[f.Path] = true
		if f.Seed {
			seeds[f.Path] = true
		}
	}
	a := &Adoption{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(rel)
		switch {
		case slash == marker:
		case writing[slash] && !seeds[slash]:
			a.Replace = append(a.Replace, slash)
		default:
			a.Leave = append(a.Leave, slash)
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	slices.Sort(a.Replace)
	slices.Sort(a.Leave)
	return a, nil
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
	for _, line := range strings.Split(strings.TrimSpace(string(held)), "\n")[1:] {
		owned[strings.TrimSpace(line)] = true
	}
	return owned, nil
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
