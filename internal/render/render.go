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

// Plan builds the payload for one target from the source tree at root. It
// returns an *Unmet when the target cannot carry every artifact.
func Plan(m Manifest, t *Target, root string) (*Payload, error) {
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
		files, err := p.render(a, t, root)
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
	p.Files = append(p.Files, File{Path: marker, Body: p.markerBody()})
	slices.SortFunc(p.Files, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	return p, nil
}

// markerBody is the target and everything the render writes, one per line.
func (p *Payload) markerBody() []byte {
	var b strings.Builder
	b.WriteString(p.Target.Name)
	b.WriteString("\n")
	for _, f := range p.Files {
		if f.Path != marker {
			b.WriteString(f.Path)
			b.WriteString("\n")
		}
	}
	return []byte(b.String())
}

// render copies one artifact into payload files, substituting the resource
// root in prose and giving an alias its own file, because no harness here has
// an alias mechanism of its own.
func (p *Payload) render(a Artifact, t *Target, root string) ([]File, error) {
	dir := t.dirs[a.Kind]
	sources, err := a.files(root)
	if err != nil {
		return nil, err
	}

	var out []File
	for _, src := range sources {
		body, mode, err := readSource(root, src)
		if err != nil {
			return nil, fmt.Errorf("artifact %q: %w", a.Name, err)
		}
		if strings.HasSuffix(src, ".md") {
			body = bytes.ReplaceAll(body, []byte(rootToken), []byte(t.Root))
			if t.Plugin != "" {
				body = bytes.ReplaceAll(body, []byte(pluginToken), []byte(t.Plugin))
			}
			if found := leftoverToken.Find(body); found != nil {
				return nil, fmt.Errorf("artifact %q: %s names %s, which %s has no value for",
					a.Name, src, found, t.Name)
			}
		}
		if t.transform != nil {
			body, err = t.transform(a, body)
			if err != nil {
				return nil, err
			}
		}
		for _, name := range a.Names() {
			out = append(out, File{Path: p.destination(a, dir, name, src), Body: body, Mode: mode})
		}
	}
	return out, nil
}

// destination places one source file in the payload. A skill keeps its
// directory and everything under it; everything else becomes one file named
// for the command, agent, hook, or script that the harness will look up.
func (p *Payload) destination(a Artifact, dir, name, src string) string {
	if a.Kind == KindSkill {
		rel := strings.TrimPrefix(src, a.Source+"/")
		return path.Join(dir, name, rel)
	}
	if a.Kind == KindCommand {
		return path.Join(dir, name+".md")
	}
	if a.Kind == KindAgent {
		ext := ".md"
		if p.Target.Name == "codex" {
			ext = ".toml"
		}
		return path.Join(dir, name+ext)
	}
	return path.Join(dir, name)
}

// executable reports whether a mode carries any execute bit, which is the
// whole of what git stores and therefore the whole of what a payload promises.
// A zero mode is a file the renderer generated, which is never executable.
func executable(mode fs.FileMode) bool { return mode&0o111 != 0 }

func readSource(root, src string) ([]byte, fs.FileMode, error) {
	full := filepath.Join(root, filepath.FromSlash(src))
	body, err := os.ReadFile(full)
	if err != nil {
		return nil, 0, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, 0, err
	}
	mode := fs.FileMode(0o644)
	if info.Mode()&0o111 != 0 {
		mode = 0o755
	}
	return body, mode, nil
}

// Write puts the payload at dir.
//
// It builds the whole tree beside dir and swaps it in, so a render either
// replaces the payload or leaves the previous one untouched. Nothing is
// deleted file by file: an earlier version of this wrote each file in place
// and then removed whatever it had not written, which deleted a git
// repository that happened to hold a marker file.
func (p *Payload) Write(dir string) error {
	dir, err := p.claim(dir)
	if err != nil {
		return err
	}

	// A sibling of the destination, so the rename that swaps it in stays on
	// one filesystem. A render killed outright leaves one behind; it is named
	// so it is recognisable, and no later render reads it.
	staging, err := os.MkdirTemp(filepath.Dir(dir), "."+filepath.Base(dir)+".tstorm-staging-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	for _, f := range p.Files {
		dest := filepath.Join(staging, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		mode := f.Mode
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(dest, f.Body, mode); err != nil {
			return err
		}
		// WriteFile applies the mode only when it creates the file, and a
		// staged file is always new, but an inherited umask still narrows it.
		if err := os.Chmod(dest, mode); err != nil {
			return err
		}
	}

	previous, err := os.MkdirTemp(filepath.Dir(dir), "."+filepath.Base(dir)+".tstorm-previous-")
	if err != nil {
		return err
	}
	// MkdirTemp made the directory; rename needs the name free.
	if err := os.Remove(previous); err != nil {
		return err
	}
	swapped := false
	if err := os.Rename(dir, previous); err == nil {
		swapped = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(staging, dir); err != nil {
		if swapped {
			os.Rename(previous, dir)
		}
		return err
	}
	return os.RemoveAll(previous)
}

// claim decides whether dir may be replaced, and returns the path to replace.
//
// A payload is written into an empty or absent directory, or over a directory
// whose marker names this target and accounts for everything in it. A file the
// marker does not list means the directory is somebody's work, whatever it is
// called, and the render stops rather than replacing it.
func (p *Payload) claim(dir string) (string, error) {
	resolved, err := filepath.EvalSymlinks(dir)
	if err == nil {
		dir = resolved
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return dir, os.MkdirAll(dir, 0o755)
	case err != nil:
		return "", err
	case len(entries) == 0:
		return dir, nil
	}

	held, err := os.ReadFile(filepath.Join(dir, marker))
	if err != nil {
		return "", fmt.Errorf("%s holds files tstorm did not render; remove it or choose another --out", dir)
	}
	lines := strings.Split(strings.TrimSpace(string(held)), "\n")
	if name := strings.TrimSpace(lines[0]); name != p.Target.Name {
		return "", fmt.Errorf("%s holds the %s payload, not %s; choose another --out", dir, name, p.Target.Name)
	}
	owned := map[string]bool{marker: true}
	for _, path := range lines[1:] {
		owned[strings.TrimSpace(path)] = true
	}

	var foreign string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || foreign != "" {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if slash := filepath.ToSlash(rel); !owned[slash] {
			foreign = slash
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if foreign != "" {
		return "", fmt.Errorf("%s holds %s, which no render wrote; remove it or choose another --out", dir, foreign)
	}
	return dir, nil
}

// Diff reports how the payload at dir differs from this one, so a check can
// tell whether the committed output still matches the source. An empty result
// means they agree.
func (p *Payload) Diff(dir string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, f := range p.Files {
		seen[f.Path] = true
		full := filepath.Join(dir, filepath.FromSlash(f.Path))
		body, err := os.ReadFile(full)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			out = append(out, "missing: "+f.Path)
			continue
		case err != nil:
			return nil, err
		case !bytes.Equal(body, f.Body):
			out = append(out, "stale: "+f.Path)
		}
		// A check that reads only bytes certifies a script whose executable
		// bit a checkout dropped, and the skills invoke those by path.
		info, err := os.Stat(full)
		if err != nil {
			return nil, err
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
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if slash := filepath.ToSlash(rel); !seen[slash] {
			out = append(out, "unexpected: "+slash)
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	slices.Sort(out)
	return out, nil
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
