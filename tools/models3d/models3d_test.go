// SPDX-License-Identifier: Apache-2.0

package models3d

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const base = "https://bt.example/kicad/packages3D/9.0.9/"

// upstream builds a small kicad-packages3D-like .tar.gz: two libraries,
// the license, and files that are not libraries.
func upstream(t *testing.T, big int) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	add := func(name string, typ byte, data []byte) {
		h := &tar.Header{Name: name, Typeflag: typ, Mode: 0o664, Size: int64(len(data)), Uname: "someone"}
		if typ == tar.TypeDir {
			h.Size = 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		tw.Write(data)
	}
	add("kicad-packages3D-9.0.9/", tar.TypeDir, nil)
	add("kicad-packages3D-9.0.9/LICENSE.md", tar.TypeReg, []byte("CC-BY-SA 4.0 with an exception\n"))
	add("kicad-packages3D-9.0.9/README.md", tar.TypeReg, []byte("readme\n"))
	add("kicad-packages3D-9.0.9/CMakeModules/", tar.TypeDir, nil)
	add("kicad-packages3D-9.0.9/CMakeModules/x.cmake", tar.TypeReg, []byte("x\n"))
	// Files before their directory, and out of order: the archive sorts.
	add("kicad-packages3D-9.0.9/Resistor_SMD.3dshapes/R_0603.wrl", tar.TypeReg, []byte("wrl\n"))
	add("kicad-packages3D-9.0.9/Resistor_SMD.3dshapes/", tar.TypeDir, nil)
	add("kicad-packages3D-9.0.9/Resistor_SMD.3dshapes/R_0603.step", tar.TypeReg, []byte("step\n"))
	add("kicad-packages3D-9.0.9/LED_SMD.3dshapes/", tar.TypeDir, nil)
	add("kicad-packages3D-9.0.9/LED_SMD.3dshapes/LED_0603.step", tar.TypeReg, bytes.Repeat([]byte("L"), big))
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func split(t *testing.T, up []byte) (string, []Library) {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	libs, err := Split(bytes.NewReader(up), out, filepath.Join(dir, "work"), base)
	if err != nil {
		t.Fatal(err)
	}
	return out, libs
}

// The names in a .tar.gz, in order.
func entries(t *testing.T, data []byte) []string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if gz.Name != "" || !gz.ModTime.IsZero() {
		t.Errorf("gzip header has a name %q or a time %v", gz.Name, gz.ModTime)
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return names
		}
		if err != nil {
			t.Fatal(err)
		}
		if !h.ModTime.IsZero() && h.ModTime.Unix() != 0 || h.Uid != 0 || h.Uname != "" {
			t.Errorf("%s: time %v, uid %d, uname %q", h.Name, h.ModTime, h.Uid, h.Uname)
		}
		names = append(names, h.Name)
	}
}

func TestSplit(t *testing.T) {
	out, libs := split(t, upstream(t, 10))
	var got []string
	for _, l := range libs {
		got = append(got, l.Name)
	}
	if strings.Join(got, " ") != "LED_SMD Resistor_SMD" {
		t.Fatalf("libraries %v", got)
	}
	data, err := os.ReadFile(filepath.Join(out, "Resistor_SMD.3dshapes.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	want := "LICENSE.md Resistor_SMD.3dshapes/ Resistor_SMD.3dshapes/R_0603.step Resistor_SMD.3dshapes/R_0603.wrl"
	if got := strings.Join(entries(t, data), " "); got != want {
		t.Errorf("entries\n got %s\nwant %s", got, want)
	}
	files, _ := os.ReadDir(out)
	var names []string
	for _, f := range files {
		names = append(names, f.Name())
	}
	want = "LED_SMD.3dshapes.tar.gz LED_SMD.3dshapes.tar.gz.torrent LICENSE.md Resistor_SMD.3dshapes.tar.gz Resistor_SMD.3dshapes.tar.gz.torrent"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("out\n got %s\nwant %s", got, want)
	}
}

// The same upstream gives the same bytes, whatever the order or the
// owners in the upstream tar.
func TestReproducible(t *testing.T) {
	up := upstream(t, 3*PieceLength/2)
	a, _ := split(t, up)
	b, _ := split(t, up)
	files, _ := os.ReadDir(a)
	for _, f := range files {
		x, _ := os.ReadFile(filepath.Join(a, f.Name()))
		y, _ := os.ReadFile(filepath.Join(b, f.Name()))
		if !bytes.Equal(x, y) {
			t.Errorf("%s differs between two runs", f.Name())
		}
	}
}

// A minimal bencode decoder, for the test only.
func decode(t *testing.T, s string) (any, string) {
	t.Helper()
	switch {
	case s[0] == 'i':
		e := strings.IndexByte(s, 'e')
		return s[1:e], s[e+1:]
	case s[0] == 'l':
		var l []any
		s = s[1:]
		for s[0] != 'e' {
			var v any
			v, s = decode(t, s)
			l = append(l, v)
		}
		return l, s[1:]
	case s[0] == 'd':
		m := map[string]any{}
		var keys []string
		s = s[1:]
		for s[0] != 'e' {
			var k, v any
			k, s = decode(t, s)
			v, s = decode(t, s)
			m[k.(string)] = v
			keys = append(keys, k.(string))
		}
		if !sort.StringsAreSorted(keys) {
			t.Errorf("keys not sorted: %v", keys)
		}
		return m, s[1:]
	default:
		c := strings.IndexByte(s, ':')
		n := 0
		for _, d := range s[:c] {
			n = n*10 + int(d-'0')
		}
		return s[c+1 : c+1+n], s[c+1+n:]
	}
}

func TestTorrent(t *testing.T) {
	data := bytes.Repeat([]byte("ab"), PieceLength) // two pieces exactly
	data = append(data, 'z')                        // and one byte more
	tor, err := Torrent("X.3dshapes.tar.gz", data, base+"X.3dshapes.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	v, rest := decode(t, string(tor))
	if rest != "" {
		t.Fatalf("trailing %q", rest)
	}
	m := v.(map[string]any)
	if _, ok := m["creation date"]; ok {
		t.Error("a creation date makes the torrent irreproducible")
	}
	if _, ok := m["announce"]; ok {
		t.Error("no tracker: the web seed is the source")
	}
	if ul := m["url-list"].([]any); len(ul) != 1 || ul[0] != base+"X.3dshapes.tar.gz" {
		t.Errorf("url-list %v", ul)
	}
	info := m["info"].(map[string]any)
	if info["name"] != "X.3dshapes.tar.gz" || info["length"] != "2097153" || info["piece length"] != "1048576" {
		t.Errorf("info %v", info)
	}
	pieces := info["pieces"].(string)
	if len(pieces) != 3*20 {
		t.Fatalf("%d bytes of piece hashes, want 60", len(pieces))
	}
	last := sha1.Sum([]byte("z"))
	if pieces[40:] != string(last[:]) {
		t.Error("the last piece's hash is wrong")
	}
}

func TestPins(t *testing.T) {
	_, libs := split(t, upstream(t, 10))
	p, err := Pins("9.0.9", base, libs)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`MODELS_VERSION = "9.0.9"`,
		`MODELS_BASE_URL = "` + base + `"`,
		`"LED_SMD": "sha256-`,
		`"Resistor_SMD": "sha256-`,
	} {
		if !strings.Contains(p, want) {
			t.Errorf("pins lack %q:\n%s", want, p)
		}
	}
}

// tarGz makes a .tar.gz of the given headers, each with its data.
func tarGz(t *testing.T, hs []*tar.Header, data []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for i, h := range hs {
		if h.Typeflag == tar.TypeReg {
			h.Size = int64(len(data[i]))
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(data[i]))
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

// Each bad entry comes with a valid upstream around it, so that only the
// entry can make Split fail.
func TestBadUpstream(t *testing.T) {
	for name, bad := range map[string]*tar.Header{
		"dot dot":         {Name: "top/../../evil", Typeflag: tar.TypeReg},
		"dot dot deeper":  {Name: "top/A.3dshapes/../../../evil", Typeflag: tar.TypeReg},
		"absolute":        {Name: "/evil", Typeflag: tar.TypeReg},
		"absolute in top": {Name: "/top/A.3dshapes/x", Typeflag: tar.TypeReg},
		"other top":       {Name: "other/A.3dshapes/x", Typeflag: tar.TypeReg},
		"beside top":      {Name: "file", Typeflag: tar.TypeReg},
		"symlink":         {Name: "top/A.3dshapes/l", Typeflag: tar.TypeSymlink, Linkname: "x"},
		"hard link":       {Name: "top/A.3dshapes/h", Typeflag: tar.TypeLink, Linkname: "top/A.3dshapes/x"},
	} {
		up := tarGz(t, []*tar.Header{
			{Name: "top/", Typeflag: tar.TypeDir},
			{Name: "top/LICENSE.md", Typeflag: tar.TypeReg},
			{Name: "top/A.3dshapes/x", Typeflag: tar.TypeReg},
			bad,
		}, []string{"", "license", "x", "bad"})
		dir := t.TempDir()
		if _, err := Split(bytes.NewReader(up), filepath.Join(dir, "out"), filepath.Join(dir, "work"), base); err == nil {
			t.Errorf("%s (%q): want an error", name, bad.Name)
		}
		if _, err := os.Stat(filepath.Join(dir, "evil")); err == nil {
			t.Errorf("%s: wrote outside the work directory", name)
		}
	}

	// The same upstream without a bad entry passes: the cases above fail on
	// the entry, not on something else.
	good := tarGz(t, []*tar.Header{
		{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": "abc"}},
		{Name: "top/", Typeflag: tar.TypeDir},
		{Name: "top/LICENSE.md", Typeflag: tar.TypeReg},
		{Name: "top/A.3dshapes/x", Typeflag: tar.TypeReg},
	}, []string{"", "", "license", "x"})
	dir := t.TempDir()
	if _, err := Split(bytes.NewReader(good), filepath.Join(dir, "out"), filepath.Join(dir, "work"), base); err != nil {
		t.Errorf("the good upstream: %v", err)
	}

	// A damaged gzip trailer.
	up := upstream(t, 10)
	up[len(up)-5] ^= 0xff
	dir = t.TempDir()
	if _, err := Split(bytes.NewReader(up), filepath.Join(dir, "out"), filepath.Join(dir, "work"), base); err == nil {
		t.Error("a damaged gzip trailer must fail")
	}

	for _, b := range []string{"https://no-slash", "relative/", ""} {
		dir := t.TempDir()
		if _, err := Split(bytes.NewReader(upstream(t, 1)), filepath.Join(dir, "out"), filepath.Join(dir, "work"), b); err == nil {
			t.Errorf("base URL %q must fail", b)
		}
	}

	// An out directory with files from before.
	dir = t.TempDir()
	os.MkdirAll(filepath.Join(dir, "out"), 0o755)
	os.WriteFile(filepath.Join(dir, "out", "old.tar.gz"), []byte("x"), 0o644)
	if _, err := Split(bytes.NewReader(upstream(t, 1)), filepath.Join(dir, "out"), filepath.Join(dir, "work"), base); err == nil {
		t.Error("a non-empty out directory must fail")
	}
}

// Entries of a directory follow it directly, before a sibling whose name
// sorts between them byte-wise ("a-b/" sorts before "a/x").
func TestSegmentOrder(t *testing.T) {
	up := tarGz(t, []*tar.Header{
		{Name: "top/", Typeflag: tar.TypeDir},
		{Name: "top/LICENSE.md", Typeflag: tar.TypeReg},
		{Name: "top/A.3dshapes/a-b/y", Typeflag: tar.TypeReg},
		{Name: "top/A.3dshapes/a/x", Typeflag: tar.TypeReg},
	}, []string{"", "l", "y", "x"})
	out, _ := splitUp(t, up)
	data, _ := os.ReadFile(filepath.Join(out, "A.3dshapes.tar.gz"))
	want := "LICENSE.md A.3dshapes/ A.3dshapes/a/ A.3dshapes/a/x A.3dshapes/a-b/ A.3dshapes/a-b/y"
	if got := strings.Join(entries(t, data), " "); got != want {
		t.Errorf("entries\n got %s\nwant %s", got, want)
	}
}

func splitUp(t *testing.T, up []byte) (string, []Library) {
	t.Helper()
	return split(t, up)
}

func TestFileURL(t *testing.T) {
	if got := FileURL(base, "A+B c.3dshapes.tar.gz"); got != base+"A+B%20c.3dshapes.tar.gz" {
		t.Errorf("FileURL = %s", got)
	}
}

func TestUpload(t *testing.T) {
	out, libs := split(t, upstream(t, 10))
	var mu sync.Mutex
	stored := map[string][]byte{}
	failed := map[string]bool{}
	RetryDelay = 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		// The first PUT of each file fails with 503: Upload tries again.
		if r.Method == http.MethodPut && !failed[r.URL.Path] {
			failed[r.URL.Path] = true
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodPut:
			if u, p, ok := r.BasicAuth(); !ok || u != "bt-upload" || p != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			b, _ := io.ReadAll(r.Body)
			stored[r.URL.Path] = b
			w.WriteHeader(http.StatusCreated)
		case http.MethodHead:
			if u, p, ok := r.BasicAuth(); !ok || u != "bt-upload" || p != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			b, ok := stored[r.URL.Path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(b)))
		}
	}))
	defer srv.Close()
	files := []string{LicenseFile, libs[0].Archive, libs[0].Torrent}
	var log bytes.Buffer
	if err := Upload(srv.Client(), out, srv.URL+"/k/", "bt-upload", "secret", files, &log); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		want, _ := os.ReadFile(filepath.Join(out, f))
		if !bytes.Equal(stored["/k/"+f], want) {
			t.Errorf("%s not stored as written", f)
		}
	}
	if err := Upload(srv.Client(), out, srv.URL+"/k/", "bt-upload", "wrong", files, &log); err == nil {
		t.Error("a refused PUT must fail")
	}
}
