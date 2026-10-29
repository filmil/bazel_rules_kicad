// SPDX-License-Identifier: Apache-2.0

package mirror

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func readManifest(t *testing.T) *Manifest {
	t.Helper()
	b, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// The manifest names what the kicad module extension fetches: every
// release URL of this repository there is an asset of the manifest with
// the same sha256, and every other URL there is the upstream of one.
// Then the manifest and the extension cannot drift apart.
func TestManifestMatchesExtension(t *testing.T) {
	m := readManifest(t)
	bzl, err := os.ReadFile("../../kicad/private/extensions.bzl")
	if err != nil {
		t.Fatal(err)
	}
	src := string(bzl)
	shas := map[string]bool{}
	for _, s := range regexp.MustCompile(`sha256 = "([0-9a-f]{64})"`).FindAllStringSubmatch(src, -1) {
		shas[s[1]] = true
	}
	mirrored := map[string]string{} // tag/asset to sha256
	upstream := map[string]bool{}
	for _, r := range m.Releases {
		for _, a := range r.Assets {
			mirrored[r.Tag+"/"+a.Name] = a.SHA256
			if a.URL != "" {
				upstream[a.URL] = true
			}
			if !shas[a.SHA256] {
				t.Errorf("%s/%s: sha256 %s is not in extensions.bzl", r.Tag, a.Name, a.SHA256)
			}
		}
	}
	prefix := "https://github.com/" + m.Repo + "/releases/download/"
	var inBzl []string
	for _, u := range regexp.MustCompile(`"(https://[^"]+)"`).FindAllStringSubmatch(src, -1) {
		url := u[1]
		switch {
		case strings.HasPrefix(url, prefix):
			key := strings.TrimPrefix(url, prefix)
			inBzl = append(inBzl, key)
			if _, ok := mirrored[key]; !ok {
				t.Errorf("extensions.bzl fetches %s, which is not in the manifest", url)
			}
		case strings.Contains(url, "dejavu-fonts"):
			// The fonts come from their own releases on GitHub, which
			// keep their bytes; they are not mirrored.
		case !upstream[url]:
			t.Errorf("extensions.bzl fetches %s, which is not the upstream of a mirrored asset", url)
		}
	}
	sort.Strings(inBzl)
	var inManifest []string
	for k := range mirrored {
		inManifest = append(inManifest, k)
	}
	sort.Strings(inManifest)
	if strings.Join(inBzl, " ") != strings.Join(inManifest, " ") {
		t.Errorf("release assets\n extensions.bzl %v\n manifest       %v", inBzl, inManifest)
	}
}

func TestParseRejects(t *testing.T) {
	good := `{"repo": "o/r", "releases": [{"tag": "t", "title": "T", "notes": "n", "assets": [ASSET]}]}`
	sha := strings.Repeat("a", 64)
	for name, asset := range map[string]string{
		"no source":     `{"name": "a", "sha256": "` + sha + `"}`,
		"both sources":  `{"name": "a", "sha256": "` + sha + `", "url": "https://x/a", "built": {"source": "https://x/s", "source_sha256": "` + sha + `", "script": "s.sh"}}`,
		"short sha":     `{"name": "a", "sha256": "abc", "url": "https://x/a"}`,
		"upper sha":     `{"name": "a", "sha256": "` + strings.Repeat("A", 64) + `", "url": "https://x/a"}`,
		"slash in name": `{"name": "d/a", "sha256": "` + sha + `", "url": "https://x/a"}`,
		"no script":     `{"name": "a", "sha256": "` + sha + `", "built": {"source": "https://x/s", "source_sha256": "` + sha + `"}}`,
		"unknown field": `{"name": "a", "sha256": "` + sha + `", "url": "https://x/a", "sha1": "x"}`,
	} {
		if _, err := Parse([]byte(strings.Replace(good, "ASSET", asset, 1))); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
	ok := `{"name": "a", "sha256": "` + sha + `", "url": "https://x/a"}`
	if _, err := Parse([]byte(strings.Replace(good, "ASSET", ok, 1))); err != nil {
		t.Errorf("a good manifest: %v", err)
	}
	if _, err := Parse([]byte(`{"repo": "o/r", "releases": [{"tag": "t", "title": "T", "assets": [` + ok + `]}, {"tag": "t", "title": "T", "assets": [` + ok + `]}]}`)); err == nil {
		t.Errorf("a repeated tag parsed")
	}
}

// A dry run against a local server: no network.
func TestCheck(t *testing.T) {
	files := map[string]string{"/lib.tar.gz": "library", "/app.tar": "appimage"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
	defer srv.Close()
	built := t.TempDir()
	if err := os.WriteFile(filepath.Join(built, "kicad.tar.xz"), []byte("prebuilt"), 0o644); err != nil {
		t.Fatal(err)
	}
	release := func(libSHA, srcSHA, builtSHA, libPath string) Release {
		return Release{Tag: "t", Title: "T", Assets: []Asset{
			{Name: "lib.tar.gz", SHA256: libSHA, URL: srv.URL + libPath},
			{Name: "kicad.tar.xz", SHA256: builtSHA, Built: &Built{Source: srv.URL + "/app.tar", SourceSHA256: srcSHA, Script: "make.sh"}},
		}}
	}
	opts := func(builtDir string) Options {
		return Options{Client: srv.Client(), WorkDir: t.TempDir(), BuiltDir: builtDir, Log: &bytes.Buffer{}}
	}
	good := release(sum("library"), sum("appimage"), sum("prebuilt"), "/lib.tar.gz")

	// Without --built_dir: the source of the built asset is checked, and
	// only the mirrored asset is ready.
	ready, err := Check(good, opts(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 || ready["lib.tar.gz"] == "" {
		t.Errorf("ready %v, want lib.tar.gz only", ready)
	}
	if b, _ := os.ReadFile(ready["lib.tar.gz"]); string(b) != "library" {
		t.Errorf("downloaded %q", b)
	}

	// With it, both are ready.
	if ready, err := Check(good, opts(built)); err != nil || len(ready) != 2 {
		t.Errorf("ready %v, err %v; want both", ready, err)
	}

	// Every mismatch is reported, not only the first.
	for name, c := range map[string]struct {
		r    Release
		want []string
	}{
		"bad sha256s": {release(sum("other"), sum("other"), sum("other"), "/lib.tar.gz"),
			[]string{"lib.tar.gz: " + srv.URL + "/lib.tar.gz has sha256", "kicad.tar.xz: source", "kicad.tar.xz: " + built}},
		"missing": {release(sum("library"), sum("appimage"), sum("prebuilt"), "/gone"),
			[]string{"404 Not Found"}},
	} {
		_, err := Check(c.r, opts(built))
		if err == nil {
			t.Errorf("%s: no error", name)
			continue
		}
		for _, w := range c.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: %v\nwant %q in it", name, err, w)
			}
		}
	}
}

// The commands the operator sees and runs.
func TestCommands(t *testing.T) {
	r := Release{Tag: "prebuilt-kicad-libs-9.0.9", Title: "KiCad 9.0.9 libraries", Notes: "As GitLab served them; it's CC-BY-SA."}
	if got, want := Quote(CreateCommand("filmil/bazel_rules_kicad", r)),
		`gh release create prebuilt-kicad-libs-9.0.9 --repo filmil/bazel_rules_kicad --title 'KiCad 9.0.9 libraries' --notes 'As GitLab served them; it'\''s CC-BY-SA.'`; got != want {
		t.Errorf("create\n got %s\nwant %s", got, want)
	}
	if got, want := Quote(UploadCommand("filmil/bazel_rules_kicad", r, []string{"/tmp/w/a.tar.gz", "/tmp/w/b c.tar.gz"})),
		`gh release upload prebuilt-kicad-libs-9.0.9 --repo filmil/bazel_rules_kicad /tmp/w/a.tar.gz '/tmp/w/b c.tar.gz'`; got != want {
		t.Errorf("upload\n got %s\nwant %s", got, want)
	}
}
