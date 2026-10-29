// SPDX-License-Identifier: Apache-2.0

// Command golden is the program behind golden_test and update_goldens.
//
// GOLDEN_MODE says what it does:
//
//   - check (golden_test): the built file must equal the golden file, byte
//     for byte. On a difference it prints the first different line and how
//     to update the golden file, and fails.
//   - update (update_goldens, under `bazel run`): it writes each built
//     file over its golden file in BUILD_WORKSPACE_DIRECTORY. Before that,
//     it fails if a golden_test of the module is missing from
//     GOLDEN_REGISTERED, so no golden file is left out of an update.
//
// GOLDEN_PAIRS lists the pairs, one per line: the test's label, the built
// file's runfiles path, the golden file's runfiles path, and the golden
// file's path in the workspace, separated by tabs. GOLDEN_REGISTERED lists
// the labels of the registered tests, one per line.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/bazelbuild/rules_go/go/runfiles"

	"github.com/filmil/bazel_rules_kicad/kicad/private/golden"
)

func run() error {
	pairs, err := golden.ParsePairs(os.Getenv("GOLDEN_PAIRS"))
	if err != nil {
		return err
	}
	if len(pairs) == 0 {
		return fmt.Errorf("GOLDEN_PAIRS is empty")
	}
	switch mode := os.Getenv("GOLDEN_MODE"); mode {
	case "check":
		var failed []string
		for _, p := range pairs {
			if err := golden.Check(p, runfiles.Rlocation, os.Stdout); err != nil {
				failed = append(failed, err.Error())
			}
		}
		if len(failed) > 0 {
			return fmt.Errorf("%s", strings.Join(failed, "\n"))
		}
		return nil
	case "update":
		root := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
		if root == "" {
			return fmt.Errorf("BUILD_WORKSPACE_DIRECTORY is not set: run this with `bazel run`")
		}
		missing, err := golden.Unregistered(root, golden.Lines(os.Getenv("GOLDEN_REGISTERED")))
		if err != nil {
			return err
		}
		if len(missing) > 0 {
			return fmt.Errorf("these golden_test targets are not in the tests of this update_goldens target; "+
				"add them, so that their golden files are updated too:\n  %s", strings.Join(missing, "\n  "))
		}
		n, err := golden.Update(pairs, root, runfiles.Rlocation, os.Stdout)
		if err != nil {
			return err
		}
		fmt.Printf("%d of %d golden files updated. Give the reason in the commit message.\n", n, len(pairs))
		return nil
	default:
		return fmt.Errorf("GOLDEN_MODE=%q: want check or update", mode)
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "golden:", err)
		os.Exit(1)
	}
}
