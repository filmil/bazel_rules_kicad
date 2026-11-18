// SPDX-License-Identifier: Apache-2.0

// Command kicad_prebuilt makes the KiCad prebuilt archive that the KiCad
// toolchain downloads, in the pinned Ubuntu container: it builds DwarFS's
// dwarfsextract from source and unpacks KiCad's AppImage with it. The
// operator runs it by hand, with podman on the host:
//
//	bazel run //tools:kicad_prebuilt -- --out=/tmp/kicad-prebuilt
//
// --out must be missing or empty; a relative one is taken from the
// directory bazel run was started in. It receives the archive, the two
// inputs as downloaded and SHA256SUMS, and stays empty if the run fails.
// The tool fails unless the archive's sha256 is the one the toolchain
// pins. It is not part of any build: tools/mirror checks and uploads its
// output. With --dry_run it prints the command, the script as a
// here-document, and runs nothing.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/filmil/bazel_rules_kicad/tools/kicad_prebuilt"
)

func run() error {
	out := flag.String("out", "", "a missing or empty directory for the archive, its two inputs and SHA256SUMS (required)")
	jobs := flag.Int("jobs", 4, "parallel compile jobs, and the container's CPUs")
	dry := flag.Bool("dry_run", false, "print the command, with the script as a here-document; run nothing")
	flag.Parse()
	if *jobs < 1 {
		return fmt.Errorf("--jobs must be 1 or more")
	}
	abs, err := prebuilt.ResolveOut(*out, os.Getenv("BUILD_WORKING_DIRECTORY"))
	if err != nil {
		return err
	}
	if *dry {
		ca, err := prebuilt.FindCABundle()
		if err != nil {
			return err
		}
		// The run mounts a copy of it; the dry run names the bundle itself.
		fmt.Print(prebuilt.DryRun(prebuilt.Command(abs, ca, *jobs)))
		return nil
	}
	tmp, err := os.MkdirTemp("", "kicad-prebuilt-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	ca, err := prebuilt.CopyCABundle(tmp)
	if err != nil {
		return err
	}
	cmd := prebuilt.Command(abs, ca, *jobs)
	if err := prebuilt.CheckOut(abs); err != nil {
		return err
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
	if err := prebuilt.VerifyArchive(archive); err != nil {
		return err
	}
	fmt.Printf("ok %s: sha256 %s, as pinned\n", archive, prebuilt.PrebuiltSHA256)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "kicad_prebuilt:", err)
		os.Exit(1)
	}
}
