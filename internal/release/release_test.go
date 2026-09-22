package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// The order decides whether an operator is told to update, so a build that is
// ahead of the last release has to compare as ahead.
func TestNewerOrdersReleasesAndTheBuildsBetweenThem(t *testing.T) {
	for name, tc := range map[string]struct {
		running, tag string
		newer        bool
	}{
		"a later patch":                 {"v0.1.0", "v0.1.1", true},
		"a later minor":                 {"v0.1.9", "v0.2.0", true},
		"the release after a rc":        {"v0.1.0-rc.1", "v0.1.0", true},
		"a later rc":                    {"v0.1.0-rc.1", "v0.1.0-rc.2", true},
		"the same tag":                  {"v0.1.0", "v0.1.0", false},
		"a source build of that tag":    {"v0.1.0-rc.1+g1969674", "v0.1.0-rc.1", false},
		"a source build ahead of it":    {"v0.2.0+g1969674", "v0.1.0", false},
		"an older release":              {"v0.2.0", "v0.1.0", false},
		"a rc before the release it is": {"v0.1.0", "v0.1.0-rc.1", false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := Newer(tc.running, tc.tag); got != tc.newer {
				t.Errorf("Newer(%q, %q) = %v", tc.running, tc.tag, got)
			}
		})
	}
}

func archive(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zip := gzip.NewWriter(&buf)
	w := tar.NewWriter(zip)
	for _, f := range []struct {
		name string
		body []byte
	}{{"LICENSE", []byte("a licence")}, {name, body}} {
		header := &tar.Header{Name: f.name, Mode: 0o755, Size: int64(len(f.body)), Typeflag: tar.TypeReg}
		if err := w.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(f.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zip.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestTheBinaryComesOutOfTheArchive(t *testing.T) {
	body, err := unpack(archive(t, "tstorm", []byte("ELF")))
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	if string(body) != "ELF" {
		t.Errorf("unpack returned %q", body)
	}
}

func TestAnArchiveWithoutTheBinaryIsAnError(t *testing.T) {
	if _, err := unpack(archive(t, "tstormd", []byte("ELF"))); err == nil {
		t.Error("an archive holding no tstorm was accepted")
	}
}

// A binary is replaced by what this checks, so a checksum that does not match
// has to stop before anything is written.
func TestVerifyMatchesTheChecksumForThisAsset(t *testing.T) {
	body := []byte("an archive")
	sum := sha256.Sum256(body)
	url := "https://example.invalid/tstorm_0.1.0_linux_amd64.tar.gz"
	list := "0000  tstorm_0.1.0_darwin_arm64.tar.gz\n" +
		hex.EncodeToString(sum[:]) + "  tstorm_0.1.0_linux_amd64.tar.gz\n"

	if err := verify(body, []byte(list), url); err != nil {
		t.Errorf("a matching checksum was refused: %v", err)
	}
	if err := verify([]byte("something else"), []byte(list), url); err == nil {
		t.Error("a mismatched archive was accepted")
	}
	err := verify(body, []byte("0000  tstorm_0.1.0_windows_amd64.tar.gz\n"), url)
	if err == nil || !strings.Contains(err.Error(), "does not name") {
		t.Errorf("an unlisted asset gave %v", err)
	}
}

func TestTheAssetIsTheOneForThisMachine(t *testing.T) {
	r := Release{Assets: map[string]string{
		"tstorm_0.1.0_linux_amd64.tar.gz":  "linux-amd64",
		"tstorm_0.1.0_darwin_arm64.tar.gz": "darwin-arm64",
		"checksums.txt":                    "sums",
	}}
	url, ok := r.Asset()
	if !ok {
		t.Skip("this machine's build is not among the test assets")
	}
	if strings.Contains(url, "sums") {
		t.Errorf("the checksums file was taken for an archive: %s", url)
	}
}
