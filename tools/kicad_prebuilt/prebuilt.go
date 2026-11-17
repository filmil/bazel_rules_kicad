// SPDX-License-Identifier: Apache-2.0

// Package prebuilt holds what the KiCad prebuilt archive is made from (issue
// #100): the container image, the package snapshot, DwarFS's source and
// KiCad's AppImage, each pinned, and the podman command that runs build.sh
// (embedded) to make it.
package prebuilt

import (
	_ "embed"
	"fmt"
	"sort"
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

// Command returns the podman command line that makes the archive in out (an
// absolute directory) with jobs parallel compile jobs.
func Command(out string, jobs int) []string {
	args := []string{"podman", "run", "--rm", "-i",
		fmt.Sprintf("--cpus=%d", jobs),
		"-v", out + ":/out:Z",
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
