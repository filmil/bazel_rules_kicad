// SPDX-License-Identifier: Apache-2.0

// Command tablecheck checks KiCad library tables (sym-lib-table,
// fp-lib-table) for paths that depend on the host (spec H4).
//
// Every library URI must start with ${KIPRJMOD}/ and must not name any other
// variable: an absolute path, a home-relative path, or a variable such as
// ${KICAD9_SYMBOL_DIR} would make KiCad read libraries from wherever the
// host has them.
//
// It runs as a test. TABLECHECK_FILES lists the tables, one path per line,
// relative to the runfiles directory. TABLECHECK_EXPECT is "clean" (the
// default: every table must pass) or "host" (this rule's own known-bad
// test: some table must fail).
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// uriRe finds each (uri "...") of a table. KiCad writes table strings with
// backslash escapes for quotes and backslashes.
var uriRe = regexp.MustCompile(`\(uri\s+"((?:[^"\\]|\\.)*)"\)`)

var varRe = regexp.MustCompile(`\$\{([^}]*)\}|\$\(([^)]*)\)`)

// Problem is one host-dependent URI.
type Problem struct {
	File, URI, Why string
}

func (p Problem) String() string { return fmt.Sprintf("%s: uri %q %s", p.File, p.URI, p.Why) }

// CheckURI returns why uri depends on the host, or "".
func CheckURI(uri string) string {
	for _, m := range varRe.FindAllStringSubmatch(uri, -1) {
		name := m[1] + m[2]
		if name != "KIPRJMOD" {
			return fmt.Sprintf("names the variable %s, which the host sets", name)
		}
	}
	if !strings.HasPrefix(uri, "${KIPRJMOD}/") {
		return "does not start with ${KIPRJMOD}/, so it is not relative to the project"
	}
	return ""
}

// CheckTable returns the problems of one table's text. A table with no
// library is fine; a text that is not a table is a problem.
func CheckTable(file, text string) []Problem {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "(sym_lib_table") && !strings.HasPrefix(t, "(fp_lib_table") {
		return []Problem{{File: file, Why: "is not a sym-lib-table or fp-lib-table"}}
	}
	var ps []Problem
	for _, m := range uriRe.FindAllStringSubmatch(text, -1) {
		if why := CheckURI(m[1]); why != "" {
			ps = append(ps, Problem{File: file, URI: m[1], Why: why})
		}
	}
	if n := strings.Count(text, "(lib "); n != len(uriRe.FindAllString(text, -1)) {
		ps = append(ps, Problem{File: file, Why: fmt.Sprintf("has %d libraries but %d readable URIs", n, len(uriRe.FindAllString(text, -1)))})
	}
	return ps
}

func run(files []string, expect string) (string, int) {
	var all []Problem
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return fmt.Sprintf("tablecheck: %v", err), 2
		}
		all = append(all, CheckTable(f, string(b))...)
	}
	var sb strings.Builder
	for _, p := range all {
		sb.WriteString(p.String() + "\n")
	}
	switch expect {
	case "", "clean":
		if len(all) > 0 {
			return sb.String() + "library tables depend on the host", 1
		}
		return fmt.Sprintf("%d tables checked, all paths relative to the project", len(files)), 0
	case "host":
		if len(all) == 0 {
			return "expected a host-dependent path, found none: the check is broken", 1
		}
		return sb.String() + "found, as expected", 0
	}
	return fmt.Sprintf("tablecheck: TABLECHECK_EXPECT=%q: want clean or host", expect), 2
}

func main() {
	var files []string
	for _, f := range strings.Split(os.Getenv("TABLECHECK_FILES"), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "tablecheck: TABLECHECK_FILES is empty")
		os.Exit(2)
	}
	msg, code := run(files, os.Getenv("TABLECHECK_EXPECT"))
	fmt.Println(msg)
	os.Exit(code)
}
