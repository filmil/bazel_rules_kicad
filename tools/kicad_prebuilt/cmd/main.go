// SPDX-License-Identifier: Apache-2.0

// Command kicad_prebuilt makes the KiCad prebuilt archive that the KiCad
// toolchain downloads, in the pinned Ubuntu container: it builds DwarFS's
// dwarfsextract from source and unpacks KiCad's AppImage with it. The
// operator runs it by hand, with podman on the host:
//
//	bazel run //tools:kicad_prebuilt -- --out=/tmp/kicad-prebuilt
//
// A relative --out is taken from the directory bazel run was started in.
// It fails unless the archive's sha256 is the one the toolchain pins. It is
// not part of any build: tools/mirror checks and uploads its output. With
// --dry_run it prints the podman command and the script instead.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/filmil/bazel_rules_kicad/tools/kicad_prebuilt"
)

func fileSHA256(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func run() error {
	out := flag.String("out", "", "an empty directory for the archive, its two inputs and SHA256SUMS (required)")
	jobs := flag.Int("jobs", 4, "parallel compile jobs, and the container's CPUs")
	dry := flag.Bool("dry_run", false, "print the command and the script; run nothing")
	flag.Parse()
	if *out == "" {
		return fmt.Errorf("--out is required")
	}
	if *jobs < 1 {
		return fmt.Errorf("--jobs must be 1 or more")
	}
	// bazel run starts the tool in its runfiles; a relative --out means the
	// directory bazel run was started in.
	abs := *out
	if !filepath.IsAbs(abs) {
		wd := os.Getenv("BUILD_WORKING_DIRECTORY")
		if wd == "" {
			return fmt.Errorf("--out %q is relative and BUILD_WORKING_DIRECTORY is not set; give an absolute path", abs)
		}
		abs = filepath.Join(wd, abs)
	}
	cmd := prebuilt.Command(abs, *jobs)
	if *dry {
		fmt.Println(strings.Join(cmd, " "))
		fmt.Print(prebuilt.BuildScript)
		return nil
	}
	// An old archive left in --out would pass for a new one.
	if entries, err := os.ReadDir(abs); err == nil && len(entries) > 0 {
		return fmt.Errorf("--out %s is not empty", abs)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Stdin = strings.NewReader(prebuilt.BuildScript)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return err
	}
	archive := filepath.Join(abs, prebuilt.Prebuilt+".tar.xz")
	got, err := fileSHA256(archive)
	if err != nil {
		return err
	}
	if got != prebuilt.PrebuiltSHA256 {
		return fmt.Errorf("%s has sha256 %s; the toolchain pins %s", archive, got, prebuilt.PrebuiltSHA256)
	}
	fmt.Printf("ok %s: sha256 %s, as pinned\n", archive, got)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "kicad_prebuilt:", err)
		os.Exit(1)
	}
}
