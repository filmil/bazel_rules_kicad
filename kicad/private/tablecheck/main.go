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
	"strings"
)

// Problem is one host-dependent URI, or a table that cannot be read.
type Problem struct {
	File, URI, Why string
}

func (p Problem) String() string {
	if p.URI == "" {
		return fmt.Sprintf("%s: %s", p.File, p.Why)
	}
	return fmt.Sprintf("%s: uri %q %s", p.File, p.URI, p.Why)
}

// CheckURI returns why uri depends on the host, or "". The only form
// allowed is ${KIPRJMOD}/ followed by a path with no other variable.
func CheckURI(uri string) string {
	const prefix = "${KIPRJMOD}/"
	if !strings.HasPrefix(uri, prefix) {
		return "does not start with ${KIPRJMOD}/, so it is not relative to the project"
	}
	rest := uri[len(prefix):]
	if rest == "" {
		return "names the project directory itself, not a library"
	}
	if strings.Contains(rest, "$") {
		return "names a variable after ${KIPRJMOD}/; the host may set it"
	}
	if strings.HasPrefix(rest, "/") || strings.HasPrefix(rest, "~") {
		return "is absolute after ${KIPRJMOD}/"
	}
	return ""
}

// node is an S-expression: an atom (a symbol or a string) or a list.
type node struct {
	atom   string
	quoted bool
	list   []*node
	isList bool
}

// parse reads one S-expression from KiCad's format: parentheses, quoted
// strings with backslash escapes, and bare symbols.
func parse(text string) (*node, error) {
	p := &parser{s: text}
	p.space()
	n, err := p.node()
	if err != nil {
		return nil, err
	}
	p.space()
	if p.i != len(p.s) {
		return nil, fmt.Errorf("text after the table at byte %d", p.i)
	}
	return n, nil
}

type parser struct {
	s string
	i int
}

func (p *parser) space() {
	for p.i < len(p.s) && strings.ContainsRune(" \t\r\n", rune(p.s[p.i])) {
		p.i++
	}
}

func (p *parser) node() (*node, error) {
	if p.i >= len(p.s) {
		return nil, fmt.Errorf("unexpected end")
	}
	switch c := p.s[p.i]; {
	case c == '(':
		p.i++
		n := &node{isList: true}
		for {
			p.space()
			if p.i >= len(p.s) {
				return nil, fmt.Errorf("unclosed list")
			}
			if p.s[p.i] == ')' {
				p.i++
				return n, nil
			}
			c, err := p.node()
			if err != nil {
				return nil, err
			}
			n.list = append(n.list, c)
		}
	case c == ')':
		return nil, fmt.Errorf("unexpected ) at byte %d", p.i)
	case c == '"':
		p.i++
		var b strings.Builder
		for {
			if p.i >= len(p.s) {
				return nil, fmt.Errorf("unclosed string")
			}
			c := p.s[p.i]
			p.i++
			if c == '"' {
				return &node{atom: b.String(), quoted: true}, nil
			}
			if c == '\\' {
				if p.i >= len(p.s) {
					return nil, fmt.Errorf("unclosed string")
				}
				c = p.s[p.i]
				p.i++
			}
			b.WriteByte(c)
		}
	default:
		start := p.i
		for p.i < len(p.s) && !strings.ContainsRune(" \t\r\n()\"", rune(p.s[p.i])) {
			p.i++
		}
		return &node{atom: p.s[start:p.i]}, nil
	}
}

// head returns a list's first atom, or "".
func (n *node) head() string {
	if !n.isList || len(n.list) == 0 || n.list[0].isList {
		return ""
	}
	return n.list[0].atom
}

// CheckTable returns the problems of one table's text: every library must
// have exactly one URI, and every URI must pass CheckURI.
func CheckTable(file, text string) []Problem {
	t, err := parse(text)
	if err != nil {
		return []Problem{{File: file, Why: "is not a library table: " + err.Error()}}
	}
	if h := t.head(); h != "sym_lib_table" && h != "fp_lib_table" {
		return []Problem{{File: file, Why: fmt.Sprintf("is a %q, not a sym_lib_table or fp_lib_table", h)}}
	}
	var ps []Problem
	for _, lib := range t.list[1:] {
		switch lib.head() {
		case "version":
			continue
		case "lib":
		default:
			ps = append(ps, Problem{File: file, Why: fmt.Sprintf("has an unknown entry %q", lib.head())})
			continue
		}
		var uris []string
		for _, f := range lib.list[1:] {
			if f.head() == "uri" {
				if len(f.list) != 2 || f.list[1].isList {
					ps = append(ps, Problem{File: file, Why: "has a uri that is not one string"})
					continue
				}
				uris = append(uris, f.list[1].atom)
			}
		}
		if len(uris) != 1 {
			ps = append(ps, Problem{File: file, Why: fmt.Sprintf("has a library with %d uris", len(uris))})
		}
		for _, u := range uris {
			if why := CheckURI(u); why != "" {
				ps = append(ps, Problem{File: file, URI: u, Why: why})
			}
		}
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
