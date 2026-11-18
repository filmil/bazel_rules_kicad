// SPDX-License-Identifier: Apache-2.0

// Package prebuilt holds what the KiCad prebuilt archive is made from (issue
// #100): the container image, the package snapshot, DwarFS's source and
// KiCad's AppImage, each pinned, and the podman command that runs build.sh
// (embedded) to make it.
package prebuilt

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Image is Ubuntu 26.04, the image DwarFS 0.15.10's own CI builds in
// (.docker/Dockerfile.ubuntu), pinned by digest.
const Image = "docker.io/library/ubuntu:26.04@sha256:5e275723f82c67e387ba9e3c24baa0abdcb268917f276a0561c97bef9450d0b4"

// AptSnapshot is the time of snapshot.ubuntu.com every package comes from,
// so each package's version is fixed.
const AptSnapshot = "20261117T000000Z"

// Input is one downloaded file, pinned by sha256.
type Input struct {
	URL    string
	SHA256 string
}

// DwarFS is DwarFS 0.15.10's release source tarball.
var DwarFS = Input{
	URL:    "https://github.com/mhx/dwarfs/releases/download/v0.15.10/dwarfs-0.15.10.tar.xz",
	SHA256: "036109bc210b5afa593a005f798918aacdd412a3d9dbf703bed2e70477454027",
}

// AppImage is KiCad 9.0.9.1's lite AppImage, as the tarball KiCad
// publishes it in.
var AppImage = Input{
	URL:    "https://kicad-downloads.s3.cern.ch/appimage/stable/kicad-9.0.9.1-x86_64-lite.AppImage.tar",
	SHA256: "401ed5e3e32bcf96af89d9741597670107b6a7c30b550518739b2512e6e92b63",
}

// AppImageName is the AppImage inside AppImage's tarball.
const AppImageName = "kicad-9.0.9.1-x86_64-lite.AppImage"

// Prebuilt is the archive's name, without .tar.xz.
const Prebuilt = "kicad-9.0.9.1-linux-x86_64"

// PrebuiltSHA256 is the archive's sha256: what kicad/private/extensions.bzl
// pins and tools/mirror/manifest.json mirrors. The tool fails on any other.
const PrebuiltSHA256 = "7f570bd2161a2dbdd0dca9ef7ba0ab751d422b379c6209385491ecc047f93bff"

//go:embed build.sh
var BuildScript string

// CABundles are where Linux distributions keep the CA bundle, in the order
// tried. apt in the container checks snapshot.ubuntu.com against it; it
// verifies the connection only, and reaches no output.
var CABundles = []string{
	"/etc/ssl/certs/ca-certificates.crt", // Debian, Ubuntu, Arch
	"/etc/pki/tls/certs/ca-bundle.crt",   // Fedora, RHEL
	"/etc/ssl/ca-bundle.pem",             // openSUSE
}

// CopyCABundle copies the first of CABundles that exists into dir, and
// returns the copy's path. The container mounts the copy, never the host's
// file: podman's :Z relabels what it mounts.
func CopyCABundle(dir string) (string, error) {
	src, err := FindCABundle()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	dst := filepath.Join(dir, "ca.crt")
	return dst, os.WriteFile(dst, b, 0o444)
}

// FindCABundle returns the first of CABundles that exists.
func FindCABundle() (string, error) {
	for _, p := range CABundles {
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			return p, nil
		}
	}
	return "", fmt.Errorf("no CA bundle at any of %v", CABundles)
}

// Command returns the podman command line that makes the archive in out (an
// absolute directory with no ":") with jobs parallel compile jobs. The only
// host files the container sees are out and ca, read-only: the tool's own
// copy of the CA bundle (CopyCABundle).
func Command(out, ca string, jobs int) []string {
	args := []string{"podman", "run", "--rm", "-i",
		"--platform=linux/amd64",
		fmt.Sprintf("--cpus=%d", jobs),
		"-v", out + ":/out:Z",
		"-v", ca + ":/host-ca.crt:ro,Z",
	}
	env := []string{
		"APT_SNAPSHOT=" + AptSnapshot,
		"DWARFS_URL=" + DwarFS.URL,
		"DWARFS_SHA256=" + DwarFS.SHA256,
		"APPIMAGE_URL=" + AppImage.URL,
		"APPIMAGE_SHA256=" + AppImage.SHA256,
		"APPIMAGE_NAME=" + AppImageName,
		"PREBUILT=" + Prebuilt,
		fmt.Sprintf("JOBS=%d", jobs),
	}
	sort.Strings(env)
	for _, e := range env {
		args = append(args, "-e", e)
	}
	// The script comes on standard input: bash -s reads it from there.
	return append(args, Image, "bash", "-s")
}

// ResolveOut makes --out absolute: a relative one is taken from wd, the
// directory bazel run was started in. podman's -v splits on ":", so a path
// with one is refused.
func ResolveOut(out, wd string) (string, error) {
	if out == "" {
		return "", fmt.Errorf("--out is required")
	}
	abs := out
	if !filepath.IsAbs(abs) {
		if !filepath.IsAbs(wd) {
			return "", fmt.Errorf("--out %q is relative, and the working directory %q is not absolute; give an absolute --out", out, wd)
		}
		abs = filepath.Join(wd, abs)
	}
	abs = filepath.Clean(abs)
	if strings.Contains(abs, ":") {
		return "", fmt.Errorf("--out %q has a \":\", which podman's -v cannot take", abs)
	}
	return abs, nil
}

// CheckOut fails unless dir is missing or empty: an old archive left there
// would pass for a new one.
func CheckOut(dir string) error {
	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("--out: %w", err)
	case len(entries) > 0:
		return fmt.Errorf("--out %s is not empty", dir)
	}
	return nil
}

// VerifyArchive fails unless the file at p has the sha256 the toolchain
// pins.
func VerifyArchive(p string) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != PrebuiltSHA256 {
		return fmt.Errorf("%s has sha256 %s; the toolchain pins %s", p, got, PrebuiltSHA256)
	}
	return nil
}

var plainArg = regexp.MustCompile(`^[\w@%+=:,./-]+$`)

// Quote returns argv as one line a POSIX shell reads back as argv.
func Quote(argv []string) string {
	out := make([]string, len(argv))
	for i, a := range argv {
		if plainArg.MatchString(a) {
			out[i] = a
		} else {
			out[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return strings.Join(out, " ")
}

// DryRun returns what --dry_run prints: the podman command with the script
// as a here-document on its standard input, which a shell runs as it is.
func DryRun(argv []string) string {
	return Quote(argv) + " <<'BUILD_SH'\n" + strings.TrimSuffix(BuildScript, "\n") + "\nBUILD_SH\n"
}
