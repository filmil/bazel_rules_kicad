// SPDX-License-Identifier: Apache-2.0

// Command kicad_merge is a git merge driver for KiCad's S-expression files
// (issue #58). Git runs it as
//
//	kicad_merge %O %A %B %P
//
// with the common ancestor, ours (which it overwrites with the result) and
// theirs, and the path for messages. With --out FILE first, it writes the
// result there and leaves ours alone, as a build action must. It exits 0 on a clean merge. On a
// conflict it leaves ours as it was, names each conflicting element on
// standard error, and exits 1, so git marks the file conflicted. Install
// it for a repository:
//
//	# .gitattributes
//	*.kicad_sch merge=kicad
//	*.kicad_pcb merge=kicad
//	*.kicad_sym merge=kicad
//
//	git config merge.kicad.name "KiCad S-expression merge"
//	git config merge.kicad.driver "/path/to/kicad_merge %O %A %B %P"
package main

import (
	"fmt"
	"os"

	"github.com/filmil/bazel_rules_kicad/kicad/private/merge"
)

func run(args []string) (int, error) {
	out := ""
	if len(args) >= 2 && args[0] == "--out" {
		out, args = args[1], args[2:]
	}
	if len(args) < 3 || len(args) > 4 {
		return 2, fmt.Errorf("usage: kicad_merge [--out FILE] BASE OURS THEIRS [PATH]")
	}
	if out == "" {
		out = args[1]
	}
	name := args[1]
	if len(args) == 4 {
		name = args[3]
	}
	var text [3]string
	for i, p := range args[:3] {
		b, err := os.ReadFile(p)
		if err != nil {
			return 2, err
		}
		text[i] = string(b)
	}
	merged, conflicts, err := merge.Merge(text[0], text[1], text[2])
	if err != nil {
		return 1, fmt.Errorf("%s: %v", name, err)
	}
	for _, c := range conflicts {
		fmt.Fprintf(os.Stderr, "kicad_merge: %s: conflict: %s\n", name, c)
	}
	if len(conflicts) > 0 {
		return 1, nil
	}
	return 0, os.WriteFile(out, []byte(merged), 0o644)
}

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "kicad_merge:", err)
	}
	os.Exit(code)
}
