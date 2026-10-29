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
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
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
	if !strings.HasSuffix(baseURL, "/") {
		return nil, fmt.Errorf("base URL %q must end in /", baseURL)
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
		t, err := Torrent(lib.Archive, data, baseURL+lib.Archive)
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
// directory. Only directories and regular files are kept; a path that
// would leave dir is an error.
func extract(r io.Reader, dir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := path.Clean(h.Name)
		i := strings.IndexByte(name, '/')
		if i < 0 {
			continue // the top directory, or a file beside it
		}
		rel := name[i+1:]
		if rel == "" || rel == "." || strings.HasPrefix(rel, "../") || rel == ".." || path.IsAbs(rel) {
			return fmt.Errorf("unsafe path %q", h.Name)
		}
		p := filepath.Join(dir, filepath.FromSlash(rel))
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
		}
	}
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
	sort.Strings(paths)

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
func Pins(version, baseURL string, libs []Library) string {
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
	return b.String()
}

// Upload PUTs each named file of dir to baseURL with HTTP basic auth, then
// reads its length back with HEAD. The server creates directories itself.
func Upload(client *http.Client, dir, baseURL, user, password string, files []string, log io.Writer) error {
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return err
		}
		url := baseURL + f
		req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
		if err != nil {
			return err
		}
		req.SetBasicAuth(user, password)
		req.ContentLength = int64(len(data))
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("PUT %s: %s", url, resp.Status)
		}
		head, err := client.Head(url)
		if err != nil {
			return err
		}
		head.Body.Close()
		if head.StatusCode != http.StatusOK || head.ContentLength != int64(len(data)) {
			return fmt.Errorf("HEAD %s after upload: %s, %d bytes, want %d", url, head.Status, head.ContentLength, len(data))
		}
		fmt.Fprintf(log, "uploaded %s (%d bytes)\n", url, len(data))
	}
	return nil
}
