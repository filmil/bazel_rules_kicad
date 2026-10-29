// SPDX-License-Identifier: Apache-2.0

// Package models3d publishes KiCad's 3D models (kicad-packages3D) as one
// archive per library, each with a .torrent whose web seed is the server
// the archive lives on (issue #43).
//
// Every output is reproducible: the same upstream archive gives the same
// bytes, so the pins that rules_kicad records stay valid when the files
// are published again.
package models3d

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// LicenseFile is the upstream license. It goes into every archive and
// next to the archives on the server: the license of each file comes with
// it.
const LicenseFile = "LICENSE.md"

// PieceLength is the piece size of every torrent: 1 MiB.
const PieceLength = 1 << 20

// CreatedBy names the tool in each torrent. A torrent has no creation date,
// so that it is reproducible.
const CreatedBy = "rules_kicad tools/models3d"

// libDir matches a library directory at the top of the upstream tree.
var libDir = regexp.MustCompile(`^[A-Za-z0-9_.+-]+\.3dshapes$`)

// Library is one published library.
type Library struct {
	// Name is the library's name without ".3dshapes", e.g. "Resistor_SMD".
	Name string
	// Archive is the archive's file name, "<Name>.3dshapes.tar.gz".
	Archive string
	// Torrent is the torrent's file name, "<Archive>.torrent".
	Torrent string
	// SHA256 is the archive's hash, as rules_bittorrent's `integrity`
	// wants it: "sha256-<base64>".
	SHA256 string
	// Size is the archive's length in bytes.
	Size int64
}

// ArchiveName returns the archive file name of library name.
func ArchiveName(name string) string { return name + ".3dshapes.tar.gz" }

// Split reads the upstream archive (a .tar.gz with one top directory) and
// writes, into out, one archive per <name>.3dshapes directory, a torrent
// for each, and the upstream license. Each archive holds
// <name>.3dshapes/... and LICENSE.md. baseURL is where the files will be
// served; it must end in "/". work is a directory for the extracted tree.
func Split(upstream io.Reader, out, work, baseURL string) ([]Library, error) {
	if err := checkBase(baseURL); err != nil {
		return nil, err
	}
	// A file left from an earlier run would be published with these.
	if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 {
		return nil, fmt.Errorf("%s is not empty", out)
	}
	if err := extract(upstream, work); err != nil {
		return nil, err
	}
	license, err := os.ReadFile(filepath.Join(work, LicenseFile))
	if err != nil {
		return nil, fmt.Errorf("upstream has no %s: %w", LicenseFile, err)
	}
	entries, err := os.ReadDir(work)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && libDir.MatchString(e.Name()) {
			names = append(names, strings.TrimSuffix(e.Name(), ".3dshapes"))
		}
	}
	if len(names) == 0 {
		return nil, errors.New("upstream has no <name>.3dshapes directory")
	}
	sort.Strings(names)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(out, LicenseFile), license, 0o644); err != nil {
		return nil, err
	}
	var libs []Library
	for _, n := range names {
		lib := Library{Name: n, Archive: ArchiveName(n), Torrent: ArchiveName(n) + ".torrent"}
		data, err := archive(work, n+".3dshapes", license)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		sum := sha256.Sum256(data)
		lib.SHA256 = "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
		lib.Size = int64(len(data))
		if err := os.WriteFile(filepath.Join(out, lib.Archive), data, 0o644); err != nil {
			return nil, err
		}
		t, err := Torrent(lib.Archive, data, FileURL(baseURL, lib.Archive))
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(out, lib.Torrent), t, 0o644); err != nil {
			return nil, err
		}
		libs = append(libs, lib)
	}
	return libs, nil
}

// extract unpacks the upstream .tar.gz into dir, without its top
// directory. Every entry must be under the same top directory, with no ".."
// and no absolute path; only directories and regular files may occur,
// besides the pax global header that GitLab writes. The gzip stream is read
// to its end, so a damaged trailer fails.
func extract(r io.Reader, dir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	top := ""
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		name := strings.TrimSuffix(h.Name, "/")
		if name == "" || path.IsAbs(name) {
			return fmt.Errorf("unsafe path %q", h.Name)
		}
		segs := strings.Split(name, "/")
		for _, s := range segs {
			if s == ".." || s == "." || s == "" {
				return fmt.Errorf("unsafe path %q", h.Name)
			}
		}
		if top == "" {
			top = segs[0]
		} else if segs[0] != top {
			return fmt.Errorf("%q is not under the top directory %q", h.Name, top)
		}
		if len(segs) == 1 {
			if h.Typeflag != tar.TypeDir {
				return fmt.Errorf("%q beside the top directory", h.Name)
			}
			continue
		}
		p := filepath.Join(dir, filepath.FromSlash(strings.Join(segs[1:], "/")))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return err
			}
			f, err := os.Create(p)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		default:
			// A link would be left out of the archives without notice.
			return fmt.Errorf("%q: entry type %q is not supported", h.Name, h.Typeflag)
		}
	}
	if _, err := io.Copy(io.Discard, gz); err != nil {
		return err
	}
	return gz.Close()
}

// archive writes the .tar.gz of dir/sub: LICENSE.md, then sub/ and its
// files in sorted order. Every entry has the time 0, owner 0 and a fixed
// mode, and the gzip header no name or time, so the bytes depend only on
// the files.
func archive(dir, sub string, license []byte) ([]byte, error) {
	var paths []string
	err := filepath.WalkDir(filepath.Join(dir, sub), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			rel += "/"
		} else if !d.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file", rel)
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	// By path segment, so that a directory's entries follow it directly
	// ("a/" and "a/x" before "a-b/").
	sort.Slice(paths, func(i, j int) bool {
		return strings.ReplaceAll(paths[i], "/", "\x00") < strings.ReplaceAll(paths[j], "/", "\x00")
	})

	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	tw := tar.NewWriter(gz)
	file := func(name string, data []byte) error {
		h := &tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(data)), Format: tar.FormatPAX}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}
	if err := file(LicenseFile, license); err != nil {
		return nil, err
	}
	for _, rel := range paths {
		if strings.HasSuffix(rel, "/") {
			h := &tar.Header{Name: rel, Typeflag: tar.TypeDir, Mode: 0o755, Format: tar.FormatPAX}
			if err := tw.WriteHeader(h); err != nil {
				return nil, err
			}
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		if err := file(rel, data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Torrent returns a single-file .torrent for data, named name, whose only
// source is the web seed url (BEP 19). It has no tracker and no creation
// date.
func Torrent(name string, data []byte, url string) ([]byte, error) {
	var pieces []byte
	for off := 0; off < len(data); off += PieceLength {
		end := off + PieceLength
		if end > len(data) {
			end = len(data)
		}
		sum := sha1.Sum(data[off:end])
		pieces = append(pieces, sum[:]...)
	}
	info := map[string]any{
		"length":       int64(len(data)),
		"name":         name,
		"piece length": int64(PieceLength),
		"pieces":       string(pieces),
	}
	return Bencode(map[string]any{
		"created by": CreatedBy,
		"info":       info,
		"url-list":   []any{url},
	})
}

// Bencode encodes v: a string, an int64, a []any or a map[string]any, the
// keys in sorted order as BitTorrent requires.
func Bencode(v any) ([]byte, error) {
	var b bytes.Buffer
	if err := bencode(&b, v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func bencode(b *bytes.Buffer, v any) error {
	switch v := v.(type) {
	case string:
		b.WriteString(strconv.Itoa(len(v)))
		b.WriteByte(':')
		b.WriteString(v)
	case int64:
		b.WriteByte('i')
		b.WriteString(strconv.FormatInt(v, 10))
		b.WriteByte('e')
	case []any:
		b.WriteByte('l')
		for _, e := range v {
			if err := bencode(b, e); err != nil {
				return err
			}
		}
		b.WriteByte('e')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('d')
		for _, k := range keys {
			if err := bencode(b, k); err != nil {
				return err
			}
			if err := bencode(b, v[k]); err != nil {
				return err
			}
		}
		b.WriteByte('e')
	default:
		return fmt.Errorf("cannot bencode %T", v)
	}
	return nil
}

// Pins returns the Starlark file that rules_kicad loads: the version, the
// base URL, and each library's integrity.
func Pins(version, baseURL string, libs []Library) (string, error) {
	if err := checkBase(baseURL); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# SPDX-License-Identifier: Apache-2.0\n")
	b.WriteString(`"""The published KiCad 3D model libraries. Written by bazel run //tools:models3d; do not edit."""` + "\n\n")
	fmt.Fprintf(&b, "MODELS_VERSION = %q\n\n", version)
	fmt.Fprintf(&b, "MODELS_BASE_URL = %q\n\n", baseURL)
	b.WriteString("# Library name: the integrity of <name>.3dshapes.tar.gz.\n")
	b.WriteString("MODELS = {\n")
	for _, l := range libs {
		fmt.Fprintf(&b, "    %q: %q,\n", l.Name, l.SHA256)
	}
	b.WriteString("}\n")
	return b.String(), nil
}

// checkBase checks that a base URL is absolute and ends in "/".
func checkBase(baseURL string) error {
	u, err := url.Parse(baseURL)
	if err != nil || !u.IsAbs() || !strings.HasSuffix(baseURL, "/") {
		return fmt.Errorf("base URL %q must be absolute and end in /", baseURL)
	}
	return nil
}

// FileURL returns the URL of file name under baseURL, the name escaped.
func FileURL(baseURL, name string) string { return baseURL + url.PathEscape(name) }

// Tries is how often Upload tries each request before it gives up.
const Tries = 3

// Upload PUTs each named file of dir to baseURL with HTTP basic auth, then
// reads its length back with HEAD, also with the login. A request that
// fails on the network or with a 5xx status is tried again, up to Tries
// times. The server creates directories itself.
func Upload(client *http.Client, dir, baseURL, user, password string, files []string, log io.Writer) error {
	if err := checkBase(baseURL); err != nil {
		return err
	}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return err
		}
		u := FileURL(baseURL, f)
		put := func() (*http.Request, error) {
			req, err := http.NewRequest(http.MethodPut, u, bytes.NewReader(data))
			if err != nil {
				return nil, err
			}
			req.ContentLength = int64(len(data))
			return req, nil
		}
		status, _, err := do(client, put, user, password)
		if err != nil {
			return fmt.Errorf("PUT %s: %w", u, err)
		}
		if status != http.StatusCreated && status != http.StatusNoContent && status != http.StatusOK {
			return fmt.Errorf("PUT %s: status %d", u, status)
		}
		head := func() (*http.Request, error) { return http.NewRequest(http.MethodHead, u, nil) }
		status, length, err := do(client, head, user, password)
		if err != nil {
			return fmt.Errorf("HEAD %s: %w", u, err)
		}
		if status != http.StatusOK || length != int64(len(data)) {
			return fmt.Errorf("HEAD %s after upload: status %d, %d bytes, want %d", u, status, length, len(data))
		}
		fmt.Fprintf(log, "uploaded %s (%d bytes)\n", u, len(data))
	}
	return nil
}

// do sends the request that mk makes, with the login, up to Tries times
// while it fails on the network or with a 5xx status.
func do(client *http.Client, mk func() (*http.Request, error), user, password string) (int, int64, error) {
	var last error
	for try := 1; try <= Tries; try++ {
		req, err := mk()
		if err != nil {
			return 0, 0, err
		}
		req.SetBasicAuth(user, password)
		resp, err := client.Do(req)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return resp.StatusCode, resp.ContentLength, nil
			}
			err = fmt.Errorf("status %s", resp.Status)
		}
		last = err
		if try < Tries {
			time.Sleep(time.Duration(try) * RetryDelay)
		}
	}
	return 0, 0, fmt.Errorf("%d tries: %w", Tries, last)
}

// RetryDelay is the wait before the second try; the third waits twice as
// long. Tests set it to 0.
var RetryDelay = 5 * time.Second
