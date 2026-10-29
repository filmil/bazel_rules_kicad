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
// line, relative to the runfiles directory), and SHEETCHECK_EXPECT=KIND:NAME
// makes the check pass only if it finds that problem with the sheet NAME
// (relative to the root's directory), for the rule's own known-bad tests.
// KIND is missing, unused or circular.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
// referenced but not given, given files that nothing references, and
// sheets that reference one of their own ancestors (KiCad rejects a
// circular hierarchy). Names are relative to the root's directory. read
// returns a file's text.
func Problems(root string, given []string, read func(string) (string, error)) ([]string, error) {
	root = filepath.Clean(root)
	base := filepath.Dir(root)
	name := func(p string) string {
		if r, err := filepath.Rel(base, p); err == nil {
			return filepath.ToSlash(r)
		}
		return p
	}
	have := map[string]bool{}
	for _, g := range given {
		have[filepath.Clean(g)] = true
	}
	seen := map[string]bool{root: true}
	var problems []string
	// Depth first, with the chain of sheets from the root, to find cycles.
	var visit func(f string, chain []string) error
	visit = func(f string, chain []string) error {
		text, err := read(f)
		if err != nil {
			return err
		}
		refs, err := Sheetfiles(text)
		if err != nil {
			return fmt.Errorf("%s: %w", name(f), err)
		}
		for _, r := range refs {
			p := filepath.Clean(filepath.Join(filepath.Dir(f), filepath.FromSlash(r)))
			if slices.Contains(chain, p) {
				problems = append(problems, fmt.Sprintf("sheet %q, which %s references, is one of its own parents: the hierarchy is circular", name(p), name(f)))
				continue
			}
			if seen[p] {
				continue
			}
			seen[p] = true
			if !have[p] {
				problems = append(problems, fmt.Sprintf("sheet %q, which %s references, is not in srcs", name(p), name(f)))
				continue
			}
			if err := visit(p, append(slices.Clone(chain), p)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(root, []string{root}); err != nil {
		return nil, err
	}
	var unused []string
	for g := range have {
		if !seen[g] {
			unused = append(unused, fmt.Sprintf("sheet %q is in srcs, but no sheet references it", name(g)))
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
	if kind, want, ok := strings.Cut(expect, ":"); ok {
		suffix := map[string]string{"missing": "is not in srcs", "unused": "no sheet references it", "circular": "the hierarchy is circular"}[kind]
		if suffix == "" {
			return fmt.Sprintf("SHEETCHECK_EXPECT=%q: want missing:, unused: or circular: and a sheet", expect), 2
		}
		for _, p := range ps {
			if strings.HasPrefix(p, fmt.Sprintf("sheet %q", want)) && strings.HasSuffix(p, suffix) {
				return msg + "\nfound, as expected", 0
			}
		}
		return fmt.Sprintf("expected sheet %q to be %s; problems: %q", want, kind, ps), 1
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
