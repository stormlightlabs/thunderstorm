// Package release reads what GitHub publishes for tstorm, and replaces the
// running binary with it.
//
// Nothing here is on the path of a check or a render. A release lookup is a
// remark at the end of an update, so every failure in it is silent: a machine
// with no network still updates the payload it was asked to update.
package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// api is where the releases are published. The repository is compiled in
// because a binary asking a configurable host for its own replacement is a
// supply chain nobody asked for.
//
// latest excludes prereleases, and every release so far is one, so a 404 there
// falls through to the list, newest first.
const (
	latestAPI = "https://api.github.com/repos/stormlightlabs/thunderstorm/releases/latest"
	listAPI   = "https://api.github.com/repos/stormlightlabs/thunderstorm/releases?per_page=10"
)

// cacheFor is how long an answer stands, and cacheFailureFor how long a
// failure does. An update run twice in a morning asks GitHub once; one run
// while the network was down asks again after an hour rather than tomorrow.
const (
	cacheFor        = 24 * time.Hour
	cacheFailureFor = time.Hour
)

// timeout keeps a slow network from holding up a report that is a remark.
const timeout = 3 * time.Second

// Release is one published release: its tag and the files under it.
type Release struct {
	Tag    string            `json:"tag_name"`
	Assets map[string]string `json:"assets"`
	Read   time.Time         `json:"read"`
}

// Latest reports the most recent release, from the cache when it is fresh.
// Every failure reads as "no answer", because this never blocks the work.
func Latest() (Release, bool) {
	if held, ok := cached(); ok {
		return held, held.Tag != ""
	}
	found, err := fetch()
	if err != nil {
		// A failed lookup is cached as an empty answer, so a machine with no
		// network asks once a day rather than on every update.
		store(Release{Read: time.Now()})
		return Release{}, false
	}
	store(found)
	return found, found.Tag != ""
}

// Newer reports whether tag is a later version than the build that is running.
func Newer(running, tag string) bool { return compare(running, tag) < 0 }

// Asset is the archive for this operating system and architecture.
func (r Release) Asset() (string, bool) {
	want := "_" + runtime.GOOS + "_" + runtime.GOARCH + "."
	for name, url := range r.Assets {
		if strings.Contains(name, want) && strings.HasSuffix(name, ".tar.gz") {
			return url, true
		}
	}
	return "", false
}

// Replace puts this release's tstorm where the running one is.
//
// The new binary is written beside the old one and renamed over it, which is
// atomic on one filesystem and is why the temporary file goes there rather
// than in a temporary directory. A running binary can be renamed over on Unix;
// the process keeps the inode it started with until it exits.
func (r Release) Replace(dest string) error {
	url, ok := r.Asset()
	if !ok {
		return fmt.Errorf("%s publishes nothing for %s/%s", r.Tag, runtime.GOOS, runtime.GOARCH)
	}
	archive, err := get(url)
	if err != nil {
		return err
	}
	sums, ok := r.Assets["checksums.txt"]
	if !ok {
		return fmt.Errorf("%s publishes no checksums.txt; refusing to install what nothing vouches for", r.Tag)
	}
	list, err := get(sums)
	if err != nil {
		return err
	}
	if err := verify(archive, list, url); err != nil {
		return err
	}
	binary, err := unpack(archive)
	if err != nil {
		return err
	}

	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".tstorm-*")
	if err != nil {
		return fmt.Errorf("%s: %w", dir, err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o755); err != nil {
		return err
	}
	return os.Rename(name, dest)
}

// verify checks the archive against the checksums file, matched by the asset's
// own file name.
func verify(archive, list []byte, url string) error {
	want := ""
	file := url[strings.LastIndex(url, "/")+1:]
	for _, line := range strings.Split(string(list), "\n") {
		sum, name, ok := strings.Cut(strings.TrimSpace(line), "  ")
		if ok && name == file {
			want = sum
			break
		}
	}
	if want == "" {
		return fmt.Errorf("checksums.txt does not name %s", file)
	}
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("%s hashes to %s, and checksums.txt says %s", file, got, want)
	}
	return nil
}

// unpack returns the tstorm binary inside a release archive.
func unpack(archive []byte) ([]byte, error) {
	zipped, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer zipped.Close()
	r := tar.NewReader(zipped)
	for {
		header, err := r.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("the archive holds no tstorm")
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(header.Name) != "tstorm" || header.Typeflag != tar.TypeReg {
			continue
		}
		return io.ReadAll(r)
	}
}

type published struct {
	Tag    string `json:"tag_name"`
	Draft  bool   `json:"draft"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func fetch() (Release, error) {
	if body, err := get(latestAPI); err == nil {
		var one published
		if err := json.Unmarshal(body, &one); err == nil && one.Tag != "" {
			return release(one), nil
		}
	}

	body, err := get(listAPI)
	if err != nil {
		return Release{}, err
	}
	var all []published
	if err := json.Unmarshal(body, &all); err != nil {
		return Release{}, err
	}
	for _, one := range all {
		if !one.Draft && one.Tag != "" {
			return release(one), nil
		}
	}
	return Release{}, fmt.Errorf("no release is published")
}

func release(p published) Release {
	found := Release{Tag: p.Tag, Assets: map[string]string{}, Read: time.Now()}
	for _, a := range p.Assets {
		found.Assets[a.Name] = a.URL
	}
	return found
}

func get(url string) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

func cacheFile() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "tstorm", "release.json")
}

func cached() (Release, bool) {
	file := cacheFile()
	if file == "" {
		return Release{}, false
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return Release{}, false
	}
	var held Release
	if err := json.Unmarshal(raw, &held); err != nil {
		return Release{}, false
	}
	if held.Tag == "" {
		return held, time.Since(held.Read) < cacheFailureFor
	}
	return held, time.Since(held.Read) < cacheFor
}

func store(r Release) {
	file := cacheFile()
	if file == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return
	}
	body, err := json.Marshal(r)
	if err != nil {
		return
	}
	_ = os.WriteFile(file, body, 0o644)
}

// compare orders two versions the way semantic versioning does: by the three
// numbers, and then a prerelease before the release it leads to. Build
// metadata after a + is not part of the order, which is what makes a source
// build compare equal to the tag it descends from.
func compare(a, b string) int {
	amain, apre := split(a)
	bmain, bpre := split(b)
	for i := range 3 {
		if n := amain[i] - bmain[i]; n != 0 {
			return sign(n)
		}
	}
	switch {
	case apre == "" && bpre == "":
		return 0
	case apre == "":
		return 1
	case bpre == "":
		return -1
	}
	return comparePre(strings.Split(apre, "."), strings.Split(bpre, "."))
}

// split reads a version into its three numbers and its prerelease.
func split(v string) ([3]int, string) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v, _, _ = strings.Cut(v, "+")
	core, pre, _ := strings.Cut(v, "-")
	var out [3]int
	for i, field := range strings.SplitN(core, ".", 3) {
		if i > 2 {
			break
		}
		out[i], _ = strconv.Atoi(field)
	}
	return out, pre
}

// comparePre orders prerelease identifiers: numbers numerically, anything else
// as text, and a shorter run of identifiers before a longer one that starts
// the same way.
func comparePre(a, b []string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		an, aerr := strconv.Atoi(a[i])
		bn, berr := strconv.Atoi(b[i])
		switch {
		case aerr == nil && berr == nil:
			if an != bn {
				return sign(an - bn)
			}
		case aerr == nil:
			return -1
		case berr == nil:
			return 1
		default:
			if n := strings.Compare(a[i], b[i]); n != 0 {
				return n
			}
		}
	}
	return sign(len(a) - len(b))
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}
