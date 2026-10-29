// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fake writes a small upstream archive and returns its path and sha256.
func fake(t *testing.T) (string, string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range []struct{ name, data string }{
		{"top/LICENSE.md", "license"},
		{"top/A.3dshapes/a.step", "a"},
	} {
		tw.WriteHeader(&tar.Header{Name: f.name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(f.data))})
		tw.Write([]byte(f.data))
	}
	tw.Close()
	gz.Close()
	p := filepath.Join(t.TempDir(), "up.tar.gz")
	os.WriteFile(p, buf.Bytes(), 0o644)
	sum := sha256.Sum256(buf.Bytes())
	return p, hex.EncodeToString(sum[:])
}

func TestRun(t *testing.T) {
	archive, sum := fake(t)
	dir := t.TempDir()
	pins := filepath.Join(dir, "pins.bzl")
	if err := run(archive, sum, filepath.Join(dir, "out")+"/", defaultBase, pins, false, "", ""); err != nil {
		t.Fatal(err)
	}
	p, err := os.ReadFile(pins)
	if err != nil || !strings.Contains(string(p), `"A": "sha256-`) {
		t.Errorf("pins %q, %v", p, err)
	}
	// The work directory is beside out, not in it, and gone at the end.
	if _, err := os.Stat(filepath.Join(dir, "out", ".work")); err == nil {
		t.Error("the work directory is inside out")
	}
	if _, err := os.Stat(filepath.Join(dir, "out.work")); err == nil {
		t.Error("the work directory is left behind")
	}
}

func TestRunRefuses(t *testing.T) {
	archive, sum := fake(t)
	for name, err := range map[string]error{
		"wrong sha256":     run(archive, strings.Repeat("0", 64), filepath.Join(t.TempDir(), "o"), defaultBase, "", false, "", ""),
		"no archive":       run("", sum, filepath.Join(t.TempDir(), "o"), defaultBase, "", false, "", ""),
		"upload, no user":  run(archive, sum, filepath.Join(t.TempDir(), "o"), defaultBase, "", true, "", "pw"),
		"upload, no login": run(archive, sum, filepath.Join(t.TempDir(), "o"), defaultBase, "", true, "u", ""),
	} {
		if err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
