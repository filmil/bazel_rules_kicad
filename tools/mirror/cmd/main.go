// SPDX-License-Identifier: Apache-2.0

// Command mirror checks the prebuilt archives of tools/mirror/manifest.json
// and, for the operator, uploads them as release assets.
//
// Check every upstream file (what agents run; this is the default):
//
//	bazel run //tools:mirror
//
// Upload (the operator only; it needs gh, logged in with write access):
//
//	bazel run //tools:mirror -- --dry_run=false --built_dir=/path/to/built
//
// --built_dir holds the built assets by name, such as the KiCad prebuilt
// that make-kicad-prebuilt.sh makes. Each command is printed before it
// runs. An upload stops before running any command if a file does not
// match the manifest. gh refuses to replace an asset that is already in
// the release.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/filmil/bazel_rules_kicad/tools/mirror"
)

func run() error {
	root := flag.String("root", os.Getenv("BUILD_WORKSPACE_DIRECTORY"),
		"repository root (default: the workspace of `bazel run`)")
	manifestPath := flag.String("manifest", "tools/mirror/manifest.json", "the manifest, relative to --root")
	dryRun := flag.Bool("dry_run", true, "check the files only; --dry_run=false uploads them")
	builtDir := flag.String("built_dir", "", "a directory with the built assets, by name")
	release := flag.String("release", "", "only this release tag (default: all)")
	workDir := flag.String("work_dir", "", "where to download (default: a new temporary directory)")
	flag.Parse()
	if *root == "" {
		return fmt.Errorf("no root; run it with `bazel run //tools:mirror` or pass --root")
	}
	b, err := os.ReadFile(filepath.Join(*root, *manifestPath))
	if err != nil {
		return err
	}
	m, err := mirror.Parse(b)
	if err != nil {
		return err
	}
	if *workDir == "" {
		d, err := os.MkdirTemp("", "mirror-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(d)
		*workDir = d
	}
	opts := mirror.Options{
		Client:   &http.Client{Timeout: 30 * time.Minute},
		WorkDir:  *workDir,
		BuiltDir: *builtDir,
		Log:      os.Stdout,
	}
	found := false
	for _, r := range m.Releases {
		if *release != "" && r.Tag != *release {
			continue
		}
		found = true
		ready, err := mirror.Check(r, opts)
		if err != nil {
			return err
		}
		var files []string
		for _, a := range r.Assets {
			switch p, ok := ready[a.Name]; {
			case ok:
				files = append(files, p)
			case *dryRun:
				// Where the upload will look for the built file.
				files = append(files, filepath.Join("BUILT_DIR", a.Name))
			default:
				return fmt.Errorf("%s/%s: built by %s; give --built_dir to upload it", r.Tag, a.Name, a.Built.Script)
			}
		}
		create := mirror.CreateCommand(m.Repo, r)
		upload := mirror.UploadCommand(m.Repo, r, files)
		if *dryRun {
			fmt.Println("would run, if the release is missing:", mirror.Quote(create))
			fmt.Println("would run:", mirror.Quote(upload))
			continue
		}
		if exec.Command("gh", "release", "view", r.Tag, "--repo", m.Repo).Run() != nil {
			if err := runCommand(create); err != nil {
				return err
			}
		}
		if err := runCommand(upload); err != nil {
			return err
		}
	}
	if !found {
		return fmt.Errorf("no release %q in the manifest", *release)
	}
	if *dryRun {
		fmt.Println("dry run: every file checked matches the manifest; nothing uploaded")
	}
	return nil
}

func runCommand(argv []string) error {
	fmt.Println("running:", mirror.Quote(argv))
	c := exec.Command(argv[0], argv[1:]...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mirror:", err)
		os.Exit(1)
	}
}
