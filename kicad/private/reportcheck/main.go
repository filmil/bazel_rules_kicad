// SPDX-License-Identifier: Apache-2.0

// Command reportcheck is the test behind kicad_erc_test and kicad_drc_test:
// it reads a KiCad ERC or DRC report (JSON), prints a summary, keeps the
// report as a test output, and fails if the design has violations.
//
// It runs as a test, with:
//
//   - REPORT: the report, relative to the main repository's runfiles
//     directory;
//   - REPORT_EXPECT: empty for a design that must be clean, or a violation
//     type (such as pin_not_connected) for a known-bad design, which then
//     must have at least one violation of that type (spec T3).
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

// Check returns the test's message and exit status.
func Check(b []byte, expect string) (string, int) {
	kind, vs, err := Parse(b)
	if err != nil {
		return "reportcheck: " + err.Error(), 2
	}
	msg := Summary(kind, vs)
	if expect == "" {
		if len(vs) > 0 {
			return msg + "the design must have no violations", 1
		}
		return msg, 0
	}
	for _, v := range vs {
		if v.Type == expect {
			return msg + fmt.Sprintf("found a %s violation, as expected", expect), 0
		}
	}
	return msg + fmt.Sprintf("expected a %s violation; this known-bad design must have one", expect), 1
}

func main() {
	p := filepath.Join(os.Getenv("TEST_SRCDIR"), os.Getenv("TEST_WORKSPACE"), os.Getenv("REPORT"))
	b, err := os.ReadFile(p)
	if err != nil {
		fmt.Fprintln(os.Stderr, "reportcheck:", err)
		os.Exit(2)
	}
	if d := os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR"); d != "" {
		if err := os.WriteFile(filepath.Join(d, filepath.Base(p)), b, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "reportcheck:", err)
			os.Exit(2)
		}
	}
	msg, code := Check(b, os.Getenv("REPORT_EXPECT"))
	fmt.Println(msg)
	os.Exit(code)
}
