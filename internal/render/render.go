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
	"sort"
	"strings"
)

// marker names the file every payload carries. It is what tells a later render
// that the directory is one tstorm wrote and may prune, rather than somebody's
// work that happens to sit at the path they passed to --out.
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
	p.Files = append(p.Files, File{Path: marker, Body: []byte(t.Name + "\n")})
	slices.SortFunc(p.Files, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	return p, nil
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
	if a.Kind == KindCommand || a.Kind == KindAgent {
		return path.Join(dir, name+".md")
	}
	return path.Join(dir, name)
}

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

// Write puts the payload at dir, removing whatever an earlier render left
// there and no longer produces. It refuses a directory that holds anything
// other than a payload.
func (p *Payload) Write(dir string) error {
	if err := p.claim(dir); err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, f := range p.Files {
		keep[f.Path] = true
		dest := filepath.Join(dir, filepath.FromSlash(f.Path))
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
	}
	return prune(dir, keep)
}

// claim reports whether dir is safe to write a payload into: empty, absent, or
// already carrying this target's marker.
func (p *Payload) claim(dir string) error {
	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return os.MkdirAll(dir, 0o755)
	case err != nil:
		return err
	case len(entries) == 0:
		return nil
	}
	if _, err := os.Stat(filepath.Join(dir, marker)); err != nil {
		return fmt.Errorf("%s holds files tstorm did not render; remove it or choose another --out", dir)
	}
	return nil
}

func prune(dir string, keep map[string]bool) error {
	var stale []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if !keep[filepath.ToSlash(rel)] {
			stale = append(stale, p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, f := range stale {
		if err := os.Remove(f); err != nil {
			return err
		}
	}
	return removeEmptyDirs(dir)
}

func removeEmptyDirs(dir string) error {
	var dirs []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && p != dir {
			dirs = append(dirs, p)
		}
		return err
	})
	if err != nil {
		return err
	}
	// Deepest first, so a directory emptied by the pass also goes.
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			if err := os.Remove(d); err != nil {
				return err
			}
		}
	}
	return nil
}

// Diff reports how the payload at dir differs from this one, so a check can
// tell whether the committed output still matches the source. An empty result
// means they agree.
func (p *Payload) Diff(dir string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, f := range p.Files {
		seen[f.Path] = true
		body, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f.Path)))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			out = append(out, "missing: "+f.Path)
		case err != nil:
			return nil, err
		case !bytes.Equal(body, f.Body):
			out = append(out, "stale: "+f.Path)
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
