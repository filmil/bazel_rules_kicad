// SPDX-License-Identifier: Apache-2.0

// Command sheetcheck checks that a schematic's files are exactly the sheets
// its root references (issue #13).
//
// Usage:
//
//	sheetcheck [--stamp FILE] ROOT [SHEET]...
//
// ROOT is the root .kicad_sch and the SHEETs are the other files the
// schematic target lists, all in one directory tree. From ROOT it follows
// every (sheet ... (property "Sheetfile" "...")), relative to the directory
// of the file that names it. A referenced file that is not among the given
// files is an error, and so is a given file that nothing references. The
// message names each such sheet. With --stamp, FILE is written when all is
// well, for a Bazel validation action.
//
// As a test, the files come from SHEETCHECK_FILES (root first, one path per
// line, relative to the runfiles directory), and SHEETCHECK_EXPECT=
// missing:NAME makes the check pass only if it finds the sheet NAME
// missing, for the rule's own known-bad test.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Sheetfiles returns the Sheetfile of every sheet that a schematic's text
// places, in order. It reads KiCad's S-expressions: lists, quoted strings
// with backslash escapes, and bare atoms.
func Sheetfiles(text string) ([]string, error) {
	root, err := parse(text)
	if err != nil {
		return nil, err
	}
	if root.head() != "kicad_sch" {
		return nil, fmt.Errorf("not a KiCad schematic (it starts with %q)", root.head())
	}
	var out []string
	for _, n := range root.list[1:] {
		if n.head() != "sheet" {
			continue
		}
		file := ""
		for _, p := range n.list[1:] {
			if p.head() == "property" && len(p.list) >= 3 && p.list[1].atom == "Sheetfile" {
				file = p.list[2].atom
			}
		}
		if file == "" {
			return nil, errors.New("a sheet has no Sheetfile property")
		}
		out = append(out, file)
	}
	return out, nil
}

// Problems lists what is wrong with the given files: sheets that are
// referenced but not given, and given files that nothing references.
// read returns a file's text.
func Problems(root string, given []string, read func(string) (string, error)) ([]string, error) {
	have := map[string]bool{}
	for _, g := range given {
		have[filepath.Clean(g)] = true
	}
	seen := map[string]bool{filepath.Clean(root): true}
	var problems []string
	queue := []string{filepath.Clean(root)}
	for len(queue) > 0 {
		f := queue[0]
		queue = queue[1:]
		text, err := read(f)
		if err != nil {
			return nil, err
		}
		refs, err := Sheetfiles(text)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		for _, r := range refs {
			p := filepath.Clean(filepath.Join(filepath.Dir(f), filepath.FromSlash(r)))
			if seen[p] {
				continue
			}
			seen[p] = true
			if !have[p] {
				problems = append(problems, fmt.Sprintf("sheet %q, which %s references, is not in srcs", r, filepath.Base(f)))
				continue
			}
			queue = append(queue, p)
		}
	}
	var unused []string
	for g := range have {
		if !seen[g] {
			unused = append(unused, fmt.Sprintf("%q is in srcs, but no sheet references it", filepath.Base(g)))
		}
	}
	sort.Strings(unused)
	return append(problems, unused...), nil
}

func run(args []string, expect string) (string, int) {
	stamp := ""
	if len(args) >= 2 && args[0] == "--stamp" {
		stamp, args = args[1], args[2:]
	}
	if len(args) == 0 {
		return "usage: sheetcheck [--stamp FILE] ROOT [SHEET]...", 2
	}
	read := func(p string) (string, error) {
		b, err := os.ReadFile(p)
		return string(b), err
	}
	ps, err := Problems(args[0], args[1:], read)
	if err != nil {
		return "sheetcheck: " + err.Error(), 2
	}
	msg := strings.Join(ps, "\n")
	if want, ok := strings.CutPrefix(expect, "missing:"); ok {
		for _, p := range ps {
			if strings.HasPrefix(p, fmt.Sprintf("sheet %q", want)) {
				return msg + "\nfound, as expected", 0
			}
		}
		return fmt.Sprintf("expected sheet %q to be missing; problems: %q", want, ps), 1
	}
	if len(ps) > 0 {
		return msg + "\nthe schematic's srcs do not match the sheets its root references", 1
	}
	if stamp != "" {
		if err := os.WriteFile(stamp, []byte("ok\n"), 0o644); err != nil {
			return "sheetcheck: " + err.Error(), 2
		}
	}
	return "", 0
}

func main() {
	args := os.Args[1:]
	if f := os.Getenv("SHEETCHECK_FILES"); f != "" && len(args) == 0 {
		for _, p := range strings.Split(f, "\n") {
			if p != "" {
				args = append(args, p)
			}
		}
	}
	msg, code := run(args, os.Getenv("SHEETCHECK_EXPECT"))
	if msg != "" {
		if code == 0 {
			fmt.Println(msg)
		} else {
			fmt.Fprintln(os.Stderr, msg)
		}
	}
	os.Exit(code)
}
