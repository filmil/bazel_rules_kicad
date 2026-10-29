// SPDX-License-Identifier: Apache-2.0

// Command reportcheck is the test behind kicad_erc_test and kicad_drc_test:
// it reads a KiCad ERC or DRC report (JSON), prints a summary, keeps the
// report as a test output, and fails if the design has violations.
//
// It runs as a test, with:
//
//   - REPORT: the report's runfiles path (its rlocationpath: the
//     repository's directory, then the path in it), relative to
//     TEST_SRCDIR;
//   - REPORT_EXPECT: empty for a design that must be clean, or a violation
//     type (such as pin_not_connected) for a known-bad design (spec T3),
//     which then must have violations of that type and of no other, so a
//     second, unintended problem cannot hide behind the expected one.
//
// Entries with severity "exclusion" (marked as excluded in the design)
// or "ignore" do not count.
//
// CANARY_REPORT, when set, is the runfiles path of the DRC report of the
// canary board, run with the design rules and a canary rule appended
// (kicad_drc_report, issue #17). It must have the canary rule's violation:
// if not, KiCad did not read the rule file, and the check reports the
// violation type design_rules_not_loaded.
//
// It copies the report into TEST_UNDECLARED_OUTPUTS_DIR, so `bazel test`
// keeps it in bazel-testlogs.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Violation is one problem in a report.
type Violation struct {
	Kind        string // "violation", "unconnected" or "parity"
	Severity    string
	Type        string
	Description string
	Items       []string
	Sheet       string // ERC only
}

type item struct {
	Description string `json:"description"`
}

type entry struct {
	Severity    string `json:"severity"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Items       []item `json:"items"`
}

type report struct {
	Schema string `json:"$schema"`
	Source string `json:"source"`
	// ERC
	Sheets []struct {
		Path       string  `json:"path"`
		Violations []entry `json:"violations"`
	} `json:"sheets"`
	// DRC
	Violations  []entry `json:"violations"`
	Unconnected []entry `json:"unconnected_items"`
	Parity      []entry `json:"schematic_parity"`
}

func add(out []Violation, kind, sheet string, es []entry) []Violation {
	for _, e := range es {
		if e.Severity == "exclusion" || e.Severity == "ignore" {
			continue
		}
		v := Violation{Kind: kind, Severity: e.Severity, Type: e.Type, Description: e.Description, Sheet: sheet}
		for _, i := range e.Items {
			v.Items = append(v.Items, i.Description)
		}
		out = append(out, v)
	}
	return out
}

// Parse returns the violations of an ERC or DRC report, and which it is.
func Parse(b []byte) (string, []Violation, error) {
	var r report
	if err := json.Unmarshal(b, &r); err != nil {
		return "", nil, err
	}
	var vs []Violation
	switch {
	case strings.Contains(r.Schema, "schemas.kicad.org/erc."):
		for _, s := range r.Sheets {
			vs = add(vs, "violation", s.Path, s.Violations)
		}
		return "ERC", vs, nil
	case strings.Contains(r.Schema, "schemas.kicad.org/drc."):
		vs = add(vs, "violation", "", r.Violations)
		vs = add(vs, "unconnected", "", r.Unconnected)
		vs = add(vs, "parity", "", r.Parity)
		return "DRC", vs, nil
	}
	return "", nil, fmt.Errorf("not a KiCad ERC or DRC report (schema %q)", r.Schema)
}

// Summary is what the test prints.
func Summary(kind string, vs []Violation) string {
	var b strings.Builder
	if len(vs) == 0 {
		fmt.Fprintf(&b, "%s: no violations\n", kind)
		return b.String()
	}
	counts := map[string]int{}
	for _, v := range vs {
		counts[v.Severity+" "+v.Type]++
	}
	var keys []string
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprintf(&b, "%s: %d violations\n", kind, len(vs))
	for _, k := range keys {
		fmt.Fprintf(&b, "  %3d  %s\n", counts[k], k)
	}
	for _, v := range vs {
		where := ""
		if v.Sheet != "" {
			where = " (sheet " + v.Sheet + ")"
		}
		fmt.Fprintf(&b, "%s %s%s: %s\n", v.Severity, v.Type, where, v.Description)
		for _, i := range v.Items {
			fmt.Fprintf(&b, "    - %s\n", i)
		}
	}
	return b.String()
}

// CanaryWidth is the canary rule's minimum track width as KiCad writes it in
// a DRC report (kicad/private/drucanary).
const CanaryWidth = "min width 47.1100 mm"

// CanaryFound reports whether a canary DRC report has the canary rule's
// violation.
func CanaryFound(b []byte) (bool, error) {
	kind, vs, err := Parse(b)
	if err != nil {
		return false, err
	}
	if kind != "DRC" {
		return false, fmt.Errorf("the canary report is an %s report, not DRC", kind)
	}
	for _, v := range vs {
		if v.Type == "track_width" && strings.Contains(v.Description, CanaryWidth) {
			return true, nil
		}
	}
	return false, nil
}

// NotLoaded is the violation that stands for a rule file KiCad dropped.
var NotLoaded = Violation{
	Kind:        "violation",
	Severity:    "error",
	Type:        "design_rules_not_loaded",
	Description: "KiCad did not read the design rules (.kicad_dru): the canary rule appended to them had no effect. Check the rule file's syntax; KiCad drops a file it cannot read without a word (https://gitlab.com/kicad/code/kicad/-/work_items/25848).",
}

// Check returns the test's message and exit status. canary is the canary
// report, or nil when the board has no design rules.
func Check(b []byte, expect string, canary ...[]byte) (string, int) {
	kind, vs, err := Parse(b)
	if err != nil {
		return "reportcheck: " + err.Error(), 2
	}
	if len(canary) > 0 && canary[0] != nil {
		ok, err := CanaryFound(canary[0])
		if err != nil {
			return "reportcheck: canary: " + err.Error(), 2
		}
		if !ok {
			vs = append([]Violation{NotLoaded}, vs...)
		}
	}
	msg := Summary(kind, vs)
	if expect == "" {
		if len(vs) > 0 {
			return msg + "the design must have no violations", 1
		}
		return msg, 0
	}
	found := 0
	var others []string
	for _, v := range vs {
		if v.Type == expect {
			found++
		} else {
			others = append(others, v.Type)
		}
	}
	switch {
	case found == 0:
		return msg + fmt.Sprintf("expected a %s violation; this known-bad design must have one", expect), 1
	case len(others) > 0:
		return msg + fmt.Sprintf("expected only %s violations; also found %v", expect, others), 1
	}
	return msg + fmt.Sprintf("found only %s violations, as expected", expect), 0
}

// Run is the test: it reads the report at srcdir/report (and the canary
// report, when set), keeps copies in outdir (when set), and checks them.
func Run(srcdir, report, outdir, expect string, canaryReport ...string) (string, int) {
	if srcdir == "" || report == "" {
		return "reportcheck: TEST_SRCDIR and REPORT must be set; run it as a test", 2
	}
	p := filepath.Join(srcdir, filepath.FromSlash(report))
	b, err := os.ReadFile(p)
	if err != nil {
		return "reportcheck: " + err.Error(), 2
	}
	var canary []byte
	if len(canaryReport) > 0 && canaryReport[0] != "" {
		canary, err = os.ReadFile(filepath.Join(srcdir, filepath.FromSlash(canaryReport[0])))
		if err != nil {
			return "reportcheck: " + err.Error(), 2
		}
	}
	if outdir != "" {
		if err := os.WriteFile(filepath.Join(outdir, filepath.Base(p)), b, 0o644); err != nil {
			return "reportcheck: " + err.Error(), 2
		}
		if canary != nil {
			if err := os.WriteFile(filepath.Join(outdir, "canary.json"), canary, 0o644); err != nil {
				return "reportcheck: " + err.Error(), 2
			}
		}
	}
	return Check(b, expect, canary)
}

func main() {
	msg, code := Run(os.Getenv("TEST_SRCDIR"), os.Getenv("REPORT"), os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR"), os.Getenv("REPORT_EXPECT"), os.Getenv("CANARY_REPORT"))
	fmt.Println(msg)
	os.Exit(code)
}
