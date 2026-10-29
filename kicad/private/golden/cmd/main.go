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

	"github.com/bazelbuild/rules_go/go/runfiles"

	"github.com/filmil/bazel_rules_kicad/kicad/private/golden"
)

func main() {
	if err := golden.Main(os.Getenv, runfiles.Rlocation, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "golden:", err)
		os.Exit(1)
	}
}
