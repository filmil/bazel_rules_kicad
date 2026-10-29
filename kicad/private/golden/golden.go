// SPDX-License-Identifier: Apache-2.0

// Package golden checks built files against checked-in golden files, and
// rewrites the golden files from the build (spec rule 4).
package golden

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Pair is one golden file and the built file it must equal.
type Pair struct {
	// Test is the label of the golden_test, such as //tests/bom:default_test.
	Test string
	// Built is the runfiles path of the built file.
	Built string
	// Golden is the runfiles path of the golden file.
	Golden string
	// Source is the golden file's path in the workspace, such as
	// tests/bom/expected_default.csv.
	Source string
}

// ParsePairs reads pairs written one per line, their fields separated by
// tabs, in the order of the Pair fields.
func ParsePairs(s string) ([]Pair, error) {
	var out []Pair
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 4 || f[0] == "" || f[1] == "" || f[2] == "" || f[3] == "" {
			return nil, fmt.Errorf("golden pair %q: want 4 fields separated by tabs", line)
		}
		out = append(out, Pair{Test: f[0], Built: f[1], Golden: f[2], Source: f[3]})
	}
	return out, nil
}

// FirstDifference returns the first line, counted from 1, where a and b
// differ, and the two lines; 0 when they are equal.
func FirstDifference(a, b []byte) (int, string, string) {
	if bytes.Equal(a, b) {
		return 0, "", ""
	}
	la := strings.SplitAfter(string(a), "\n")
	lb := strings.SplitAfter(string(b), "\n")
	for i := 0; ; i++ {
		var x, y string
		if i < len(la) {
			x = la[i]
		}
		if i < len(lb) {
			y = lb[i]
		}
		if x != y {
			return i + 1, x, y
		}
	}
}

// Check compares the built file with the golden file of p. resolve turns a
// runfiles path into a path to read.
func Check(p Pair, resolve func(string) (string, error), w io.Writer) error {
	built, err := readRunfile(resolve, p.Built)
	if err != nil {
		return err
	}
	want, err := readRunfile(resolve, p.Golden)
	if err != nil {
		return err
	}
	line, got, exp := FirstDifference(built, want)
	if line == 0 {
		fmt.Fprintf(w, "%s matches %s\n", p.Built, p.Source)
		return nil
	}
	fmt.Fprintf(w, "%s differs from the golden file %s, first at line %d:\n", p.Built, p.Source, line)
	fmt.Fprintf(w, "  built:  %q\n  golden: %q\n", strings.TrimSuffix(got, "\n"), strings.TrimSuffix(exp, "\n"))
	fmt.Fprintf(w, "If the change is intended, update the golden files with the module's update_goldens\n"+
		"target (bazel run //:update_goldens), and give the reason in the commit message.\n")
	return fmt.Errorf("%s: the built file differs from %s", p.Test, p.Source)
}

// Update writes the built file of each pair over its golden file in the
// workspace at root. It writes only the files that differ, and reports
// what it did.
func Update(pairs []Pair, root string, resolve func(string) (string, error), w io.Writer) (int, error) {
	changed := 0
	for _, p := range pairs {
		built, err := readRunfile(resolve, p.Built)
		if err != nil {
			return changed, err
		}
		dst := filepath.Join(root, filepath.FromSlash(p.Source))
		old, err := os.ReadFile(dst)
		if err != nil {
			return changed, fmt.Errorf("%s: the golden file: %w", p.Test, err)
		}
		if bytes.Equal(old, built) {
			fmt.Fprintf(w, "unchanged %s\n", p.Source)
			continue
		}
		fi, err := os.Stat(dst)
		if err != nil {
			return changed, err
		}
		if err := os.WriteFile(dst, built, fi.Mode().Perm()); err != nil {
			return changed, err
		}
		changed++
		fmt.Fprintf(w, "updated   %s (from %s)\n", p.Source, p.Test)
	}
	return changed, nil
}

func readRunfile(resolve func(string) (string, error), rpath string) ([]byte, error) {
	p, err := resolve(rpath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rpath, err)
	}
	return os.ReadFile(p)
}

var goldenTestRe = regexp.MustCompile(`(?m)^golden_test\(\s*\n\s*name\s*=\s*"([^"]+)"`)

// Unregistered returns the golden_test targets in the workspace at root
// that are not in registered, as labels. It reads the BUILD files of the
// module at root, and not those of a module nested in it (such as
// integration/), nor the bazel-* output links.
func Unregistered(root string, registered []string) ([]string, error) {
	known := map[string]bool{}
	for _, l := range registered {
		known[l] = true
	}
	var missing []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if p != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "bazel-")) {
				return filepath.SkipDir
			}
			if p != root && exists(filepath.Join(p, "MODULE.bazel")) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "BUILD.bazel" && d.Name() != "BUILD" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(p))
		if err != nil {
			return err
		}
		pkg := filepath.ToSlash(rel)
		if pkg == "." {
			pkg = ""
		}
		for _, m := range goldenTestRe.FindAllSubmatch(b, -1) {
			l := "//" + pkg + ":" + string(m[1])
			if !known[l] {
				missing = append(missing, l)
			}
		}
		return nil
	})
	sort.Strings(missing)
	return missing, err
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Lines splits a list given one item per line.
func Lines(s string) []string {
	var out []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		if t := strings.TrimSpace(sc.Text()); t != "" {
			out = append(out, t)
		}
	}
	return out
}
