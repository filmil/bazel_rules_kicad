// SPDX-License-Identifier: Apache-2.0

// Command drucanary writes a copy of a KiCad design rules file (.kicad_dru)
// with one more rule at the end: the canary (issue #17).
//
// Usage:
//
//	drucanary IN OUT
//
// KiCad drops a rule file it cannot read without a word: DRC then finds no
// violation and exits 0, even with --exit-code-violations
// (https://gitlab.com/kicad/code/kicad/-/work_items/25848). The canary
// rule asks for tracks at least 47.11 mm wide. kicad_drc_report runs DRC
// with OUT on a canary board that has one narrow track: if the report has
// that rule's violation, KiCad read the whole file; if not, it dropped it.
// Last in the file, the canary wins over any rule before it.
//
// IN is never changed: changing a rule file needs a person (AGENTS.md
// rule 7).
package main

import (
	"fmt"
	"os"
)

// Width is the canary's minimum track width, in the form KiCad writes it in
// a DRC report ("min width 47.1100 mm").
const Width = "47.1100 mm"

// Rule is the canary rule.
const Rule = `(rule "rules_kicad_canary"
  (constraint track_width (min 47.11mm)))
`

// Append returns the rule file with the canary rule after it.
func Append(rules []byte) []byte {
	out := append([]byte{}, rules...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return append(out, "\n# Appended by rules_kicad: the canary of kicad_drc_test.\n"+Rule...)
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: drucanary IN OUT")
		os.Exit(2)
	}
	in, err := os.ReadFile(os.Args[1])
	if err == nil {
		err = os.WriteFile(os.Args[2], Append(in), 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "drucanary:", err)
		os.Exit(1)
	}
}
